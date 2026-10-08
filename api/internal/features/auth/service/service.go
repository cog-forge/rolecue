package service

import (
	"context"
	"net/http"

	"github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

type UserReader interface {
	Get(context.Context, uuid.UUID) (domain.User, error)
}
type AuthService interface {
	Authenticate(context.Context, *http.Request) (domain.User, []string, error)
}
type Service struct {
	verifier SessionVerifier
	users    UserReader
}

func NewService(verifier SessionVerifier, users UserReader) AuthService {
	return &Service{verifier: verifier, users: users}
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
