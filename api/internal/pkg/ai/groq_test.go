package ai

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/cloudwego/eino/schema"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGroqHTTP(t *testing.T) {
	for _, status := range []int{200, 429, 503, 401} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Header.Get("Authorization") != "Bearer fake-key" {
					t.Error("authorization missing")
				}
				var body struct {
					Model  string `json:"model"`
					Format struct {
						Type string `json:"type"`
					} `json:"response_format"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if body.Model != "test-model" || body.Format.Type != "json_object" {
					t.Error("model or JSON mode missing")
				}
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{}"}}],"usage":{"prompt_tokens":12,"completion_tokens":3,"total_tokens":15}}`))
			}))
			defer srv.Close()
			m := &groqModel{key: "fake-key", model: "test-model", endpoint: srv.URL, client: srv.Client()}
			msg, err := m.Generate(context.Background(), []*schema.Message{schema.UserMessage("JSON")})
			if calls != 1 {
				t.Fatal("unexpected retry")
			}
			if status == 200 {
				if err != nil || msg.ResponseMeta.Usage.TotalTokens != 15 {
					t.Fatal("usage missing", err)
				}
			} else {
				var apiErr *HTTPError
				if !errors.As(err, &apiErr) || apiErr.Status != status || apiErr.RetryAfter != time.Minute {
					t.Fatal(err)
				}
			}
		})
	}
}
