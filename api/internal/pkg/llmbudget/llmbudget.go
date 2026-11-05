// Package llmbudget caps how many LLM-backed operations one user may start per UTC day (KAN-278), so a
// signed-in user cannot run up the provider bill. Every feature that starts an LLM call asks the Budget
// first through the one-method Guard interface:
//
//	if err := guard.Consume(ctx, userID); err != nil { return err }
//
// The count lives in the llm_usage table and is incremented by one atomic upsert that refuses to pass the
// limit, so concurrent requests cannot overshoot it and the cap holds across API instances. Only counts
// are stored and logged, never prompt text or model output.
package llmbudget

import (
	"context"
	"errors"
	"time"

	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// Guard is what features depend on. A nil Guard means "no budget" for features that treat it as optional.
type Guard interface {
	// Consume counts one LLM call for the user. It returns an LLM_BUDGET_EXCEEDED error when today's budget
	// is spent, in which case nothing is counted.
	Consume(ctx context.Context, userID uuid.UUID) error
}

// Store counts a user's calls per UTC day.
type Store interface {
	// Consume adds one call to the user's count for the day unless the count already reached limit (0 means
	// unlimited). ok is false when the budget was already spent; calls is the count after the call.
	Consume(ctx context.Context, userID uuid.UUID, day time.Time, limit int) (calls int, ok bool, err error)
}

// Budget implements Guard on top of a Store.
type Budget struct {
	store Store
	limit int
	now   func() time.Time
}

// New returns a Budget allowing dailyCalls LLM operations per user and UTC day. Zero or less is unlimited:
// calls are still counted but never refused.
func New(store Store, dailyCalls int) *Budget {
	if dailyCalls < 0 {
		dailyCalls = 0
	}
	return &Budget{store: store, limit: dailyCalls, now: time.Now}
}

// DailyLimit is the configured number of calls per user and day (0 = unlimited).
func (b *Budget) DailyLimit() int { return b.limit }

var _ Guard = (*Budget)(nil)

// Consume implements Guard.
func (b *Budget) Consume(ctx context.Context, userID uuid.UUID) error {
	day := b.now().UTC().Truncate(24 * time.Hour)
	calls, ok, err := b.store.Consume(ctx, userID, day, b.limit)
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "Could not check your AI usage. Please try again.", err)
	}
	if !ok {
		logger.FromContext(ctx).Warn("llm daily budget exhausted",
			logger.Op("llmbudget.consume"), logger.String("user_id", userID.String()), logger.Int("limit", b.limit))
		return ErrExceeded()
	}
	if b.limit > 0 && calls == b.limit {
		logger.FromContext(ctx).Info("llm daily budget reached",
			logger.Op("llmbudget.consume"), logger.String("user_id", userID.String()), logger.Int("calls", calls))
	}
	return nil
}

// ErrExceeded is the error a spent budget answers with.
func ErrExceeded() error {
	return apperror.New(apperror.CodeLLMBudgetExceeded, "Daily AI usage limit reached. It resets at midnight UTC.")
}

// IsExceeded reports whether err is a spent budget.
func IsExceeded(err error) bool {
	var appErr *apperror.AppError
	return errors.As(err, &appErr) && appErr != nil && appErr.Code == apperror.CodeLLMBudgetExceeded
}
