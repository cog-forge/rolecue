package service

import (
	"context"
	"errors"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

type roleRepository struct {
	UserRepository
	calls int
	id    uuid.UUID
	role  string
	err   error
}

func (r *roleRepository) SelectRole(_ context.Context, id uuid.UUID, role string) (domain.User, error) {
	r.calls++
	r.id, r.role = id, role
	return domain.User{ID: id, Role: role, OnboardingRoleSelected: true}, r.err
}

func TestSelectRoleAllowlist(t *testing.T) {
	for _, role := range []string{"candidate", "recruiter", "admin", "", "Candidate", " recruiter "} {
		t.Run(role, func(t *testing.T) {
			repo := &roleRepository{}
			svc := NewService(nil, repo)
			id := uuid.New()
			user, err := svc.SelectRole(context.Background(), id, role)
			if role != "candidate" && role != "recruiter" {
				var appErr *apperror.AppError
				if !errors.As(err, &appErr) || appErr.Code != apperror.CodeValidation || repo.calls != 0 {
					t.Fatalf("error=%v calls=%d; want VALIDATION without persistence", err, repo.calls)
				}
				return
			}
			if err != nil || repo.calls != 1 || repo.id != id || repo.role != role || user.ID != id || user.Role != role || !user.OnboardingRoleSelected {
				t.Fatalf("user=%+v repo=%+v error=%v", user, repo, err)
			}
		})
	}
}

func TestSelectRolePropagatesRepositoryRefusal(t *testing.T) {
	refusal := apperror.New(apperror.CodeForbidden, "role cannot be changed")
	repo := &roleRepository{err: refusal}
	_, err := NewService(nil, repo).SelectRole(context.Background(), uuid.New(), "recruiter")
	if !errors.Is(err, refusal) || repo.calls != 1 {
		t.Fatalf("error=%v calls=%d", err, repo.calls)
	}
}
