package service

import (
	"context"
	"errors"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"go.uber.org/zap"
)

const generationFailedMessage = "Question generation failed."

// generateWithRetry mirrors the JD extraction retry: one shared budget that includes proposals that
// fail validation, a retry only for invalid output, and invalid output is never fed back to the model.
func (s *Service) generateWithRetry(ctx context.Context, in domain.GenerationInput) ([]string, error) {
	fail := func(cause error) error {
		return apperror.Wrap(apperror.CodeQuestionGenerationFailed, generationFailedMessage, cause)
	}
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, fail(err)
		}
		candidate, err := s.generator.Generate(ctx, in, s.size)
		if ctx.Err() != nil {
			return nil, fail(ctx.Err())
		}
		if err == nil {
			var questions []string
			questions, err = validateCandidate(candidate, s.size)
			if err == nil {
				logger.FromContext(ctx).Debug("question generation attempt accepted", logger.Op("questionbank.generate"), logger.Int("attempt", attempt+1))
				return questions, nil
			}
		}
		if !isInvalidOutput(err) {
			err = fail(err)
		}
		willRetry := attempt < s.maxRetries && isInvalidOutput(err)
		logRejectedAttempt(ctx, attempt, willRetry, len(candidate.Questions), err)
		if !willRetry {
			return nil, err
		}
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, fail(ctx.Err())
		case <-timer.C:
		}
	}
}

// logRejectedAttempt records a rejected proposal with its rule and counts, never the question text.
// A retried rejection is swallowed, so it is logged at Warn; a final one is returned to the caller.
func logRejectedAttempt(ctx context.Context, attempt int, willRetry bool, proposed int, err error) {
	fields := []zap.Field{logger.Op("questionbank.generate"), logger.Int("attempt", attempt+1), zap.Bool("will_retry", willRetry),
		logger.Int("count", proposed), logger.Cause(err)}
	if willRetry {
		logger.FromContext(ctx).Warn("question generation attempt rejected, retrying", fields...)
		return
	}
	logger.FromContext(ctx).Debug("question generation attempt rejected", fields...)
}

func isInvalidOutput(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var appErr *apperror.AppError
	return errors.As(err, &appErr) && appErr != nil && appErr.Code == apperror.CodeInvalidQuestionGenerationOutput
}
