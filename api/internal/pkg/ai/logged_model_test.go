package ai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/genai"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
)

const (
	secretPrompt = "Confidential job description: pay band 180k"
	secretAnswer = "Candidate answer with private salary history"
	secretKey    = "sk-test-secret-key-value"
)

// fakeModel returns a fixed error from Generate and Stream, or a fixed answer.
type fakeModel struct{ err error }

func (f fakeModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	if f.err != nil {
		return nil, f.err
	}
	return schema.AssistantMessage(secretAnswer, nil), nil
}

func (f fakeModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, f.err
}

// rendered flattens every logged entry, including all fields, into one string for absence checks.
func rendered(entries []observer.LoggedEntry) string {
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

func TestLoggedModelClassifiesFailures(t *testing.T) {
	for _, tt := range []struct {
		name   string
		err    error
		reason string
		status int
	}{
		{"rate limited", genai.APIError{Code: 429}, "rate_limited", 429},
		{"auth rejected", genai.APIError{Code: 403}, "auth_rejected", 403},
		{"client error", genai.APIError{Code: 400}, "http_4xx", 400},
		{"server error", genai.APIError{Code: 503}, "http_5xx", 503},
		{"deadline", context.DeadlineExceeded, "timeout", 0},
		{"canceled", context.Canceled, "canceled", 0},
		{"other", errors.New("boom"), "other", 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			core, logs := observer.New(zapcore.DebugLevel)
			ctx := logger.ToContext(context.Background(), &logger.Logger{Logger: zap.New(core)})
			m := &loggedModel{inner: fakeModel{err: tt.err}, provider: "gemini", model: "test-model"}

			if _, err := m.Generate(ctx, []*schema.Message{schema.UserMessage(secretPrompt)}); err == nil || err.Error() != tt.err.Error() {
				t.Fatalf("error not returned unchanged: %v", err)
			}
			entries := logs.All()
			if len(entries) != 1 || entries[0].Level != zapcore.WarnLevel || entries[0].Message != "step failed" {
				t.Fatalf("want one warn step failure, got:\n%s", rendered(entries))
			}
			out := rendered(entries)
			if !strings.Contains(out, "reason:"+tt.reason) {
				t.Fatalf("reason %q missing:\n%s", tt.reason, out)
			}
			if tt.status != 0 && !strings.Contains(out, fmt.Sprintf("status_code:%d", tt.status)) {
				t.Fatalf("status %d missing:\n%s", tt.status, out)
			}
			if strings.Contains(out, secretPrompt) {
				t.Fatalf("prompt text was logged:\n%s", out)
			}
		})
	}
}

func TestLoggedModelLogsSuccessWithoutContent(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	ctx := logger.ToContext(context.Background(), &logger.Logger{Logger: zap.New(core)})
	m := &loggedModel{inner: fakeModel{}, provider: "gemini", model: "test-model"}

	msg, err := m.Generate(ctx, []*schema.Message{schema.SystemMessage(secretPrompt), schema.UserMessage("hi")})
	if err != nil || msg.Content != secretAnswer {
		t.Fatalf("decorator changed the result: msg=%v err=%v", msg, err)
	}
	entries := logs.All()
	if len(entries) != 1 || entries[0].Level != zapcore.DebugLevel || entries[0].Message != "step ok" {
		t.Fatalf("want one debug success, got:\n%s", rendered(entries))
	}
	out := rendered(entries)
	if !strings.Contains(out, "messages:2") || !strings.Contains(out, "op:llm.generate") {
		t.Fatalf("missing count or op:\n%s", out)
	}
	if strings.Contains(out, secretPrompt) || strings.Contains(out, secretAnswer) {
		t.Fatalf("content was logged:\n%s", out)
	}
}

func TestLoggedStreamOpenFailureIsLogged(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	ctx := logger.ToContext(context.Background(), &logger.Logger{Logger: zap.New(core)})
	m := &loggedModel{inner: fakeModel{err: genai.APIError{Code: 500}}, provider: "gemini", model: "test-model"}

	if _, err := m.Stream(ctx, []*schema.Message{schema.UserMessage(secretPrompt)}); err == nil {
		t.Fatal("stream error swallowed")
	}
	out := rendered(logs.All())
	if !strings.Contains(out, "op:llm.stream_open") || !strings.Contains(out, "reason:http_5xx") {
		t.Fatalf("stream open failure not classified:\n%s", out)
	}
	if strings.Contains(out, secretPrompt) {
		t.Fatalf("prompt text was logged:\n%s", out)
	}
}

// The real Gemini path: the API key and prompt must never reach a log field, even when the
// provider's answer is an error.
func TestChatModelLogsOutcomeWithoutSecrets(t *testing.T) {
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		_, _ = io.ReadAll(r.Body)
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"error":{"code":503,"message":"unavailable","status":"UNAVAILABLE"}}`)), Request: r}, nil
	})
	m, err := NewChatModel(context.Background(), config.LLMConfig{Provider: "gemini", APIKey: secretKey, Model: "test-model", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	core, logs := observer.New(zapcore.DebugLevel)
	ctx := logger.ToContext(context.Background(), &logger.Logger{Logger: zap.New(core)})
	if _, err := m.Generate(ctx, []*schema.Message{schema.UserMessage(secretPrompt)}); err == nil {
		t.Fatal("expected provider failure")
	}
	out := rendered(logs.All())
	if !strings.Contains(out, "reason:http_5xx") || !strings.Contains(out, "provider:gemini") {
		t.Fatalf("failure not logged with reason and provider:\n%s", out)
	}
	for _, secret := range []string{secretKey, secretPrompt, "unavailable"} {
		if strings.Contains(out, secret) {
			t.Fatalf("%q was logged:\n%s", secret, out)
		}
	}
}
