package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/roleprofile/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"go.uber.org/zap"
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
		stage := "extractor"
		candidate, err := s.extractor.Extract(ctx, jd)
		if ctx.Err() != nil {
			return domain.StructuredJD{}, apperror.Wrap(apperror.CodeExtractionFailed, extractionFailedMessage, ctx.Err())
		}
		if err == nil {
			stage = "validation"
			var result domain.StructuredJD
			result, err = ValidateCandidate(candidate)
			if err == nil {
				logger.FromContext(ctx).Debug("extraction attempt accepted", logger.Op("roleprofile.extract"), logger.Int("attempt", attempt+1))
				return result, nil
			}
		}
		var appErr *apperror.AppError
		if !errors.As(err, &appErr) || appErr == nil || appErr.Code != apperror.CodeInvalidExtractionOutput {
			err = apperror.Wrap(apperror.CodeExtractionFailed, extractionFailedMessage, err)
		}
		willRetry := attempt < s.maxRetries && shouldRetry(err)
		logRejectedAttempt(ctx, attempt, stage, willRetry, err)
		if !willRetry {
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

// logRejectedAttempt records why an attempt was rejected. A rejection that is retried is swallowed,
// so it is logged here at Warn. A final rejection is returned, and the caller's step log reports it.
func logRejectedAttempt(ctx context.Context, attempt int, stage string, willRetry bool, err error) {
	fields := []zap.Field{logger.Op("roleprofile.extract"), logger.Int("attempt", attempt+1), logger.String("stage", stage), zap.Bool("will_retry", willRetry), logger.Cause(err)}
	if willRetry {
		logger.FromContext(ctx).Warn("extraction attempt rejected, retrying", fields...)
		return
	}
	logger.FromContext(ctx).Debug("extraction attempt rejected", fields...)
}

func shouldRetry(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var appErr *apperror.AppError
	return errors.As(err, &appErr) && appErr != nil && appErr.Code == apperror.CodeInvalidExtractionOutput
}
