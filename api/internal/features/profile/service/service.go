package service

import (
	"context"

	"github.com/cog-forge/rolecue/api/internal/features/profile/domain"
	"github.com/google/uuid"
)

type Repository interface {
	Get(context.Context, uuid.UUID) (domain.Profile, error)
	Update(context.Context, uuid.UUID, string, domain.Patch) (domain.Profile, error)
}

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (domain.Profile, error) {
	return s.repository.Get(ctx, id)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, role string, patch domain.Patch) (domain.Profile, error) {
	if err := validatePatch(&patch, role); err != nil {
		return domain.Profile{}, err
	}
	return s.repository.Update(ctx, id, role, patch)
}
