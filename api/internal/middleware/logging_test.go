package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
)

// accessLine returns the single "HTTP Request" access entry written during a request.
func accessLine(t *testing.T, logs *observer.ObservedLogs) observer.LoggedEntry {
	t.Helper()
	entries := logs.FilterMessage("HTTP Request").All()
	if len(entries) != 1 {
		t.Fatalf("want exactly one access line, got %d", len(entries))
	}
	return entries[0]
}

func newAccessEngine(t *testing.T) (*gin.Engine, *observer.ObservedLogs) {
	t.Helper()
	log, logs := newObservedLogger(zapcore.DebugLevel)
	r := gin.New()
	r.Use(LoggingMiddleware(log))
	return r, logs
}

func serve(r *gin.Engine, method, target string) {
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(method, target, nil))
}

func TestLoggingMiddleware_SuccessLineUsesRoutePattern(t *testing.T) {
	r, logs := newAccessEngine(t)
	r.GET("/users/:id", func(c *gin.Context) {
		response.OK(c, gin.H{"id": c.Param("id")})
	})

	serve(r, http.MethodGet, "/users/42")

	entry := accessLine(t, logs)
	if entry.Level != zapcore.InfoLevel {
		t.Fatalf("level = %v, want info for 2xx", entry.Level)
	}
	m := entry.ContextMap()
	checks := map[string]any{
		"method": "GET",
		"path":   "/users/42",
		"route":  "/users/:id",
		"status": int64(http.StatusOK),
	}
	for k, want := range checks {
		if m[k] != want {
			t.Fatalf("%s = %v, want %v (all %v)", k, m[k], want, m)
		}
	}
	if b, ok := m["bytes"].(int64); !ok || b <= 0 {
		t.Fatalf("bytes = %v, want positive", m["bytes"])
	}
	for _, k := range []string{"latency", "client_ip"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("%s missing: %v", k, m)
		}
	}
	for _, k := range []string{"error_code", "user_id", "role", "slow"} {
		if _, ok := m[k]; ok {
			t.Fatalf("%s should be absent for a fast anonymous 200: %v", k, m)
		}
	}
}

func TestLoggingMiddleware_UnmatchedRouteIsWarn(t *testing.T) {
	r, logs := newAccessEngine(t)

	serve(r, http.MethodGet, "/no/such/path")

	entry := accessLine(t, logs)
	if entry.Level != zapcore.WarnLevel {
		t.Fatalf("level = %v, want warn for 404", entry.Level)
	}
	m := entry.ContextMap()
	if m["route"] != "unmatched" || m["status"] != int64(http.StatusNotFound) {
		t.Fatalf("fields = %v", m)
	}
}

func TestLoggingMiddleware_LevelFollowsStatus(t *testing.T) {
	r, logs := newAccessEngine(t)
	r.GET("/bad", func(c *gin.Context) {
		response.Error(c, apperror.New(apperror.CodeValidation, "bad input"))
	})
	r.GET("/broken", func(c *gin.Context) {
		response.Error(c, errors.New("boom"))
	})

	serve(r, http.MethodGet, "/bad")
	bad := accessLine(t, logs)
	if bad.Level != zapcore.WarnLevel {
		t.Fatalf("4xx level = %v, want warn", bad.Level)
	}
	if got := bad.ContextMap()["error_code"]; got != string(apperror.CodeValidation) {
		t.Fatalf("error_code = %v, want %s", got, apperror.CodeValidation)
	}

	logs.TakeAll()
	serve(r, http.MethodGet, "/broken")
	broken := accessLine(t, logs)
	if broken.Level != zapcore.ErrorLevel {
		t.Fatalf("5xx level = %v, want error", broken.Level)
	}
	if got := broken.ContextMap()["error_code"]; got != string(apperror.CodeInternal) {
		t.Fatalf("error_code = %v, want %s", got, apperror.CodeInternal)
	}
}

func TestLoggingMiddleware_IncludesAuthenticatedUser(t *testing.T) {
	log, logs := newObservedLogger(zapcore.DebugLevel)
	r := gin.New()
	r.Use(LoggingMiddleware(log))
	userID := uuid.New()
	r.Use(func(c *gin.Context) {
		// Same unexported key RequireAuth uses, so CurrentUser reads it back.
		c.Set(currentUserKey, authdomain.User{ID: userID, Role: "recruiter"})
		c.Next()
	})
	r.GET("/me", func(c *gin.Context) { response.OK(c, nil) })

	serve(r, http.MethodGet, "/me")

	m := accessLine(t, logs).ContextMap()
	if m["user_id"] != userID.String() || m["role"] != "recruiter" {
		t.Fatalf("user fields = user_id %v role %v", m["user_id"], m["role"])
	}
}
