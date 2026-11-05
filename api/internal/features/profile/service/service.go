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
	imageHosts []string
	repository Repository
}

// Option configures the Service.
type Option func(*Service)

// WithImageHosts sets the hosts a profile image URL may point to. Without it the sign-in providers' avatar
// hosts are the only ones allowed.
func WithImageHosts(hosts []string) Option {
	return func(s *Service) { s.imageHosts = hosts }
}

func NewService(repository Repository, opts ...Option) *Service {
	s := &Service{repository: repository, imageHosts: []string{"lh3.googleusercontent.com", "avatars.githubusercontent.com"}}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (domain.Profile, error) {
	return s.repository.Get(ctx, id)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, role string, patch domain.Patch) (domain.Profile, error) {
	if err := validatePatch(&patch, role, s.imageHosts); err != nil {
		return domain.Profile{}, err
	}
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return domain.Profile{}, err
	}
	if err := validateOnboarding(current, patch); err != nil {
		return domain.Profile{}, err
	}
	return s.repository.Update(ctx, id, role, patch)
}
