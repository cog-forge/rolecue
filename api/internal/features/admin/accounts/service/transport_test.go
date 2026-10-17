package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/features/admin/accounts/domain"
	"github.com/google/uuid"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestBetterAuthPluginInvocation(t *testing.T) {
	a := NewBetterAuthAdmin(config.AuthConfig{URL: "https://app.example.com", Timeout: time.Second, CookiePrefix: "rolecue", AdminSecret: "internal-secret"})
	id := uuid.New()
	a.client.Transport = roundTrip(func(req *http.Request) (*http.Response, error) {
		var body map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if req.URL.Path != "/api/auth/admin/ban-user" || req.Header.Get("Cookie") != "rolecue.session_token=admin" || req.Header.Get("X-RoleCue-Admin-Secret") != "internal-secret" || req.Header.Get("X-RoleCue-Account-Notification") != "send" || body["userId"] != id.String() || body["banReason"] != "Public reason" {
			t.Fatalf("unexpected plugin request: %+v", body)
		}
		data, _ := json.Marshal(map[string]any{"notification": map[string]string{"status": "failed"}, "user": map[string]any{"id": id, "banned": true, "name": "Alex", "email": "alex@example.com", "role": "candidate", "createdAt": time.Now(), "updatedAt": time.Now()}})
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(data)))}, nil
	})
	result, err := a.SetLock(context.Background(), "analytics=unrelated; rolecue.session_token=admin; rolecue.other=unrelated", id, true, true, domain.LockRequest{Reason: "Public reason"})
	if err != nil || result.User.ID != id || !result.User.IsLocked || result.Notification.Status != "failed" {
		t.Fatalf("plugin response: %+v %v", result, err)
	}
}

func TestBetterAuthFiltersCookies(t *testing.T) {
	for _, tc := range []struct {
		name   string
		prefix string
		secure bool
		cookie string
		want   string
	}{
		{"plain", "rolecue", false,
			"analytics=tracking; rolecue.session_token=admin; rolecue.dont_remember=true; rolecue.session_data=cached; rolecue.other=unrelated; __Secure-rolecue.session_token=wrong",
			"rolecue.session_token=admin; rolecue.dont_remember=true; rolecue.session_data=cached"},
		{"secure custom prefix", "custom", true,
			"custom.session_token=wrong; __Secure-rolecue.session_token=wrong; __Secure-custom.session_token=admin; __Secure-custom.dont_remember=true; __Secure-custom.session_data=cached; __Secure-custom.other=unrelated",
			"__Secure-custom.session_token=admin; __Secure-custom.dont_remember=true; __Secure-custom.session_data=cached"},
		{"unrelated only", "rolecue", false, "analytics=tracking; other.session_token=wrong", ""},
		{"empty prefix", "", false, ".session_token=wrong; analytics=tracking", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, locking := range []bool{true, false} {
				a := NewBetterAuthAdmin(config.AuthConfig{URL: "https://app.example.com", Timeout: time.Second, AdminSecret: "internal-secret", CookiePrefix: tc.prefix, SecureCookies: tc.secure})
				id := uuid.New()
				a.client.Transport = roundTrip(func(req *http.Request) (*http.Response, error) {
					if got := req.Header.Get("Cookie"); got != tc.want {
						t.Fatalf("locking=%v: Cookie = %q, want %q", locking, got, tc.want)
					}
					data, err := json.Marshal(map[string]any{"user": map[string]any{"id": id, "banned": locking}})
					if err != nil {
						t.Fatal(err)
					}
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(data)))}, nil
				})
				if _, err := a.SetLock(context.Background(), tc.cookie, id, locking, false, domain.LockRequest{Reason: "Public reason"}); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
