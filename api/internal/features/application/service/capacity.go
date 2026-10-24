package service

import (
	"context"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// CapacityRepository is the persistence contract of interview capacity. The application Repository
// implements it; it is kept apart from Repository so the CV screening contract stays unchanged.
type CapacityRepository interface {
	GetCapacity(ctx context.Context, recruiterID, postingID uuid.UUID) (domain.CapacityState, error)
}

// capacityRepository returns the capacity contract of the configured repository.
func (s *Service) capacityRepository() (CapacityRepository, error) {
	repo, ok := s.repository.(CapacityRepository)
	if !ok {
		return nil, apperror.New(apperror.CodeInternal, "interview capacity is not available")
	}
	return repo, nil
}

// GetCapacity reads the interview capacity of one of the Recruiter's own postings: slot, approvals,
// consumed interviews, what is still open for approval and, once closed, the refund.
func (s *Service) GetCapacity(ctx context.Context, recruiterID uuid.UUID, role string, postingID uuid.UUID) (domain.CapacityState, error) {
	if role != recruiterRole {
		return domain.CapacityState{}, apperror.New(apperror.CodeForbidden, "only recruiters can read interview capacity")
	}
	repo, err := s.capacityRepository()
	if err != nil {
		return domain.CapacityState{}, err
	}
	return repo.GetCapacity(ctx, recruiterID, postingID)
}
