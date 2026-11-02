package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap/zapcore"
)

func TestRecoveryMiddleware_LogsSanitizedPanicValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	log, logs := newObservedLogger(zapcore.DebugLevel)
	r := gin.New()
	r.Use(RequestIDMiddleware(log), RecoveryMiddleware(log))
	r.GET("/boom", func(*gin.Context) { panic("upstream said token=super-secret-value for user@example.vn") })

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	entries := logs.FilterMessage("Panic recovered in HTTP request").All()
	if len(entries) != 1 {
		t.Fatalf("want one panic entry, got %v", logs.AllUntimed())
	}
	panicText, _ := entries[0].ContextMap()["panic"].(string)
	for _, leaked := range []string{"super-secret-value", "user@example.vn"} {
		if strings.Contains(panicText, leaked) {
			t.Fatalf("%q leaked into panic log: %q", leaked, panicText)
		}
	}
	if entries[0].ContextMap()["request_id"] == nil {
		t.Fatal("panic log must carry the request ID")
	}
}
