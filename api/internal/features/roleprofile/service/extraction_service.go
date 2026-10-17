package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/roleprofile/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

// Extractor performs one attempt. Implementations must not retry internally.
// normalizedJD is untrusted data, never a source of executable instructions.
type Extractor interface {
	Extract(ctx context.Context, normalizedJD string) (domain.ExtractionCandidate, error)
}

type ExtractionService struct {
	extractor  Extractor
	maxRetries int
}

func NewExtractionService(extractor Extractor, maxRetries int) (*ExtractionService, error) {
	if extractor == nil {
		return nil, fmt.Errorf("JD extractor is required")
	}
	if maxRetries < 0 || maxRetries > 1 {
		return nil, fmt.Errorf("JD max retries must be 0 or 1")
	}
	return &ExtractionService{extractor: extractor, maxRetries: maxRetries}, nil
}

func (s *ExtractionService) Extract(ctx context.Context, raw string) (domain.StructuredJD, error) {
	jd, err := NormalizeInput(raw)
	if err != nil {
		return domain.StructuredJD{}, err
	}
	return s.extractWithRetry(ctx, jd)
}

const extractionFailedMessage = "Job description extraction failed."

// The service owns one shared retry budget, including domain-invalid proposals.
func (s *ExtractionService) extractWithRetry(ctx context.Context, jd string) (domain.StructuredJD, error) {
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
		if !errors.As(err, &appErr) || appErr == nil || appErr.Code != apperror.CodeInvalidExtractionOutput {
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
	return errors.As(err, &appErr) && appErr != nil && appErr.Code == apperror.CodeInvalidExtractionOutput
}
