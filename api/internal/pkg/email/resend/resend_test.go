package resend_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/pkg/email"
	"github.com/cog-forge/rolecue/api/internal/pkg/email/resend"
)

const testKey = "re_test-key-DO-NOT-LEAK"

// newSender serves handler and returns a Sender pointed at it.
func newSender(t *testing.T, handler http.HandlerFunc, timeout time.Duration) email.Sender {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return resend.New(resend.Config{
		APIKey:  testKey,
		From:    "RoleCue <no-reply@example.com>",
		BaseURL: srv.URL,
		Timeout: timeout,
	})
}

func status(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(code) }
}

var sample = email.Message{To: "candidate@example.com", Subject: "Hello", Body: "Plain body"}

func TestSendRequestShape(t *testing.T) {
	var method, path, auth, contentType string
	var body map[string]any
	s := newSender(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		auth, contentType = r.Header.Get("Authorization"), r.Header.Get("Content-Type")
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"abc"}`))
	}, 5*time.Second)

	if err := s.Send(context.Background(), sample); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if method != http.MethodPost || path != "/emails" {
		t.Fatalf("request = %s %s, want POST /emails", method, path)
	}
	if auth != "Bearer "+testKey || contentType != "application/json" {
		t.Fatalf("auth=%q content-type=%q", auth, contentType)
	}
	to, _ := body["to"].([]any)
	if body["from"] != "RoleCue <no-reply@example.com>" || len(to) != 1 || to[0] != sample.To ||
		body["subject"] != sample.Subject || body["text"] != sample.Body {
		t.Fatalf("body = %v", body)
	}
}

func TestStatusClassification(t *testing.T) {
	cases := []struct {
		code      int
		permanent bool
	}{
		{http.StatusOK, false},
		{http.StatusAccepted, false},
		{http.StatusTooManyRequests, false},
		{http.StatusRequestTimeout, false},
		// A retry that meets the still-running first attempt (same Idempotency-Key) is retried, not dropped.
		{http.StatusConflict, false},
		{http.StatusInternalServerError, false},
		{http.StatusServiceUnavailable, false},
		{http.StatusBadRequest, true},
		{http.StatusUnauthorized, true},
		{http.StatusForbidden, true},
		{http.StatusUnprocessableEntity, true},
	}
	for _, tc := range cases {
		s := newSender(t, status(tc.code), 5*time.Second)
		err := s.Send(context.Background(), sample)
		if tc.code < 300 {
			if err != nil {
				t.Fatalf("HTTP %d: err = %v, want nil", tc.code, err)
			}
			continue
		}
		if err == nil {
			t.Fatalf("HTTP %d: expected an error", tc.code)
		}
		if email.IsPermanent(err) != tc.permanent {
			t.Fatalf("HTTP %d: permanent = %v, want %v", tc.code, email.IsPermanent(err), tc.permanent)
		}
	}
}

func TestNetworkErrorIsRetryable(t *testing.T) {
	srv := httptest.NewServer(status(http.StatusOK))
	url := srv.URL
	srv.Close() // nothing listens any more: the connection is refused
	s := resend.New(resend.Config{APIKey: testKey, From: "a@example.com", BaseURL: url, Timeout: time.Second})

	err := s.Send(context.Background(), sample)
	if err == nil || email.IsPermanent(err) {
		t.Fatalf("err = %v, want a retryable error", err)
	}
}

func TestErrorsNeverContainKeyOrResponseBody(t *testing.T) {
	const echoed = "SECRET-ECHO-FROM-PROVIDER"
	for _, code := range []int{http.StatusBadRequest, http.StatusInternalServerError} {
		s := newSender(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
			_, _ = io.WriteString(w, echoed+" "+r.Header.Get("Authorization"))
		}, 5*time.Second)

		err := s.Send(context.Background(), sample)
		if err == nil {
			t.Fatalf("HTTP %d: expected an error", code)
		}
		if strings.Contains(err.Error(), testKey) || strings.Contains(err.Error(), echoed) {
			t.Fatalf("HTTP %d: error leaked key or body: %v", code, err)
		}
	}
}

func TestSlowServerHitsTimeout(t *testing.T) {
	s := newSender(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(500 * time.Millisecond):
		}
	}, 50*time.Millisecond)

	start := time.Now()
	err := s.Send(context.Background(), sample)
	if err == nil || email.IsPermanent(err) {
		t.Fatalf("err = %v, want a retryable timeout", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("Send took %v, the 50ms timeout was not applied", elapsed)
	}
}

func TestRedirectIsNotFollowed(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/elsewhere" {
			hits.Add(1)
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Redirect(w, r, "/elsewhere", http.StatusFound)
	}))
	t.Cleanup(srv.Close)
	s := resend.New(resend.Config{APIKey: testKey, From: "a@example.com", BaseURL: srv.URL, Timeout: 5 * time.Second})

	err := s.Send(context.Background(), sample)
	if err == nil {
		t.Fatal("a redirect must not be reported as success")
	}
	if hits.Load() != 0 {
		t.Fatal("the redirect was followed, which would resend the bearer key to another location")
	}
	if !email.IsPermanent(err) {
		t.Fatalf("a 3xx response must be permanent, got %v", err)
	}
}

func TestIdempotencyKeyIsTheMessageIDOnEveryAttempt(t *testing.T) {
	var keys []string
	var mu sync.Mutex
	s := newSender(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}, 5*time.Second)
	withID := sample
	withID.ID = "msg-123"
	for i := 0; i < 2; i++ {
		if err := s.Send(context.Background(), withID); err != nil {
			t.Fatal(err)
		}
	}
	// A message without an ID sends no key rather than an empty one.
	if err := s.Send(context.Background(), sample); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(keys) != 3 || keys[0] != "msg-123" || keys[1] != "msg-123" || keys[2] != "" {
		t.Fatalf("idempotency keys = %q", keys)
	}
}

func TestHostedTemplateRequestShape(t *testing.T) {
	var body map[string]json.RawMessage
	var key string
	s := newSender(t, func(w http.ResponseWriter, r *http.Request) {
		key = r.Header.Get("Idempotency-Key")
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		w.WriteHeader(http.StatusOK)
	}, time.Second)
	msg := email.Message{ID: "decision-id", To: "owner@example.test", Template: &email.HostedTemplate{
		ID: "approved-template", Variables: map[string]string{"POSTING_TITLE": "Engineer", "POSTING_URL": "https://rolecue.test/posting"},
	}}
	if err := s.Send(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"subject", "text", "html", "react"} {
		if _, ok := body[field]; ok {
			t.Fatalf("hosted send unexpectedly contains %s", field)
		}
	}
	var template struct {
		ID        string
		Variables map[string]string
	}
	if err := json.Unmarshal(body["template"], &template); err != nil {
		t.Fatal(err)
	}
	if template.ID != msg.Template.ID || template.Variables["POSTING_TITLE"] != "Engineer" || template.Variables["POSTING_URL"] != msg.Template.Variables["POSTING_URL"] || key != msg.ID {
		t.Fatal("template or idempotency data changed")
	}
}

func TestInvalidHostedTemplateNeverCallsProvider(t *testing.T) {
	var calls int
	s := newSender(t, func(w http.ResponseWriter, _ *http.Request) { calls++; w.WriteHeader(http.StatusOK) }, time.Second)
	for _, msg := range []email.Message{
		{Template: &email.HostedTemplate{}},
		{Template: &email.HostedTemplate{ID: "template"}, Body: "inline"},
		{Template: &email.HostedTemplate{ID: "template"}, Subject: "inline"},
	} {
		if err := s.Send(context.Background(), msg); !email.IsPermanent(err) {
			t.Fatal("invalid template must fail permanently before sending")
		}
	}
	if calls != 0 {
		t.Fatal("invalid payload reached provider")
	}
}

func TestHostedTemplateRetryKeepsPayloadAndIdempotencyKey(t *testing.T) {
	var payloads []string
	var keys []string
	s := newSender(t, func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		payloads = append(payloads, string(body))
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		if len(payloads) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}, time.Second)
	dispatcher := email.NewDispatcher(s, email.DispatcherOptions{MaxAttempts: 2})
	dispatcher.Dispatch("moderation:test:rejected", email.Message{
		ID: "stable-decision-id", To: "owner@example.test", Template: &email.HostedTemplate{
			ID: "rejected-template", Variables: map[string]string{"POSTING_TITLE": "Engineer", "REJECT_REASON_1": "Add salary"},
		},
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := dispatcher.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if len(payloads) != 2 || payloads[0] != payloads[1] || keys[0] != "stable-decision-id" || keys[0] != keys[1] {
		t.Fatal("hosted retry changed payload or idempotency key")
	}
}
