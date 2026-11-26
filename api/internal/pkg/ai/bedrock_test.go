package ai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"github.com/cog-forge/rolecue/api/internal/config"
)

const bedrockOKBody = `{"output":{"message":{"role":"assistant","content":[{"reasoningContent":{"reasoningText":{"text":"thinking"}}},{"text":"{\"ok\":"},{"text":"true}"}]}},"stopReason":"end_turn","usage":{"inputTokens":12,"outputTokens":7,"totalTokens":19}}`

func bedrockConfig(baseURL string) config.LLMConfig {
	return config.LLMConfig{Provider: "bedrock", APIKey: "bedrock-key", Model: "openai.gpt-oss-120b-1:0", Timeout: 5 * time.Second, MaxOutputTokens: 777, BaseURL: baseURL}
}

func bedrockGenerate(t *testing.T, cfg config.LLMConfig, input ...*schema.Message) (*schema.Message, error) {
	t.Helper()
	m, err := newBedrock(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(input) == 0 {
		input = []*schema.Message{schema.UserMessage("JSON")}
	}
	return m.Generate(context.Background(), input)
}

func TestBedrockRequest(t *testing.T) {
	var uri, auth, agent string
	var body struct {
		System   []map[string]string `json:"system"`
		Messages []struct {
			Role    string              `json:"role"`
			Content []map[string]string `json:"content"`
		} `json:"messages"`
		InferenceConfig map[string]any `json:"inferenceConfig"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uri, auth, agent = r.RequestURI, r.Header.Get("Authorization"), r.Header.Get("User-Agent")
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		_, _ = w.Write([]byte(bedrockOKBody))
	}))
	defer srv.Close()
	msg, err := bedrockGenerate(t, bedrockConfig(srv.URL),
		schema.SystemMessage("be brief"), schema.UserMessage("first"), schema.UserMessage("second"),
		schema.AssistantMessage("earlier", nil), schema.UserMessage("JSON"))
	if err != nil {
		t.Fatal(err)
	}
	if uri != "/model/openai.gpt-oss-120b-1%3A0/converse" {
		t.Errorf("request uri = %q", uri)
	}
	if auth != "Bearer bedrock-key" || agent != "rolecue/1.0" {
		t.Errorf("headers: authorization=%q user-agent=%q", auth, agent)
	}
	if len(body.System) != 1 || body.System[0]["text"] != "be brief" {
		t.Errorf("system = %v", body.System)
	}
	// Converse wants alternating roles: consecutive messages of one role become one message with two blocks.
	if len(body.Messages) != 3 || body.Messages[0].Role != "user" || len(body.Messages[0].Content) != 2 ||
		body.Messages[1].Role != "assistant" || body.Messages[2].Role != "user" || body.Messages[2].Content[0]["text"] != "JSON" {
		t.Errorf("messages = %+v", body.Messages)
	}
	if body.InferenceConfig["maxTokens"] != float64(777) {
		t.Errorf("inferenceConfig = %v", body.InferenceConfig)
	}
	if msg.Content != `{"ok":true}` {
		t.Errorf("content = %q (reasoning blocks must be dropped, text blocks joined)", msg.Content)
	}
	if u := msg.ResponseMeta.Usage; u == nil || u.PromptTokens != 12 || u.CompletionTokens != 7 || u.TotalTokens != 19 {
		t.Errorf("usage = %+v", msg.ResponseMeta.Usage)
	}
}

func TestBedrockOmitsTokenCapWhenUnset(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(bedrockOKBody))
	}))
	defer srv.Close()
	cfg := bedrockConfig(srv.URL)
	cfg.MaxOutputTokens = 0
	if _, err := bedrockGenerate(t, cfg); err != nil {
		t.Fatal(err)
	}
	if _, present := body["inferenceConfig"]; present {
		t.Errorf("inferenceConfig sent without a cap: %v", body)
	}
}

func TestBedrockErrors(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		retryAfter string
		body       string
		wantRetry  time.Duration
	}{
		{"throttled with retry-after", 429, "7", `{"message":"secret-body Too many requests"}`, 7 * time.Second},
		{"restricted account", 400, "", `{"message":"secret-body Operation not allowed"}`, 0},
		{"bad gateway html", 503, "", `<html>secret-body</html>`, 0},
		{"expired key", 403, "", `{"message":"secret-body invalid token"}`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if tc.retryAfter != "" {
					w.Header().Set("Retry-After", tc.retryAfter)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			_, err := bedrockGenerate(t, bedrockConfig(srv.URL))
			var httpErr *HTTPError
			if !errors.As(err, &httpErr) || httpErr.Status != tc.status || httpErr.RetryAfter != tc.wantRetry {
				t.Fatalf("error = %#v", err)
			}
			if strings.Contains(err.Error(), "secret-body") {
				t.Fatal("provider body leaked into the error")
			}
			if calls.Load() != 1 {
				t.Fatalf("requests = %d, want exactly one", calls.Load())
			}
		})
	}
}

func TestBedrockNoRedirect(t *testing.T) {
	var elsewhere atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { elsewhere.Add(1) }))
	defer other.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL, http.StatusTemporaryRedirect)
	}))
	defer srv.Close()
	_, err := bedrockGenerate(t, bedrockConfig(srv.URL))
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusTemporaryRedirect || elsewhere.Load() != 0 {
		t.Fatalf("error = %#v, redirect hits = %d", err, elsewhere.Load())
	}
}

// A 200 reply with nothing to decode is invalid output, never an empty success.
func TestBedrockInvalidOutput(t *testing.T) {
	for name, body := range map[string]string{
		"reasoning only": `{"output":{"message":{"content":[{"reasoningContent":{"reasoningText":{"text":"only thoughts"}}}]}}}`,
		"no content":     `{"output":{"message":{"content":[]}}}`,
		"no output":      `{}`,
		"not json":       `<html>nope</html>`,
		"truncated":      `{"output":{"message":{"content":[{"text":"{`,
		"over the cap":   `{"output":{"message":{"content":[{"text":"` + strings.Repeat("a", 3<<20) + `"}]}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
			defer srv.Close()
			_, err := bedrockGenerate(t, bedrockConfig(srv.URL))
			if !errors.Is(err, ErrInvalidOutput) {
				t.Fatalf("error = %#v", err)
			}
			if strings.Contains(err.Error(), "nope") || strings.Contains(err.Error(), "only thoughts") {
				t.Fatal("provider text leaked into the error")
			}
		})
	}
}

func TestBedrockTimeoutKeepsDeadlineCause(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		select {
		case <-r.Context().Done():
		case <-time.After(3 * time.Second):
		}
	}))
	defer srv.Close()
	cfg := bedrockConfig(srv.URL)
	cfg.Timeout = 30 * time.Millisecond
	_, err := bedrockGenerate(t, cfg)
	if err == nil || llmFailureReason(err) != "timeout" {
		t.Fatalf("reason = %q, error = %v", llmFailureReason(err), err)
	}
}

// Headers arrive but the body stalls: that is the provider being slow, not an unusable reply.
func TestBedrockSlowBodyIsATimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
		case <-time.After(3 * time.Second):
		}
	}))
	defer srv.Close()
	cfg := bedrockConfig(srv.URL)
	cfg.Timeout = 50 * time.Millisecond
	_, err := bedrockGenerate(t, cfg)
	if err == nil || llmFailureReason(err) != "timeout" {
		t.Fatalf("reason = %q, error = %v", llmFailureReason(err), err)
	}
}

func TestBedrockRejectsUnsupportedRoles(t *testing.T) {
	_, err := bedrockGenerate(t, bedrockConfig("http://127.0.0.1:1"), &schema.Message{Role: schema.Tool, Content: "x"})
	if err == nil {
		t.Fatal("a tool message was accepted")
	}
}

func TestBedrockStreamUnsupported(t *testing.T) {
	m, err := newBedrock(bedrockConfig("http://127.0.0.1:1"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Stream(context.Background(), nil); err == nil {
		t.Fatal("Stream must be unsupported")
	}
}
