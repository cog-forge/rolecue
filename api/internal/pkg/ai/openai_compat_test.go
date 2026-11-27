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

const compatOKBody = `{"choices":[{"message":{"content":"{\"ok\":true}"}}],"usage":{"prompt_tokens":12,"completion_tokens":3,"total_tokens":15}}`

func compatConfig(baseURL string) config.LLMConfig {
	return config.LLMConfig{Provider: "groq", APIKey: "fake-key", Model: "test-model", Timeout: 5 * time.Second, MaxOutputTokens: 777, BaseURL: baseURL}
}

func compatGenerate(t *testing.T, cfg config.LLMConfig) (*schema.Message, error) {
	t.Helper()
	m, err := newOpenAICompat(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return m.Generate(context.Background(), []*schema.Message{schema.SystemMessage("be brief"), schema.UserMessage("JSON")})
}

func TestOpenAICompatRequest(t *testing.T) {
	for _, provider := range []string{"groq", "openrouter"} {
		t.Run(provider, func(t *testing.T) {
			var seen struct {
				path   string
				header http.Header
				body   map[string]any
			}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seen.path, seen.header = r.URL.Path, r.Header.Clone()
				if err := json.NewDecoder(r.Body).Decode(&seen.body); err != nil {
					t.Error(err)
				}
				_, _ = w.Write([]byte(compatOKBody))
			}))
			defer srv.Close()
			cfg := compatConfig(srv.URL)
			cfg.Provider = provider
			msg, err := compatGenerate(t, cfg)
			if err != nil {
				t.Fatal(err)
			}
			if seen.path != "/chat/completions" {
				t.Errorf("path = %q", seen.path)
			}
			if seen.header.Get("Authorization") != "Bearer fake-key" || seen.header.Get("User-Agent") != "rolecue/1.0" {
				t.Errorf("headers = %v", seen.header)
			}
			if title := seen.header.Get("X-Title"); (provider == "openrouter") != (title == "RoleCue") {
				t.Errorf("X-Title = %q for %s", title, provider)
			}
			if seen.body["model"] != "test-model" || seen.body["reasoning_effort"] != "low" || seen.body["max_completion_tokens"] != float64(777) {
				t.Errorf("body = %v", seen.body)
			}
			if format, _ := seen.body["response_format"].(map[string]any); format["type"] != "json_object" {
				t.Errorf("response_format = %v", seen.body["response_format"])
			}
			messages, _ := seen.body["messages"].([]any)
			if len(messages) != 2 || messages[0].(map[string]any)["role"] != "system" || messages[1].(map[string]any)["content"] != "JSON" {
				t.Errorf("messages = %v", seen.body["messages"])
			}
			if msg.Content != `{"ok":true}` || msg.ResponseMeta == nil || msg.ResponseMeta.Usage == nil || msg.ResponseMeta.Usage.TotalTokens != 15 {
				t.Errorf("reply = %+v", msg)
			}
		})
	}
}

func TestOpenAICompatOmitsTokenCapWhenUnset(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(compatOKBody))
	}))
	defer srv.Close()
	cfg := compatConfig(srv.URL)
	cfg.MaxOutputTokens = 0
	if _, err := compatGenerate(t, cfg); err != nil {
		t.Fatal(err)
	}
	if _, present := body["max_completion_tokens"]; present {
		t.Errorf("token cap sent: %v", body)
	}
}

func TestOpenAICompatErrors(t *testing.T) {
	future := time.Now().Add(5 * time.Second).UTC().Format(http.TimeFormat)
	past := time.Now().Add(-time.Minute).UTC().Format(http.TimeFormat)
	for _, tc := range []struct {
		name       string
		status     int
		retryAfter string
		body       string
		wantRetry  func(time.Duration) bool
	}{
		{"429 seconds", 429, "7", `{"error":{"message":"secret-body slow down"}}`, func(d time.Duration) bool { return d == 7*time.Second }},
		{"429 http-date", 429, future, `{"error":{"message":"secret-body"}}`, func(d time.Duration) bool { return d > 3*time.Second && d <= 5*time.Second }},
		{"503 html", 503, "", `<html>secret-body bad gateway</html>`, func(d time.Duration) bool { return d == 0 }},
		{"401 json", 401, "", `{"error":{"message":"secret-body invalid key","type":"auth"}}`, func(d time.Duration) bool { return d == 0 }},
		{"400 json", 400, "", `{"error":{"message":"secret-body bad request"}}`, func(d time.Duration) bool { return d == 0 }},
		{"402 plain", 402, "", `secret-body payment required`, func(d time.Duration) bool { return d == 0 }},
		{"garbage retry-after", 429, "banana", `{}`, func(d time.Duration) bool { return d == 0 }},
		{"negative retry-after", 429, "-5", `{}`, func(d time.Duration) bool { return d == 0 }},
		{"past retry-after", 429, past, `{}`, func(d time.Duration) bool { return d == 0 }},
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
			_, err := compatGenerate(t, compatConfig(srv.URL))
			var httpErr *HTTPError
			if !errors.As(err, &httpErr) || httpErr.Status != tc.status || !tc.wantRetry(httpErr.RetryAfter) {
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

func TestOpenAICompatNoRedirect(t *testing.T) {
	var elsewhere atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { elsewhere.Add(1) }))
	defer other.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL, http.StatusFound)
	}))
	defer srv.Close()
	_, err := compatGenerate(t, compatConfig(srv.URL))
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusFound {
		t.Fatalf("error = %#v", err)
	}
	if elsewhere.Load() != 0 {
		t.Fatal("redirect was followed")
	}
}

func TestOpenAICompatInvalidOutput(t *testing.T) {
	huge := `{"choices":[{"message":{"content":"` + strings.Repeat("a", 3<<20) + `"}}]}`
	for name, body := range map[string]string{
		"not json":     `<html>nope</html>`,
		"no choices":   `{"choices":[]}`,
		"truncated":    `{"choices":[{"message":{"content":"{`,
		"over the cap": huge,
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
			defer srv.Close()
			_, err := compatGenerate(t, compatConfig(srv.URL))
			if !errors.Is(err, ErrInvalidOutput) {
				t.Fatalf("error = %#v", err)
			}
			if strings.Contains(err.Error(), "nope") {
				t.Fatal("provider body leaked into the error")
			}
		})
	}
}

func TestOpenAICompatTimeoutKeepsDeadlineCause(t *testing.T) {
	// The body is read first: the server only notices a vanished client once the request has been consumed.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		select {
		case <-r.Context().Done():
		case <-time.After(3 * time.Second):
		}
	}))
	defer srv.Close()
	cfg := compatConfig(srv.URL)
	cfg.Timeout = 30 * time.Millisecond
	_, err := compatGenerate(t, cfg)
	if err == nil || llmFailureReason(err) != "timeout" {
		t.Fatalf("reason = %q, error = %v", llmFailureReason(err), err)
	}
}

// Headers arrive but the body stalls: that is the provider being slow, not an unusable reply.
func TestOpenAICompatSlowBodyIsATimeout(t *testing.T) {
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
	cfg := compatConfig(srv.URL)
	cfg.Timeout = 50 * time.Millisecond
	_, err := compatGenerate(t, cfg)
	if err == nil || llmFailureReason(err) != "timeout" {
		t.Fatalf("reason = %q, error = %v", llmFailureReason(err), err)
	}
}

func TestOpenAICompatStreamUnsupported(t *testing.T) {
	m, err := newOpenAICompat(compatConfig("http://127.0.0.1:1"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Stream(context.Background(), nil); err == nil {
		t.Fatal("Stream must be unsupported")
	}
}
