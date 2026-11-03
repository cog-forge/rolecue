package ai

import (
	"context"
	"errors"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cog-forge/rolecue/api/internal/config"
)

var ErrInvalidOutput = errors.New("invalid structured output")

type routeEntry struct {
	model  model.BaseChatModel
	config config.LLMConfig
}
type Route struct {
	legacy  bool
	task    config.Task
	budget  time.Duration
	entries []routeEntry
}

func NewRoute(ctx context.Context, cfg *config.Config, task config.Task) (*Route, error) {
	primary, fallback := cfg.TaskLLM(task)
	r := &Route{task: task, budget: primary.Timeout, legacy: cfg.AI.Route(task) == (config.TaskRoute{})}
	if task == config.TaskFollowUp && r.budget > 5*time.Second {
		r.budget = 5 * time.Second
	}
	for _, c := range append([]config.LLMConfig{primary}, fallback...) {
		m, err := NewChatModel(ctx, c)
		if err != nil {
			return nil, err
		}
		// The task attempt log includes validation; avoid duplicate success logs before validation.
		if logged, ok := m.(*loggedModel); ok && !r.legacy {
			m = logged.inner
		}
		r.entries = append(r.entries, routeEntry{m, c})
	}
	return r, nil
}

// Run includes decoding and existing domain validation in every attempt. Failed outputs are never
// replayed to another provider. Timeouts, retries and fallback share one absolute task deadline.
func Run[T any](ctx context.Context, r *Route, call func(context.Context, model.BaseChatModel) (T, error)) (T, error) {
	var zero T
	if r.legacy {
		return call(ctx, r.entries[0].model)
	}
	ctx, cancel := context.WithTimeout(ctx, r.budget)
	defer cancel()
	var last error
	for index, entry := range r.entries {
		for attempt := 0; attempt <= entry.config.MaxRetries; attempt++ {
			if err := ctx.Err(); err != nil {
				return zero, err
			}
			// Reserve a fair share for remaining providers/retries, especially inside the live 5s budget.
			deadline, _ := ctx.Deadline()
			slots := len(r.entries) - index + entry.config.MaxRetries - attempt
			duration := time.Until(deadline) / time.Duration(slots)
			attemptCtx, stop := context.WithTimeout(ctx, duration)
			lm := &loggedModel{inner: entry.model, provider: entry.config.Provider, model: entry.config.Model, task: string(r.task)}
			done := lm.startAttempt(attemptCtx)
			result, err := call(attemptCtx, lm)
			if attemptCtx.Err() != nil {
				err = attemptCtx.Err()
			}
			done(sanitizedFailure(err), failureFields(err)...)
			stop()
			if err == nil {
				return result, nil
			}
			last = err
			if ctx.Err() != nil {
				return zero, ctx.Err()
			}
			if !canFallback(err) {
				return zero, sanitizedFailure(err)
			}
			// 429 is an immediate fallback, never an unbounded sleep or a same-provider retry.
			if llmFailureReason(err) == "rate_limited" {
				break
			}
		}
	}
	return zero, sanitizedFailure(last)
}
func canFallback(err error) bool {
	switch llmFailureReason(err) {
	case "timeout", "rate_limited", "http_5xx", "network", "invalid_output":
		return true
	}
	return false
}
