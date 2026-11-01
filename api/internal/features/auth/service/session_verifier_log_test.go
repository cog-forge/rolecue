package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

const verifierCookieSecret = "cookie-secret-DO-NOT-LOG"

func verifierFor(t *testing.T, handler http.HandlerFunc) (*HTTPVerifier, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	v := NewHTTPVerifier(config.AuthConfig{URL: server.URL, Timeout: time.Second, CookiePrefix: "rolecue", CookieDomain: "example.test"})
	return v, server
}

func verifierRequest() *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "rolecue.session_token", Value: verifierCookieSecret})
	return req
}

func verifierLogs() (context.Context, *observer.ObservedLogs) {
	core, logs := observer.New(zapcore.DebugLevel)
	return logger.ToContext(context.Background(), &logger.Logger{Logger: zap.New(core)}), logs
}

func renderVerifierLogs(entries []observer.LoggedEntry) string {
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

func TestVerifierLogsTheUpstreamStepWithoutSecrets(t *testing.T) {
	id := uuid.New()
	future := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	v, _ := verifierFor(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `{"user":{"id":%q},"session":{"id":%q,"userId":%q,"expiresAt":%q}}`, id, uuid.New(), id, future)
	})
	ctx, logs := verifierLogs()

	got, err := v.Verify(ctx, verifierRequest())
	if err != nil || got.UserID != id {
		t.Fatalf("verify: id=%s err=%v", got.UserID, err)
	}
	out := renderVerifierLogs(logs.All())
	if !strings.Contains(out, "debug step ok") || !strings.Contains(out, "op:auth.verify_session") || !strings.Contains(out, "user_id:"+id.String()) {
		t.Fatalf("success step not logged:\n%s", out)
	}
	assertNoVerifierSecrets(t, out)
}

func TestVerifierClassifiesRejectionsAndFailures(t *testing.T) {
	future := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	past := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	id := uuid.New()
	session := func(expires, impersonated string) string {
		return fmt.Sprintf(`{"user":{"id":%q},"session":{"id":%q,"userId":%q,"expiresAt":%q%s}}`, id, uuid.New(), id, expires, impersonated)
	}
	for _, tt := range []struct {
		name   string
		status int
		body   string
		reason string
		code   int
		sent   error
	}{
		{"session rejected", http.StatusUnauthorized, "", "auth_rejected", 401, domain.ErrInvalidSession},
		{"auth service outage", http.StatusServiceUnavailable, "", "http_5xx", 503, domain.ErrUnavailable},
		{"auth service client error", http.StatusNotFound, "", "http_4xx", 404, domain.ErrUnavailable},
		{"malformed answer", http.StatusOK, "{not json", "decode_error", 200, domain.ErrUnavailable},
		{"incomplete answer", http.StatusOK, `{"user":{"id":""}}`, "malformed_response", 200, domain.ErrUnavailable},
		{"null session", http.StatusOK, "null", "session_absent", 200, domain.ErrInvalidSession},
		{"expired session", http.StatusOK, session(past, ""), "session_expired", 200, domain.ErrInvalidSession},
		{"impersonated session", http.StatusOK, session(future, fmt.Sprintf(`,"impersonatedBy":%q`, uuid.NewString())), "impersonated_session", 200, domain.ErrInvalidSession},
	} {
		t.Run(tt.name, func(t *testing.T) {
			v, _ := verifierFor(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			})
			ctx, logs := verifierLogs()

			_, err := v.Verify(ctx, verifierRequest())
			if !errors.Is(err, tt.sent) {
				t.Fatalf("err = %v, want sentinel %v", err, tt.sent)
			}
			if tt.sent == domain.ErrInvalidSession && !apperror.IsCode(err, apperror.CodeInvalidToken) {
				t.Fatalf("client code lost: %v", err)
			}
			out := renderVerifierLogs(logs.All())
			if !strings.Contains(out, "reason:"+tt.reason) {
				t.Fatalf("reason %s missing:\n%s", tt.reason, out)
			}
			if tt.code != 200 && !strings.Contains(out, fmt.Sprintf("status_code:%d", tt.code)) {
				t.Fatalf("status %d missing:\n%s", tt.code, out)
			}
			assertNoVerifierSecrets(t, out)
		})
	}
}

func TestVerifierClassifiesUnreachableAuthService(t *testing.T) {
	v, server := verifierFor(t, func(http.ResponseWriter, *http.Request) {})
	server.Close()
	ctx, logs := verifierLogs()

	if _, err := v.Verify(ctx, verifierRequest()); !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("err = %v", err)
	}
	out := renderVerifierLogs(logs.All())
	if !strings.Contains(out, "reason:unreachable") {
		t.Fatalf("unreachable not classified:\n%s", out)
	}
	assertNoVerifierSecrets(t, out)
}

func TestVerifierMissingCookieNeverCallsUpstream(t *testing.T) {
	v, _ := verifierFor(t, func(http.ResponseWriter, *http.Request) { t.Error("upstream called without a session cookie") })
	ctx, logs := verifierLogs()

	_, err := v.Verify(ctx, httptest.NewRequest(http.MethodGet, "/", nil))
	if !errors.Is(err, domain.ErrInvalidSession) || !strings.Contains(err.Error(), "no session cookie") {
		t.Fatalf("err = %v", err)
	}
	if logs.Len() != 0 {
		t.Fatalf("missing cookie logged a step, the response layer logs the rejection:\n%s", renderVerifierLogs(logs.All()))
	}
}

func assertNoVerifierSecrets(t *testing.T, out string) {
	t.Helper()
	for _, secret := range []string{verifierCookieSecret, "disableCookieCache", "session_token"} {
		if strings.Contains(out, secret) {
			t.Fatalf("%q logged:\n%s", secret, out)
		}
	}
}
