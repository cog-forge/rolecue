package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"github.com/cog-forge/rolecue/api/internal/config"
)

// Groq, Cloudflare and OpenRouter are served by the OpenAI-compatible adapter: the base URL comes from the
// config, and the model keeps the logged wrapper the route relies on.
func TestNewChatModelServesCompatProviders(t *testing.T) {
	for _, provider := range []string{"groq", "cloudflare", "openrouter"} {
		t.Run(provider, func(t *testing.T) {
			var hits int
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits++
				if r.URL.Path != "/chat/completions" || r.Header.Get("Authorization") != "Bearer fake-key" {
					t.Errorf("unexpected request %s %q", r.URL.Path, r.Header.Get("Authorization"))
				}
				_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{}"}}]}`))
			}))
			defer srv.Close()
			m, err := NewChatModel(context.Background(), config.LLMConfig{Provider: provider, APIKey: "fake-key", Model: "test-model", Timeout: time.Second, BaseURL: srv.URL})
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := m.(*loggedModel); !ok {
				t.Fatalf("model is %T, want the logged wrapper", m)
			}
			if _, err := m.Generate(context.Background(), []*schema.Message{schema.UserMessage("JSON")}); err != nil || hits != 1 {
				t.Fatalf("hits=%d err=%v", hits, err)
			}
		})
	}
}

// A compat provider without an API root would silently call the zero host; refuse it at construction.
func TestNewChatModelNeedsABaseURLForCompatProviders(t *testing.T) {
	for _, provider := range []string{"groq", "cloudflare", "openrouter"} {
		t.Run(provider, func(t *testing.T) {
			_, err := NewChatModel(context.Background(), config.LLMConfig{Provider: provider, APIKey: "k", Model: "m", Timeout: time.Second})
			if err == nil {
				t.Fatal("missing base URL accepted")
			}
		})
	}
}
