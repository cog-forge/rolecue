package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/cog-forge/rolecue/api/internal/config"
	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
)

const sessionCookieSecret = "rolecue.session_token=cookie-secret-DO-NOT-LOG"

// loggedRouter serves handlers behind a request-scoped observer logger, so rejections are visible.
func loggedRouter(t *testing.T, mw ...gin.HandlerFunc) (*gin.Engine, *observer.ObservedLogs) {
	t.Helper()
	core, logs := observer.New(zapcore.DebugLevel)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(logger.ToContext(c.Request.Context(), &logger.Logger{Logger: zap.New(core)}))
		c.Next()
	})
	r.Use(mw...)
	r.Any("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	return r, logs
}

func renderMiddlewareLogs(entries []observer.LoggedEntry) string {
	var b strings.Builder
	for _, e := range entries {
		enc := zapcore.NewMapObjectEncoder()
		for _, f := range e.Context {
			f.AddTo(enc)
		}
		fmt.Fprintf(&b, "%s %s %v\n", e.Level, e.Message, enc.Fields)
	}
	return b.String()
}

func TestRequireAuthLogsWhyTheRequestIsRejected(t *testing.T) {
	for _, tt := range []struct {
		name   string
		origin string
		method string
		err    error
		status int
		want   string
	}{
		{"missing session", "http://localhost:3000", "GET", fmt.Errorf("%w: no session cookie", authdomain.ErrInvalidSession), http.StatusUnauthorized, "no session cookie"},
		{"expired session", "", "GET", fmt.Errorf("%w: session_expired", authdomain.ErrInvalidSession), http.StatusUnauthorized, "session_expired"},
		{"auth unavailable", "", "GET", fmt.Errorf("%w: timeout", authdomain.ErrUnavailable), http.StatusServiceUnavailable, "timeout"},
		{"untrusted origin", "https://evil.example.com", "POST", nil, http.StatusForbidden, "origin not in allowed list for POST"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			service := &authStub{err: tt.err, id: uuid.New()}
			r, logs := loggedRouter(t, RequireAuth(service, config.CORSConfig{AllowOrigins: []string{"http://localhost:3000"}}))
			req := httptest.NewRequest(tt.method, "/", nil)
			req.Header.Set("Origin", tt.origin)
			req.Header.Set("Cookie", sessionCookieSecret)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.status {
				t.Fatalf("status = %d, want %d", w.Code, tt.status)
			}
			// Client errors log as a rejection at warn; server errors as a failure at error (response.Error).
			wantLevel, wantMsg := zapcore.WarnLevel, "request rejected"
			if tt.status >= 500 {
				wantLevel, wantMsg = zapcore.ErrorLevel, "request failed"
			}
			entries := logs.All()
			if len(entries) != 1 || entries[0].Level != wantLevel || entries[0].Message != wantMsg {
				t.Fatalf("want one %s %q, got:\n%s", wantLevel, wantMsg, renderMiddlewareLogs(entries))
			}
			out := renderMiddlewareLogs(entries)
			if !strings.Contains(out, tt.want) {
				t.Fatalf("reason %q missing:\n%s", tt.want, out)
			}
			if strings.Contains(out, "cookie-secret-DO-NOT-LOG") {
				t.Fatalf("session cookie logged:\n%s", out)
			}
		})
	}
}

func TestRequireRoleLogsRequiredAndActualRole(t *testing.T) {
	id := uuid.New()
	r, logs := loggedRouter(t, func(c *gin.Context) {
		c.Set(currentUserKey, authdomain.User{ID: id, Role: "candidate", OnboardingCompleted: true})
		c.Next()
	}, RequireRole("admin", "recruiter"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d", w.Code)
	}
	out := renderMiddlewareLogs(logs.All())
	for _, want := range []string{"required [admin,recruiter]", `actual "candidate"`, "user_id=" + id.String(), "error_code:FORBIDDEN"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q:\n%s", want, out)
		}
	}
}

func TestRequireOnboardingLogsIncompleteRole(t *testing.T) {
	id := uuid.New()
	r, logs := loggedRouter(t, func(c *gin.Context) {
		c.Set(currentUserKey, authdomain.User{ID: id, Role: "recruiter", OnboardingCompleted: false})
		c.Next()
	}, RequireOnboarding())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d", w.Code)
	}
	out := renderMiddlewareLogs(logs.All())
	if !strings.Contains(out, "onboarding incomplete role=recruiter onboarding_completed=false user_id="+id.String()) {
		t.Fatalf("onboarding reason missing:\n%s", out)
	}
}

func TestAllowedRequestsAreNotLogged(t *testing.T) {
	id := uuid.New()
	r, logs := loggedRouter(t, func(c *gin.Context) {
		c.Set(currentUserKey, authdomain.User{ID: id, Role: "admin"})
		c.Next()
	}, RequireRole("admin"), RequireOnboarding())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != http.StatusNoContent || logs.Len() != 0 {
		t.Fatalf("status=%d logs=%d, allowed requests must not log", w.Code, logs.Len())
	}
}
