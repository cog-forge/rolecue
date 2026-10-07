package ai

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/cog-forge/rolecue/api/internal/config"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Exercise our runtime wiring, including disabling the SDK's default retries.
func TestChatModelRuntime(t *testing.T) {
	for _, timeout := range []bool{false, true} {
		t.Run(map[bool]string{false: "one native request", true: "timeout"}[timeout], func(t *testing.T) {
			original := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = original })
			calls := 0
			http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.URL.Host != "generativelanguage.googleapis.com" || r.URL.Path != "/v1beta/models/test-model:generateContent" {
					t.Error("unexpected native Gemini endpoint")
				}
				if r.Header.Get("x-goog-api-key") != "test-key" {
					t.Error("API key not configured")
				}
				if timeout {
					<-r.Context().Done()
					return nil, r.Context().Err()
				}
				return &http.Response{StatusCode: http.StatusServiceUnavailable, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"error":{"code":503,"message":"unavailable","status":"UNAVAILABLE"}}`)), Request: r}, nil
			})
			m, err := NewChatModel(context.Background(), config.LLMConfig{Provider: "gemini", APIKey: "test-key", Model: "test-model", Timeout: 20 * time.Millisecond})
			if err != nil {
				t.Fatal(err)
			}
			_, err = m.Generate(context.Background(), []*schema.Message{schema.UserMessage("test")})
			if err == nil || calls != 1 {
				t.Fatalf("err=%v requests=%d", err, calls)
			}
			if timeout && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("deadline cause lost: %v", err)
			}
		})
	}
}

func TestRejectUnsupportedRuntimeConfiguration(t *testing.T) {
	valid := config.LLMConfig{Provider: "gemini", APIKey: "test-key", Model: "test-model", Timeout: time.Second}
	for _, tt := range []struct {
		name   string
		change func(*config.LLMConfig)
	}{
		{"unsupported provider", func(c *config.LLMConfig) { c.Provider = "openai_compatible" }},
		{"missing key", func(c *config.LLMConfig) { c.APIKey = "" }},
		{"invalid key", func(c *config.LLMConfig) { c.APIKey = "test\nkey" }},
		{"missing model", func(c *config.LLMConfig) { c.Model = " " }},
		{"invalid timeout", func(c *config.LLMConfig) { c.Timeout = 0 }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := valid
			tt.change(&c)
			if _, err := NewChatModel(context.Background(), c); err == nil {
				t.Fatal("invalid model configuration accepted")
			}
		})
	}
}
