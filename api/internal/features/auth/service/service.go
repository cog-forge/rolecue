package service

import (
	"context"
	"fmt"
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

// Authenticate returns the current user for a valid session. The rejection reasons (locked, unverified,
// role not permitted) travel in the error chain, which the response layer logs; the client message is unchanged.
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
		cause := fmt.Errorf("locked=%t email_verified=%t user_id=%s", user.IsLocked, user.EmailVerified, user.ID)
		return domain.User{}, verified.SetCookies, apperror.Wrap(apperror.CodeForbidden, "account is locked or email is unverified", cause)
	}
	if user.Role != "candidate" && user.Role != "recruiter" && user.Role != "admin" {
		cause := fmt.Errorf("role %q is not permitted user_id=%s", user.Role, user.ID)
		return domain.User{}, verified.SetCookies, apperror.Wrap(apperror.CodeForbidden, "account role is not permitted", cause)
	}
	return user, verified.SetCookies, nil
}
