package auth

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

type User struct {
	ID            uuid.UUID `json:"id"`
	Email         string    `json:"email"`
	FullName      string    `json:"full_name"`
	Role          string    `json:"role"`
	EmailVerified bool      `json:"email_verified"`
	IsLocked      bool      `json:"is_locked"`
}

type UserReader interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}
type AuthService interface {
	Authenticate(context.Context, *http.Request) (User, []string, error)
}
type Service struct {
	verifier SessionVerifier
	users    UserReader
}

func NewService(verifier SessionVerifier, users UserReader) AuthService {
	return &Service{verifier: verifier, users: users}
}

func (s *Service) Authenticate(ctx context.Context, req *http.Request) (User, []string, error) {
	verified, err := s.verifier.Verify(ctx, req)
	if err != nil {
		return User{}, verified.SetCookies, err
	}
	var user User
	// Always read the current application role and lock after SDK validation.
	err = s.users.QueryRow(ctx, `SELECT id,email,name,coalesce(role,'candidate'),email_verified,coalesce(is_locked,false) FROM public.users WHERE id=$1`, verified.UserID).Scan(&user.ID, &user.Email, &user.FullName, &user.Role, &user.EmailVerified, &user.IsLocked)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, verified.SetCookies, ErrInvalidSession
	}
	if err != nil {
		return User{}, verified.SetCookies, ErrUnavailable
	}
	if user.IsLocked || !user.EmailVerified {
		return User{}, verified.SetCookies, apperror.New(apperror.CodeForbidden, "account is locked or email is unverified")
	}
	if user.Role != "candidate" && user.Role != "recruiter" && user.Role != "admin" {
		return User{}, verified.SetCookies, apperror.New(apperror.CodeForbidden, "account role is not permitted")
	}
	return user, verified.SetCookies, nil
}
