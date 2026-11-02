package response_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// errorEngine serves one GET route whose handler calls response.Error with err. The logger is put
// on the request context the way RequestIDMiddleware does, so response.Error logs through it.
// The returned func reports the error code the gin context recorded after the handler ran.
func errorEngine(t *testing.T, err error) (*gin.Engine, *observer.ObservedLogs, *string) {
	t.Helper()
	core, logs := observer.New(zapcore.DebugLevel)
	log := &logger.Logger{Logger: zap.New(core, zap.AddCaller())}

	recorded := new(string)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(logger.ToContext(c.Request.Context(), log))
		c.Next()
		*recorded = c.GetString(response.ErrorCodeKey)
	})
	r.GET("/roles/:id", func(c *gin.Context) {
		response.Error(c, err)
	})
	return r, logs, recorded
}

func perform(r *gin.Engine) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/roles/7", nil))
	return rec
}

func singleEntry(t *testing.T, logs *observer.ObservedLogs) observer.LoggedEntry {
	t.Helper()
	all := logs.All()
	if len(all) != 1 {
		t.Fatalf("want exactly one log entry, got %d: %v", len(all), logs.AllUntimed())
	}
	return all[0]
}

func TestError_KnownClientCodeLogsWarnWithCauseChain(t *testing.T) {
	inner := errors.New("pq: connection refused")
	mid := fmt.Errorf("load user: %w", inner)
	err := apperror.Wrap(apperror.CodeAccountNotFound, "account not found", mid)

	r, logs, recorded := errorEngine(t, err)
	rec := perform(r)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	entry := singleEntry(t, logs)
	if entry.Level != zapcore.WarnLevel {
		t.Fatalf("level = %v, want warn for 4xx", entry.Level)
	}
	m := entry.ContextMap()
	if m["error_code"] != string(apperror.CodeAccountNotFound) {
		t.Fatalf("error_code = %v", m["error_code"])
	}
	if m["status"] != int64(http.StatusNotFound) || m["route"] != "/roles/:id" || m["method"] != "GET" {
		t.Fatalf("fields = %v", m)
	}
	chain, ok := m["cause_chain"].([]interface{})
	if !ok {
		t.Fatalf("cause_chain missing: %v", m)
	}
	if !containsString(chain, "pq: connection refused") || !containsString(chain, "load user: pq: connection refused") {
		t.Fatalf("cause_chain does not include the wrapped cause: %v", chain)
	}

	if *recorded != string(apperror.CodeAccountNotFound) {
		t.Fatalf("gin key %s = %q, want %s", response.ErrorCodeKey, *recorded, apperror.CodeAccountNotFound)
	}
}

func TestError_ServerCodeLogsErrorAndEnvelopeHidesCause(t *testing.T) {
	inner := errors.New("storage bucket secret-bucket-name unreachable")
	err := apperror.Wrap(apperror.CodeAvatarPersistenceFailed, "could not save avatar", inner)

	r, logs, _ := errorEngine(t, err)
	rec := perform(r)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if entry := singleEntry(t, logs); entry.Level != zapcore.ErrorLevel {
		t.Fatalf("level = %v, want error for 5xx", entry.Level)
	}
	if !strings.Contains(fmt.Sprint(singleEntry(t, logs).ContextMap()["cause_chain"]), "secret-bucket-name") {
		t.Fatalf("cause chain should carry the root cause for operators")
	}
	if strings.Contains(rec.Body.String(), "secret-bucket-name") || strings.Contains(rec.Body.String(), "unreachable") {
		t.Fatalf("client body leaked the cause: %s", rec.Body.String())
	}
}

func TestError_PlainErrorIsInternal(t *testing.T) {
	r, logs, recorded := errorEngine(t, errors.New("boom"))
	rec := perform(r)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	var env response.Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if env.Success || env.Error == nil || env.Error.Code != apperror.CodeInternal {
		t.Fatalf("envelope = %+v, want INTERNAL_ERROR", env)
	}
	if strings.Contains(rec.Body.String(), "boom") {
		t.Fatalf("client body leaked the cause: %s", rec.Body.String())
	}

	entry := singleEntry(t, logs)
	if entry.Level != zapcore.ErrorLevel {
		t.Fatalf("level = %v, want error", entry.Level)
	}
	m := entry.ContextMap()
	if m["error_code"] != string(apperror.CodeInternal) {
		t.Fatalf("error_code = %v", m["error_code"])
	}
	if chain, _ := m["cause_chain"].([]interface{}); len(chain) != 1 || chain[0] != "boom" {
		t.Fatalf("cause_chain = %v, want [boom]", m["cause_chain"])
	}
	if *recorded != string(apperror.CodeInternal) {
		t.Fatalf("gin key = %q, want INTERNAL_ERROR", *recorded)
	}
}

func TestError_UnmappedAppCodeFallsBackToInternal(t *testing.T) {
	r, logs, _ := errorEngine(t, apperror.New(apperror.Code("NOT_IN_TABLE"), "mystery"))
	rec := perform(r)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 for an unmapped code", rec.Code)
	}
	if singleEntry(t, logs).Level != zapcore.ErrorLevel {
		t.Fatalf("unmapped code should log at error")
	}
}

func containsString(items []interface{}, want string) bool {
	for _, item := range items {
		if s, ok := item.(string); ok && s == want {
			return true
		}
	}
	return false
}
