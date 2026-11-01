package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
)

// newObservedLogger returns a logger that keeps entries in memory so tests can inspect fields.
func newObservedLogger(level zapcore.Level) (*logger.Logger, *observer.ObservedLogs) {
	core, logs := observer.New(level)
	return &logger.Logger{Logger: zap.New(core, zap.AddCaller())}, logs
}

func init() {
	gin.SetMode(gin.TestMode)
}

func TestRequestIDMiddleware_IDHandling(t *testing.T) {
	for _, tc := range []struct {
		name     string
		header   string // empty means the header is not sent
		wantEcho bool   // true when the client ID must be kept as-is
	}{
		{name: "absent header gets a generated uuid"},
		{name: "valid client id is kept", header: "abc-123_XYZ.9", wantEcho: true},
		{name: "too short is replaced", header: "short"},
		{name: "spaces are replaced", header: "has space here1"},
		{name: "angle brackets are replaced", header: "<script>alert1</script>"},
		{name: "too long is replaced", header: strings.Repeat("a", 65)},
		{name: "newline is replaced", header: "line\nbreak12345"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			log, _ := newObservedLogger(zapcore.DebugLevel)
			r := gin.New()
			r.Use(RequestIDMiddleware(log))
			r.GET("/x", func(c *gin.Context) { c.Status(http.StatusNoContent) })

			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			if tc.header != "" {
				req.Header.Set(RequestIDHeader, tc.header)
			}
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			got := rec.Header().Get(RequestIDHeader)
			if tc.wantEcho {
				if got != tc.header {
					t.Fatalf("echoed %q, want client id %q", got, tc.header)
				}
				return
			}
			if _, err := uuid.Parse(got); err != nil {
				t.Fatalf("response id %q is not a uuid: %v", got, err)
			}
			if got == tc.header {
				t.Fatalf("invalid client id %q was accepted", got)
			}
		})
	}
}

func TestRequestIDMiddleware_DownstreamSeesIDAndLogsCarryIt(t *testing.T) {
	log, logs := newObservedLogger(zapcore.DebugLevel)
	r := gin.New()
	r.Use(RequestIDMiddleware(log))

	var seenInCtx string
	r.GET("/x", func(c *gin.Context) {
		ctx := c.Request.Context()
		seenInCtx = logger.RequestID(ctx)
		logger.FromContext(ctx).Info("handler ran")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(RequestIDHeader, "client-req-42")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if seenInCtx != "client-req-42" {
		t.Fatalf("handler saw request id %q, want client-req-42", seenInCtx)
	}
	if got := rec.Header().Get(RequestIDHeader); got != "client-req-42" {
		t.Fatalf("response header %q, want client-req-42", got)
	}

	entries := logs.FilterMessage("handler ran").All()
	if len(entries) != 1 {
		t.Fatalf("want one handler entry, got %d", len(entries))
	}
	if got := entries[0].ContextMap()["request_id"]; got != "client-req-42" {
		t.Fatalf("entry request_id = %v, want client-req-42", got)
	}
}
