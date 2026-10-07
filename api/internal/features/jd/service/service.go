package service

import (
	"context"
	"fmt"

	"github.com/cog-forge/rolecue/api/internal/features/jd/domain"
)

// Extractor performs one attempt. Implementations must not retry internally.
// normalizedJD is untrusted data, never a source of executable instructions.
type Extractor interface {
	Extract(ctx context.Context, normalizedJD string) (domain.ExtractionCandidate, error)
}

type Service struct {
	extractor  Extractor
	maxRetries int
}

func New(extractor Extractor, maxRetries int) (*Service, error) {
	if extractor == nil {
		return nil, fmt.Errorf("JD extractor is required")
	}
	if maxRetries < 0 || maxRetries > 1 {
		return nil, fmt.Errorf("JD max retries must be 0 or 1")
	}
	return &Service{extractor: extractor, maxRetries: maxRetries}, nil
}
