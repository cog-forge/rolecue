package resend_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
