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
	a := NewBetterAuthAdmin(config.AuthConfig{URL: "https://app.example.com", Timeout: time.Second, AdminSecret: "internal-secret"})
	id := uuid.New()
	a.client.Transport = roundTrip(func(req *http.Request) (*http.Response, error) {
		var body map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if req.URL.Path != "/api/auth/admin/ban-user" || req.Header.Get("Cookie") != "session=admin" || req.Header.Get("X-RoleCue-Admin-Secret") != "internal-secret" || req.Header.Get("X-RoleCue-Account-Notification") != "send" || body["userId"] != id.String() || body["banReason"] != "Public reason" {
			t.Fatalf("unexpected plugin request: %+v", body)
		}
		data, _ := json.Marshal(map[string]any{"notification": map[string]string{"status": "failed"}, "user": map[string]any{"id": id, "banned": true, "name": "Alex", "email": "alex@example.com", "role": "candidate", "createdAt": time.Now(), "updatedAt": time.Now()}})
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(data)))}, nil
	})
	result, err := a.SetLock(context.Background(), "session=admin", id, true, true, domain.LockRequest{Reason: "Public reason"})
	if err != nil || result.User.ID != id || !result.User.IsLocked || result.Notification.Status != "failed" {
		t.Fatalf("plugin response: %+v %v", result, err)
	}
}
