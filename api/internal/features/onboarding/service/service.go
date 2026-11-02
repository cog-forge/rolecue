package service

import (
	"context"

	"github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/google/uuid"
)

type RoleSelector interface {
	SelectRole(context.Context, uuid.UUID, string) (domain.User, error)
}
type Service struct{ auth RoleSelector }

func NewService(auth RoleSelector) *Service { return &Service{auth: auth} }
func (s *Service) SelectRole(ctx context.Context, id uuid.UUID, role string) (domain.User, error) {
	user, err := s.auth.SelectRole(ctx, id, role)
	if err != nil {
		return domain.User{}, err
	}
	logger.FromContext(ctx).Info("onboarding role selected",
		logger.Op("onboarding.select_role"),
		logger.String("user_id", id.String()),
		logger.String("role", role),
	)
	return user, nil
}
