package service

import (
	"context"
	"time"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// ExpiryRepository is the persistence contract of the auto-reject pass. The Repository implementation
// provides it; the service asserts it at call time.
type ExpiryRepository interface {
	AutoRejectExpired(ctx context.Context, now time.Time) (int, error)
}

// ExpiryIDsRepository is the auto-reject pass that also returns the rejected Applications, so each gets
// its notice (KAN-219). The service prefers it over ExpiryRepository when the Repository provides it.
type ExpiryIDsRepository interface {
	AutoRejectExpiredIDs(ctx context.Context, now time.Time) ([]uuid.UUID, error)
}

// AutoRejectExpired runs one expiry pass at now and returns how many Applications it rejected (KAN-214).
// The pass is idempotent: a second run at the same time rejects nothing new.
func (s *Service) AutoRejectExpired(ctx context.Context, now time.Time) (int, error) {
	if idRepository, ok := s.repository.(ExpiryIDsRepository); ok {
		ids, err := idRepository.AutoRejectExpiredIDs(ctx, now)
		if err != nil {
			return 0, err
		}
		s.notify(ctx, noticeAutoRejected, ids...)
		return len(ids), nil
	}
	repository, ok := s.repository.(ExpiryRepository)
	if !ok {
		return 0, apperror.New(apperror.CodeInternal, "application expiry is not configured")
	}
	return repository.AutoRejectExpired(ctx, now)
}
