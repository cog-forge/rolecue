package service

import (
	"context"
	"errors"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
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
				return questions, nil
			}
		}
		if !isInvalidOutput(err) {
			err = fail(err)
		}
		if attempt >= s.maxRetries || !isInvalidOutput(err) {
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

func isInvalidOutput(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var appErr *apperror.AppError
	return errors.As(err, &appErr) && appErr != nil && appErr.Code == apperror.CodeInvalidQuestionGenerationOutput
}
