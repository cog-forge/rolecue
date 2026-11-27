package ai

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/cog-forge/rolecue/api/internal/config"
	qd "github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	md "github.com/cog-forge/rolecue/api/internal/features/roadmap/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/llmerr"
)

// A skipped (cooling) entry is a rate-limited provider that was not called; it must count when the other entries
// fail with something less informative, or the first request of a burst is reported as busy and every later one
// as a configuration problem (review of KAN-386).
func TestRouteSkippedEntriesCountTowardsTheFailure(t *testing.T) {
	for _, tc := range []struct {
		name    string
		cooling []int
		errs    []error
		reason  string
	}{
		{"cooling first, then two client errors", []int{0}, []error{nil, &HTTPError{Status: 403}, &HTTPError{Status: 400}}, "rate_limited"},
		{"client error, then cooling", []int{1}, []error{&HTTPError{Status: 403}, nil}, "rate_limited"},
		{"cooling first, then unusable replies", []int{0}, []error{nil, ErrInvalidOutput, ErrInvalidOutput}, "rate_limited"},
		{"a real 5xx still wins over nothing", []int{0}, []error{nil, &HTTPError{Status: 503}}, "http_5xx"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := NewProviderHealth(newFakeClock().now)
			var models []*countedModel
			for _, err := range tc.errs {
				models = append(models, &countedModel{err: err})
			}
			r := chainOf(h, models...)
			for _, i := range tc.cooling {
				h.RateLimited(entryKey(r.entries[i].config), time.Minute)
			}
			_, err := runChain(r)
			if err == nil || llmFailureReason(err) != tc.reason {
				t.Fatalf("reason = %q (error %v), want %q", llmFailureReason(err), err, tc.reason)
			}
			for _, i := range tc.cooling {
				if models[i].calls != 0 {
					t.Errorf("cooling entry %d was called", i)
				}
			}
		})
	}
}

// slowModel answers nothing until its call is cancelled by a deadline.
type slowModel struct{}

func (slowModel) Generate(ctx context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (slowModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("unused")
}

func slowChain(budget time.Duration) *TaskAdapter {
	r := &Route{task: config.TaskBank, budget: budget, health: NewProviderHealth(time.Now)}
	for i, m := range []model.BaseChatModel{
		&countedModel{err: &HTTPError{Status: 429, RetryAfter: time.Minute}},
		&countedModel{err: &HTTPError{Status: 503}},
		slowModel{},
	} {
		r.entries = append(r.entries, routeEntry{model: m, config: config.LLMConfig{Provider: "p" + string(rune('0'+i)), Model: "m"}})
	}
	return &TaskAdapter{route: r}
}

// The bank and the topic step give a whole generation one deadline of the route's budget. Running out of it while
// the chain was failing transiently is the route's own timeout, a busy service, not the caller's deadline.
func TestGenerationBudgetRunningOutIsATransientFailure(t *testing.T) {
	for name, run := range map[string]func(*TaskAdapter, context.Context) error{
		"question bank": func(a *TaskAdapter, ctx context.Context) error {
			_, err := a.Generate(ctx, qd.GenerationInput{}, 10)
			return err
		},
		"roadmap topics": func(a *TaskAdapter, ctx context.Context) error {
			_, err := a.GenerateTopics(ctx, md.TopicInput{})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := run(slowChain(300*time.Millisecond), context.Background())
			if err == nil || llmerr.Reason(err) != "timeout" || !llmerr.Transient(err) {
				t.Fatalf("error = %v (classified reason %q), want a transient timeout", err, llmerr.Reason(err))
			}
		})
	}
}

// The caller's own deadline is still the caller's: it must not be turned into a provider failure.
func TestCallerDeadlineStaysTheCallers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := slowChain(5*time.Second).Generate(ctx, qd.GenerationInput{}, 10)
	if !errors.Is(err, context.DeadlineExceeded) || llmerr.Reason(err) != "" {
		t.Fatalf("error = %v (classified reason %q), want the caller's raw deadline", err, llmerr.Reason(err))
	}
}
