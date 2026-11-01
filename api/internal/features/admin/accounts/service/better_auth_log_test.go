package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/features/admin/accounts/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/google/uuid"
)

const (
	lockAdminSecret = "internal-secret-DO-NOT-LOG"
	lockCookie      = "rolecue.session_token=admin-session-DO-NOT-LOG"
	lockReasonText  = "Public reason with personal details"
)

func lockLogs(t *testing.T) (context.Context, *observer.ObservedLogs) {
	t.Helper()
	core, logs := observer.New(zapcore.DebugLevel)
	return logger.ToContext(context.Background(), &logger.Logger{Logger: zap.New(core)}), logs
}

func renderLockLogs(entries []observer.LoggedEntry) string {
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

func newLockAdmin(rt roundTrip) *BetterAuthAdmin {
	a := NewBetterAuthAdmin(config.AuthConfig{URL: "https://app.example.com", Timeout: time.Second, CookiePrefix: "rolecue", AdminSecret: lockAdminSecret})
	a.client.Transport = rt
	return a
}

func TestSetLockLogsOutcomeWithoutSecrets(t *testing.T) {
	id := uuid.New()
	ctx, logs := lockLogs(t)
	a := newLockAdmin(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Request: req,
			Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{"notification":{"status":"failed"},"user":{"id":%q,"banned":true,"role":"candidate"}}`, id.String())))}, nil
	})

	if _, err := a.SetLock(ctx, lockCookie, id, true, true, domain.LockRequest{Reason: lockReasonText}); err != nil {
		t.Fatal(err)
	}
	entries := logs.All()
	out := renderLockLogs(entries)
	if len(entries) != 2 || !strings.Contains(out, "debug step ok") || !strings.Contains(out, "warn account change confirmed but notification failed") {
		t.Fatalf("want step ok plus a notification warning, got:\n%s", out)
	}
	if !strings.Contains(out, "reason:notification_failed") || !strings.Contains(out, "op:accounts.set_lock") {
		t.Fatalf("notification failure not classified:\n%s", out)
	}
	for _, secret := range []string{lockAdminSecret, lockCookie, lockReasonText} {
		if strings.Contains(out, secret) {
			t.Fatalf("%q was logged:\n%s", secret, out)
		}
	}
}

func TestSetLockClassifiesFailures(t *testing.T) {
	for _, tt := range []struct {
		name   string
		status int
		body   string
		err    error
		reason string
		code   int
	}{
		{"credentials rejected", http.StatusUnauthorized, "", nil, "auth_rejected", 401},
		{"forbidden", http.StatusForbidden, "", nil, "forbidden", 403},
		{"upstream error", http.StatusBadGateway, "", nil, "http_5xx", 502},
		{"unreadable answer", http.StatusOK, "{not json", nil, "decode_error", 0},
		{"unreachable", 0, "", errors.New("dial tcp: connection refused"), "unreachable", 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, logs := lockLogs(t)
			a := newLockAdmin(func(req *http.Request) (*http.Response, error) {
				if tt.err != nil {
					return nil, tt.err
				}
				return &http.Response{StatusCode: tt.status, Header: make(http.Header), Request: req, Body: io.NopCloser(strings.NewReader(tt.body))}, nil
			})

			if _, err := a.SetLock(ctx, lockCookie, uuid.New(), false, false, domain.LockRequest{}); err == nil {
				t.Fatal("failure swallowed")
			}
			entries := logs.All()
			if len(entries) != 1 || entries[0].Level != zapcore.WarnLevel {
				t.Fatalf("want one warn, got:\n%s", renderLockLogs(entries))
			}
			out := renderLockLogs(entries)
			if !strings.Contains(out, "reason:"+tt.reason) {
				t.Fatalf("reason %s missing:\n%s", tt.reason, out)
			}
			if tt.code != 0 && !strings.Contains(out, fmt.Sprintf("status_code:%d", tt.code)) {
				t.Fatalf("status %d missing:\n%s", tt.code, out)
			}
			for _, secret := range []string{lockAdminSecret, lockCookie} {
				if strings.Contains(out, secret) {
					t.Fatalf("%q was logged:\n%s", secret, out)
				}
			}
		})
	}
}
