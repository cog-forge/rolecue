package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/cog-forge/rolecue/api/internal/config"
	qd "github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	md "github.com/cog-forge/rolecue/api/internal/features/roadmap/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/llmerr"
)

// retryPolicy retries without waiting and without a minimum budget, so a test controls the count only.
func retryPolicy(retries int) *llmerr.Policy {
	return &llmerr.Policy{MaxRetries: retries, Retryable: llmerr.TransientOrInvalid,
		Sleep: func(context.Context, time.Duration) bool { return true }}
}

func topicsReply(n int) *schema.Message {
	topics := make([]map[string]string, n)
	for i := range topics {
		topics[i] = map[string]string{"title": fmt.Sprintf("Topic %d", i), "description": "Covers the area."}
	}
	body, _ := json.Marshal(map[string]any{"topics": topics})
	return schema.AssistantMessage(string(body), nil)
}

func scriptedModel(calls *atomic.Int32, script func(call int) (*schema.Message, error)) replyModel {
	return replyModel{generate: func([]*schema.Message) (*schema.Message, error) {
		return script(int(calls.Add(1)))
	}}
}

func bankAdapter(m replyModel, retries int) *TaskAdapter {
	return &TaskAdapter{retry: retryPolicy(retries), route: &Route{task: config.TaskBank, budget: 5 * time.Second,
		entries: []routeEntry{{model: m, config: config.LLMConfig{Provider: "groq"}}}}}
}

func TestGenerateTopicsRepeatsATransientRouteFailureThenSucceeds(t *testing.T) {
	for name, failure := range map[string]error{"503": &HTTPError{Status: 503}, "429": &HTTPError{Status: 429}, "malformed": ErrInvalidOutput} {
		t.Run(name, func(t *testing.T) {
			var calls atomic.Int32
			m := scriptedModel(&calls, func(call int) (*schema.Message, error) {
				if call == 1 {
					if failure == ErrInvalidOutput {
						return schema.AssistantMessage(`{"topics":[{"title":"Cut`, nil), nil // truncated: unexpected EOF
					}
					return nil, failure
				}
				return topicsReply(5), nil
			})
			got, err := bankAdapter(m, 2).GenerateTopics(context.Background(), md.TopicInput{})
			if err != nil || len(got.Topics) != 5 || calls.Load() != 2 {
				t.Fatalf("topics=%d calls=%d err=%v", len(got.Topics), calls.Load(), err)
			}
		})
	}
}

func TestGenerateTopicsStopsAtTheRetryLimitAndNeverRepeatsAPermanentFailure(t *testing.T) {
	var calls atomic.Int32
	always := scriptedModel(&calls, func(int) (*schema.Message, error) { return nil, &HTTPError{Status: 503} })
	_, err := bankAdapter(always, 2).GenerateTopics(context.Background(), md.TopicInput{})
	if llmerr.Reason(err) != "http_5xx" || calls.Load() != 3 {
		t.Fatalf("exhaustion: calls=%d err=%v, want 3 calls and the classified failure", calls.Load(), err)
	}
	calls.Store(0)
	auth := scriptedModel(&calls, func(int) (*schema.Message, error) { return nil, &HTTPError{Status: 401} })
	if _, err = bankAdapter(auth, 2).GenerateTopics(context.Background(), md.TopicInput{}); err == nil || calls.Load() != 1 {
		t.Fatalf("auth rejection: calls=%d err=%v, want one call", calls.Load(), err)
	}
}

// A batch that failed on every provider is repeated alone: batch one is kept and not asked again, and the
// bank that comes back is the same as a run without a failure.
func TestBankRepeatsOnlyTheFailedBatch(t *testing.T) {
	var calls atomic.Int32
	asked := map[int]int{} // batch index (by existing-question count) -> times asked
	m := replyModel{generate: func(in []*schema.Message) (*schema.Message, error) {
		call := int(calls.Add(1))
		var data struct {
			Existing []string `json:"untrusted_existing_questions"`
		}
		if err := json.Unmarshal([]byte(in[1].Content), &data); err != nil {
			t.Fatal(err)
		}
		batch := len(data.Existing) / 10
		asked[batch]++
		if call == 2 {
			return nil, &HTTPError{Status: 504}
		}
		questions := make([]string, 10)
		for i := range questions {
			questions[i] = fmt.Sprintf("Question %d?", batch*10+i)
		}
		body, _ := json.Marshal(map[string]any{"questions": questions})
		return schema.AssistantMessage(string(body), nil), nil
	}}
	bank, err := bankAdapter(m, 2).Generate(context.Background(), qd.GenerationInput{}, 30)
	if err != nil || len(bank.Questions) != 30 {
		t.Fatalf("questions=%d err=%v", len(bank.Questions), err)
	}
	if asked[0] != 1 || asked[1] != 2 || asked[2] != 1 || calls.Load() != 4 {
		t.Fatalf("asked per batch = %v (calls %d), want only batch 1 asked twice", asked, calls.Load())
	}
	seen := map[string]bool{}
	for _, q := range bank.Questions {
		if seen[q.Content] {
			t.Fatalf("duplicate question %q after a repeated batch", q.Content)
		}
		seen[q.Content] = true
	}
}

func TestBankFailsClearlyWhenEveryRepeatFails(t *testing.T) {
	var calls atomic.Int32
	m := scriptedModel(&calls, func(int) (*schema.Message, error) { return nil, &HTTPError{Status: 503} })
	if _, err := bankAdapter(m, 2).Generate(context.Background(), qd.GenerationInput{}, 30); err == nil || calls.Load() != 3 {
		t.Fatalf("calls=%d err=%v, want 3 calls then an error", calls.Load(), err)
	}
}

// The repeat never outlives the call's absolute budget: a budget with no room left fails after one attempt.
func TestGenerationRepeatNeedsRoomInTheBudget(t *testing.T) {
	var calls atomic.Int32
	m := scriptedModel(&calls, func(int) (*schema.Message, error) { return nil, &HTTPError{Status: 503} })
	a := bankAdapter(m, 2)
	a.retry.MinRemaining = time.Minute
	if _, err := a.GenerateTopics(context.Background(), md.TopicInput{}); err == nil || calls.Load() != 1 {
		t.Fatalf("calls=%d err=%v, want one attempt when the budget has no room", calls.Load(), err)
	}
	// An already expired caller deadline makes no call at all and is not a transient failure.
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond)
	calls.Store(0)
	if _, err := bankAdapter(m, 2).GenerateTopics(ctx, md.TopicInput{}); err == nil || calls.Load() != 0 {
		t.Fatalf("expired caller: calls=%d err=%v", calls.Load(), err)
	}
}
