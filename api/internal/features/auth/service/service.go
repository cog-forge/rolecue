package service

import (
	"context"
	"net/http"

	"github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

type UserRepository interface {
	Get(context.Context, uuid.UUID) (domain.User, error)
	SelectRole(context.Context, uuid.UUID, string) (domain.User, error)
}
type Authenticator interface {
	Authenticate(context.Context, *http.Request) (domain.User, []string, error)
}
type AuthService interface {
	Authenticator
	SelectRole(context.Context, uuid.UUID, string) (domain.User, error)
}
type Service struct {
	verifier SessionVerifier
	users    UserRepository
}

func NewService(verifier SessionVerifier, users UserRepository) AuthService {
	return &Service{verifier: verifier, users: users}
}

func (s *Service) SelectRole(ctx context.Context, id uuid.UUID, role string) (domain.User, error) {
	if role != "candidate" && role != "recruiter" {
		return domain.User{}, apperror.New(apperror.CodeValidation, "role must be candidate or recruiter")
	}
	return s.users.SelectRole(ctx, id, role)
}

func (s *Service) Authenticate(ctx context.Context, req *http.Request) (domain.User, []string, error) {
	verified, err := s.verifier.Verify(ctx, req)
	if err != nil {
		return domain.User{}, verified.SetCookies, err
	}
	// Always read the current application role and lock after session validation.
	user, err := s.users.Get(ctx, verified.UserID)
	if err != nil {
		return domain.User{}, verified.SetCookies, err
	}
	if user.IsLocked || !user.EmailVerified {
		return domain.User{}, verified.SetCookies, apperror.New(apperror.CodeForbidden, "account is locked or email is unverified")
	}
	if user.Role != "candidate" && user.Role != "recruiter" && user.Role != "admin" {
		return domain.User{}, verified.SetCookies, apperror.New(apperror.CodeForbidden, "account role is not permitted")
	}
	return user, verified.SetCookies, nil
}
