package ai

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"google.golang.org/genai"

	"github.com/cog-forge/rolecue/api/internal/config"
)

// countedModel answers every Generate with one fixed error (or "ok") and records how it was called.
type countedModel struct {
	err   error
	calls int
	name  string
	log   *[]string
}

func (m *countedModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	m.calls++
	if m.log != nil {
		*m.log = append(*m.log, m.name)
	}
	if m.err != nil {
		return nil, m.err
	}
	return schema.AssistantMessage("ok", nil), nil
}

func (m *countedModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("unused")
}

// chainOf builds a route whose entries are named p0, p1, ... so each has its own cooldown key.
func chainOf(health *ProviderHealth, models ...*countedModel) *Route {
	r := &Route{task: config.TaskBank, budget: 5 * time.Second, health: health}
	for i, m := range models {
		r.entries = append(r.entries, routeEntry{model: m, config: config.LLMConfig{Provider: fmt.Sprintf("p%d", i), Model: "m"}})
	}
	return r
}

func runChain(r *Route) (string, error) {
	return Run(context.Background(), r, func(ctx context.Context, m model.BaseChatModel) (string, error) {
		msg, err := m.Generate(ctx, nil)
		if err != nil {
			return "", err
		}
		return msg.Content, nil
	})
}

// The ticket acceptance: Groq rate limited and Gemini overloaded at once still succeed through a third provider.
func TestRouteThirdProviderSucceeds(t *testing.T) {
	var order []string
	a := &countedModel{name: "a", log: &order, err: &HTTPError{Status: 429, RetryAfter: 2 * time.Second}}
	b := &countedModel{name: "b", log: &order, err: &HTTPError{Status: 503}}
	c := &countedModel{name: "c", log: &order}
	r := chainOf(NewProviderHealth(time.Now), a, b, c)
	got, err := runChain(r)
	if err != nil || got != "ok" {
		t.Fatalf("result = %q, error = %v", got, err)
	}
	if a.calls != 1 || b.calls != 1 || c.calls != 1 || fmt.Sprint(order) != "[a b c]" {
		t.Fatalf("calls a=%d b=%d c=%d order=%v", a.calls, b.calls, c.calls, order)
	}
}

func TestRouteCooldownSkipsUntilExpiry(t *testing.T) {
	clock := newFakeClock()
	a := &countedModel{err: &HTTPError{Status: 429, RetryAfter: 30 * time.Second}}
	b := &countedModel{}
	r := chainOf(NewProviderHealth(clock.now), a, b)
	if _, err := runChain(r); err != nil || a.calls != 1 || b.calls != 1 {
		t.Fatalf("first run: a=%d b=%d err=%v", a.calls, b.calls, err)
	}
	clock.advance(29 * time.Second)
	if _, err := runChain(r); err != nil || a.calls != 1 || b.calls != 2 {
		t.Fatalf("inside the cooldown a was called again: a=%d b=%d err=%v", a.calls, b.calls, err)
	}
	clock.advance(2 * time.Second)
	a.err = nil
	if _, err := runChain(r); err != nil || a.calls != 2 || b.calls != 2 {
		t.Fatalf("after expiry a was not tried: a=%d b=%d err=%v", a.calls, b.calls, err)
	}
}

func TestRouteCooldownIsSharedByRoutesOfTheSameProvider(t *testing.T) {
	clock := newFakeClock()
	h := NewProviderHealth(clock.now)
	a1, a2 := &countedModel{err: &HTTPError{Status: 429, RetryAfter: time.Minute}}, &countedModel{}
	b1, b2 := &countedModel{}, &countedModel{}
	if _, err := runChain(chainOf(h, a1, b1)); err != nil {
		t.Fatal(err)
	}
	if _, err := runChain(chainOf(h, a2, b2)); err != nil || a2.calls != 0 {
		t.Fatalf("a second route called a cooled provider: calls=%d err=%v", a2.calls, err)
	}
}

func TestRouteCooldownOnly429(t *testing.T) {
	a := &countedModel{err: &HTTPError{Status: 503}}
	b := &countedModel{}
	r := chainOf(NewProviderHealth(time.Now), a, b)
	for i := 0; i < 2; i++ {
		if _, err := runChain(r); err != nil {
			t.Fatal(err)
		}
	}
	if a.calls != 2 {
		t.Fatalf("a 503 started a cooldown: a was called %d times, want 2", a.calls)
	}
}

func TestRouteNoRetryAfterUsesTheDefaultCooldown(t *testing.T) {
	clock := newFakeClock()
	a := &countedModel{err: &HTTPError{Status: 429}}
	b := &countedModel{}
	r := chainOf(NewProviderHealth(clock.now), a, b)
	_, _ = runChain(r)
	clock.advance(9 * time.Second)
	_, _ = runChain(r)
	if a.calls != 1 {
		t.Fatalf("a was called again inside the 10 s default: %d calls", a.calls)
	}
	clock.advance(2 * time.Second)
	_, _ = runChain(r)
	if a.calls != 2 {
		t.Fatalf("a was not tried after the default cooldown: %d calls", a.calls)
	}
}

func TestRouteAllCooling(t *testing.T) {
	a := &countedModel{err: &HTTPError{Status: 429, RetryAfter: time.Minute}}
	b := &countedModel{err: &HTTPError{Status: 429, RetryAfter: time.Minute}}
	r := chainOf(NewProviderHealth(newFakeClock().now), a, b)
	if _, err := runChain(r); err == nil {
		t.Fatal("expected the first run to fail: both providers are rate limited")
	}
	a.calls, b.calls = 0, 0
	_, err := runChain(r)
	if err == nil || llmFailureReason(err) != "rate_limited" {
		t.Fatalf("reason = %q, error = %v", llmFailureReason(err), err)
	}
	if a.calls != 0 || b.calls != 0 {
		t.Fatalf("a cooling provider was called: a=%d b=%d", a.calls, b.calls)
	}
}

func TestRouteWithoutHealthHasNoCooldown(t *testing.T) {
	a := &countedModel{err: &HTTPError{Status: 429, RetryAfter: time.Hour}}
	b := &countedModel{}
	r := chainOf(nil, a, b)
	_, _ = runChain(r)
	_, _ = runChain(r)
	if a.calls != 2 {
		t.Fatalf("a was skipped without a health store: %d calls", a.calls)
	}
}

func TestNewRouteSharesTheProcessWideHealth(t *testing.T) {
	h := NewProviderHealth(time.Now)
	useSharedHealth(t, h)
	cfg := &config.Config{LLM: config.LLMConfig{Provider: "gemini", APIKey: "k", Model: "m", Timeout: time.Second}}
	r, err := NewRoute(context.Background(), cfg, config.TaskJD)
	if err != nil {
		t.Fatal(err)
	}
	if r.health != h {
		t.Fatal("NewRoute did not use the shared health store")
	}
}

func TestAttemptSlots(t *testing.T) {
	entries := []routeEntry{
		{config: config.LLMConfig{Provider: "a", MaxRetries: 0}},
		{config: config.LLMConfig{Provider: "b", MaxRetries: 0}},
		{config: config.LLMConfig{Provider: "c", MaxRetries: 0}},
	}
	none := func(routeEntry) bool { return false }
	cooled := func(name string) func(routeEntry) bool {
		return func(e routeEntry) bool { return e.config.Provider == name }
	}
	for _, tc := range []struct {
		name           string
		index, attempt int
		cooling        func(routeEntry) bool
		want           int
	}{
		{"no cooldown, first of three", 0, 0, none, 3},
		{"no cooldown, last of three", 2, 0, none, 1},
		{"last entry cooled frees its share", 0, 0, cooled("c"), 2},
		{"two later entries cooled", 0, 0, func(e routeEntry) bool { return e.config.Provider != "a" }, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := attemptSlots(entries, tc.index, tc.attempt, tc.cooling); got != tc.want {
				t.Fatalf("slots = %d, want %d", got, tc.want)
			}
		})
	}
	withRetries := []routeEntry{{config: config.LLMConfig{MaxRetries: 2}}, {config: config.LLMConfig{}}}
	if got := attemptSlots(withRetries, 0, 1, none); got != 3 {
		t.Fatalf("slots with a retry left = %d, want 3 (this attempt, one retry, the next entry)", got)
	}
}

// A Retry-After survives sanitisation, so the route can read it; Gemini carries it in the error details.
func TestRetryAfterOf(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want time.Duration
	}{
		{"raw http error", &HTTPError{Status: 429, RetryAfter: 7 * time.Second}, 7 * time.Second},
		{"sanitised", safeProviderError(&HTTPError{Status: 429, RetryAfter: 9 * time.Second}), 9 * time.Second},
		{"gemini retry info", safeProviderError(genai.APIError{Code: 429, Details: []map[string]any{{"@type": "type.googleapis.com/google.rpc.RetryInfo", "retryDelay": "34s"}}}), 34 * time.Second},
		{"gemini without details", safeProviderError(genai.APIError{Code: 429}), 0},
		{"unrelated error", errors.New("boom"), 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := retryAfterOf(tc.err); got != tc.want {
				t.Fatalf("retryAfterOf = %s, want %s", got, tc.want)
			}
		})
	}
}
