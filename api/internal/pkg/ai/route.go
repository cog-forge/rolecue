package ai

import (
	"context"
	"errors"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
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
	// health holds the 429 cooldowns shared by every route of the process (KAN-386). Nil turns cooldowns off.
	health *ProviderHealth
}

func NewRoute(ctx context.Context, cfg *config.Config, task config.Task) (*Route, error) {
	primary, fallback := cfg.TaskLLM(task)
	r := &Route{task: task, budget: primary.Timeout, legacy: cfg.AI.Route(task) == (config.TaskRoute{}), health: sharedHealth}
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
	parent := ctx
	ctx, cancel := context.WithTimeout(ctx, r.budget)
	defer cancel()
	// The route's own budget running out is a timeout of the provider call, which a caller may retry; the
	// caller's own deadline or cancellation stays what it is.
	expired := func(err error) error {
		if parent.Err() == nil && errors.Is(err, context.DeadlineExceeded) {
			return &providerFailure{reason: "timeout"}
		}
		return err
	}
	cooling := func(e routeEntry) bool { return r.cooling(e) > 0 }
	// kept is the most informative failure so far (see moreInformative); it is what an exhausted chain returns.
	var kept error
	for index, entry := range r.entries {
		if remaining := r.cooling(entry); remaining > 0 {
			logger.FromContext(ctx).Info("llm provider cooling down, skipped", logger.Op("ai.route"), logger.String("task", string(r.task)),
				logger.String("provider", entry.config.Provider), logger.String("model", entry.config.Model),
				logger.Int("remaining_seconds", int(remaining.Round(time.Second)/time.Second)))
			// A skipped entry is a rate-limited provider that was not called. It counts as that failure, so a chain
			// whose other entries fail with something less informative is still reported as busy, and a chain
			// that is entirely cooling fails at once without a call.
			kept = moreInformative(kept, &providerFailure{reason: "rate_limited"})
			continue
		}
		for attempt := 0; attempt <= entry.config.MaxRetries; attempt++ {
			if err := ctx.Err(); err != nil {
				return zero, expired(err)
			}
			// Reserve a fair share for remaining providers/retries, especially inside the live 5s budget.
			deadline, _ := ctx.Deadline()
			slots := attemptSlots(r.entries, index, attempt, cooling)
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
			if ctx.Err() != nil {
				return zero, expired(ctx.Err())
			}
			reason := llmFailureReason(err)
			clientError := reason == "auth_rejected" || reason == "http_4xx"
			if !canFallback(err) && !clientError {
				// Cancellation, or an error that is not a provider answer: stop at once.
				return zero, sanitizedFailure(err)
			}
			kept = moreInformative(kept, err)
			if clientError {
				// The provider refused this call (rejected key, bad request, no credit, unknown model, restricted
				// account). That says nothing about the providers behind it, so go on to the next entry, without
				// retrying this one and without a cooldown.
				logger.FromContext(ctx).Warn("llm provider rejected the call, trying the next entry", logger.Op("ai.route"),
					logger.String("task", string(r.task)), logger.String("provider", entry.config.Provider),
					logger.String("model", entry.config.Model), logger.String("reason", reason), logger.Int("status_code", statusOf(err)))
				break
			}
			// A 429 starts a cooldown for this provider and model and goes to the next entry at once: never a
			// sleep, never a same-provider retry.
			if reason == "rate_limited" {
				if r.health != nil {
					r.health.RateLimited(entryKey(entry.config), retryAfterOf(err))
				}
				break
			}
		}
	}
	return zero, sanitizedFailure(kept)
}

// cooling reports how long the entry must still be left alone.
func (r *Route) cooling(e routeEntry) time.Duration {
	if r.health == nil {
		return 0
	}
	return r.health.Cooling(entryKey(e.config))
}

// attemptSlots counts the attempts the time left must be shared between when entry index tries attempt: this
// attempt, the retries this entry still has, and one for each later entry that is not cooling down. A cooling
// entry will be skipped, so its share goes to the entries before it.
func attemptSlots(entries []routeEntry, index, attempt int, cooling func(routeEntry) bool) int {
	slots := 1 + entries[index].config.MaxRetries - attempt
	for _, later := range entries[index+1:] {
		if !cooling(later) {
			slots++
		}
	}
	return slots
}
func canFallback(err error) bool {
	switch llmFailureReason(err) {
	case "timeout", "rate_limited", "http_5xx", "network", "invalid_output":
		return true
	}
	return false
}

// moreInformative returns the failure a caller should see when several entries failed: a transient one (timeout,
// 429, 5xx, network) beats an unusable reply, which beats a client error, and among equals the later one wins. A
// congestion error must not be hidden behind a configuration-looking one.
func moreInformative(kept, err error) error {
	if kept == nil || failureRank(err) >= failureRank(kept) {
		return err
	}
	return kept
}

func failureRank(err error) int {
	switch llmFailureReason(err) {
	case "timeout", "rate_limited", "http_5xx", "network":
		return 3
	case "invalid_output":
		return 2
	}
	return 1
}

// statusOf returns the HTTP status a failure carries, or 0.
func statusOf(err error) int {
	var safe *providerFailure
	if errors.As(err, &safe) && safe.status != 0 {
		return safe.status
	}
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.Status
	}
	return 0
}
