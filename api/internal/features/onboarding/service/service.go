package service

import (
	"context"

	"github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/google/uuid"
)

type RoleSelector interface {
	SelectRole(context.Context, uuid.UUID, string) (domain.User, error)
}
type Service struct{ auth RoleSelector }

func NewService(auth RoleSelector) *Service { return &Service{auth: auth} }
func (s *Service) SelectRole(ctx context.Context, id uuid.UUID, role string) (domain.User, error) {
	return s.auth.SelectRole(ctx, id, role)
}
