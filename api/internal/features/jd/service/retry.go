package service

import (
	"context"
	"errors"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/jd/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

const extractionFailedMessage = "Job description extraction failed."

// The service owns one shared retry budget, including domain-invalid proposals.
func (s *Service) extractWithRetry(ctx context.Context, jd string) (domain.StructuredJD, error) {
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return domain.StructuredJD{}, apperror.Wrap(apperror.CodeExtractionFailed, extractionFailedMessage, err)
		}
		candidate, err := s.extractor.Extract(ctx, jd)
		if ctx.Err() != nil {
			return domain.StructuredJD{}, apperror.Wrap(apperror.CodeExtractionFailed, extractionFailedMessage, ctx.Err())
		}
		if err == nil {
			var result domain.StructuredJD
			result, err = ValidateCandidate(candidate)
			if err == nil {
				return result, nil
			}
		}
		var appErr *apperror.AppError
		if !errors.As(err, &appErr) ||
			appErr == nil ||
			appErr.Code != apperror.CodeInvalidExtractionOutput {
			err = apperror.Wrap(apperror.CodeExtractionFailed, extractionFailedMessage, err)
		}
		if attempt >= s.maxRetries || !shouldRetry(err) {
			return domain.StructuredJD{}, err
		}
		// Never replay invalid output as instructions.
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return domain.StructuredJD{}, apperror.Wrap(apperror.CodeExtractionFailed, extractionFailedMessage, ctx.Err())
		case <-timer.C:
		}
	}
}

func shouldRetry(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var appErr *apperror.AppError
	return errors.As(err, &appErr) && appErr != nil &&
		appErr.Code == apperror.CodeInvalidExtractionOutput
}
