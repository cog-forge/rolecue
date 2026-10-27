package service

import (
	"context"
	"time"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

// ExpiryRepository is the persistence contract of the auto-reject pass. The Repository implementation
// provides it; the service asserts it at call time.
type ExpiryRepository interface {
	AutoRejectExpired(ctx context.Context, now time.Time) (int, error)
}

// AutoRejectExpired runs one expiry pass at now and returns how many Applications it rejected (KAN-214).
// The pass is idempotent: a second run at the same time rejects nothing new.
func (s *Service) AutoRejectExpired(ctx context.Context, now time.Time) (int, error) {
	repository, ok := s.repository.(ExpiryRepository)
	if !ok {
		return 0, apperror.New(apperror.CodeInternal, "application expiry is not configured")
	}
	return repository.AutoRejectExpired(ctx, now)
}
