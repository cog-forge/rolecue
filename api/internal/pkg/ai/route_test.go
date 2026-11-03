package ai

import (
	"context"
	"errors"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/cog-forge/rolecue/api/internal/config"
	"google.golang.org/genai"
	"strings"
	"testing"
	"time"
)

func TestRouteFallback(t *testing.T) {
	for _, tc := range []struct {
		name      string
		first     error
		wantCalls int
		wantErr   bool
	}{
		{"primary success", nil, 1, false},
		{"503", &HTTPError{Status: 503}, 2, false},
		{"invalid output", ErrInvalidOutput, 2, false},
		{"429", &HTTPError{Status: 429, RetryAfter: time.Hour}, 2, false},
		{"auth", &HTTPError{Status: 401}, 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &Route{budget: time.Second, entries: []routeEntry{{model: fakeModel{err: tc.first}, config: config.LLMConfig{}}, {model: fakeModel{}, config: config.LLMConfig{}}}}
			calls := 0
			_, err := Run(context.Background(), r, func(ctx context.Context, m model.BaseChatModel) (*schema.Message, error) {
				calls++
				return m.Generate(ctx, nil)
			})
			if (err != nil) != tc.wantErr || calls != tc.wantCalls {
				t.Fatalf("calls=%d error=%v", calls, err)
			}
		})
	}
}
func TestRouteBudgetAndTimeoutFallback(t *testing.T) {
	for _, exhausted := range []bool{false, true} {
		t.Run(map[bool]string{false: "attempt timeout reserves fallback", true: "parent budget exhausted"}[exhausted], func(t *testing.T) {
			r := &Route{budget: 40 * time.Millisecond, entries: []routeEntry{{model: fakeModel{}}, {model: fakeModel{}}}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			_, err := Run(ctx, r, func(ctx context.Context, _ model.BaseChatModel) (bool, error) {
				calls++
				if exhausted {
					cancel()
				}
				if calls == 1 {
					<-ctx.Done()
					return false, ctx.Err()
				}
				return true, nil
			})
			if exhausted {
				if !errors.Is(err, context.Canceled) || calls != 1 {
					t.Fatal(calls, err)
				}
			} else if err != nil || calls != 2 {
				t.Fatal(calls, err)
			}
		})
	}
}

func TestConfiguredModelSanitizesBeforeFeatureLogging(t *testing.T) {
	m := &loggedModel{task: "jd_extraction", inner: fakeModel{err: genai.APIError{Code: 503, Message: secretPrompt + secretAnswer + secretKey}}}
	_, err := m.Generate(context.Background(), nil)
	if err == nil || llmFailureReason(err) != "http_5xx" {
		t.Fatal("classification lost")
	}
	for _, secret := range []string{secretPrompt, secretAnswer, secretKey} {
		if strings.Contains(err.Error(), secret) {
			t.Fatal("provider content leaked")
		}
	}
}
