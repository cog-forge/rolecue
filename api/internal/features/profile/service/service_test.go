package service

import (
	"context"
	"errors"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/profile/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

type profileRepository struct {
	current     domain.Profile
	getCalls    int
	updateCalls int
	id          uuid.UUID
	role        string
	patch       domain.Patch
	getErr      error
	updateErr   error
}

func (r *profileRepository) Get(_ context.Context, id uuid.UUID) (domain.Profile, error) {
	r.getCalls++
	r.id = id
	return r.current, r.getErr
}

func (r *profileRepository) Update(_ context.Context, id uuid.UUID, role string, patch domain.Patch) (domain.Profile, error) {
	r.updateCalls++
	r.id, r.role, r.patch = id, role, patch
	return r.current, r.updateErr
}

func change(value string) domain.StringChange {
	return domain.StringChange{Present: true, Value: &value}
}

func TestUpdateOnboardingRules(t *testing.T) {
	name := change("  Alex  ")
	company := change("  Acme  ")
	website := change("  https://acme.example  ")
	null := domain.StringChange{Present: true}
	for _, tc := range []struct {
		name      string
		role      string
		selected  bool
		completed bool
		patch     domain.Patch
		code      apperror.Code
	}{
		{"candidate completes with only a name", "candidate", true, false, domain.Patch{FullName: name}, ""},
		{"role must be selected first", "candidate", false, false, domain.Patch{FullName: name}, apperror.CodeForbidden},
		{"candidate needs an explicit name", "candidate", true, false, domain.Patch{Image: null}, apperror.CodeValidation},
		{"blank name", "candidate", true, false, domain.Patch{FullName: change(" ")}, apperror.CodeValidation},
		{"null name", "candidate", true, false, domain.Patch{FullName: null}, apperror.CodeValidation},
		{"candidate cannot send company name", "candidate", true, false, domain.Patch{FullName: name, CompanyName: null}, apperror.CodeForbidden},
		{"candidate cannot send company website", "candidate", true, false, domain.Patch{FullName: name, CompanyWebsite: website}, apperror.CodeForbidden},
		{"recruiter completes with all mandatory fields", "recruiter", true, false, domain.Patch{FullName: name, CompanyName: company, CompanyWebsite: website}, ""},
		{"recruiter missing company", "recruiter", true, false, domain.Patch{FullName: name, CompanyWebsite: website}, apperror.CodeValidation},
		{"recruiter null company", "recruiter", true, false, domain.Patch{FullName: name, CompanyName: null, CompanyWebsite: website}, apperror.CodeValidation},
		{"recruiter blank company", "recruiter", true, false, domain.Patch{FullName: name, CompanyName: change(" "), CompanyWebsite: website}, apperror.CodeValidation},
		{"recruiter missing website", "recruiter", true, false, domain.Patch{FullName: name, CompanyName: company}, apperror.CodeValidation},
		{"recruiter null website", "recruiter", true, false, domain.Patch{FullName: name, CompanyName: company, CompanyWebsite: null}, apperror.CodeValidation},
		{"recruiter blank website", "recruiter", true, false, domain.Patch{FullName: name, CompanyName: company, CompanyWebsite: change(" ")}, apperror.CodeValidation},
		{"recruiter HTTP website", "recruiter", true, false, domain.Patch{FullName: name, CompanyName: company, CompanyWebsite: change("http://acme.example")}, apperror.CodeValidation},
		{"recruiter credential URL", "recruiter", true, false, domain.Patch{FullName: name, CompanyName: company, CompanyWebsite: change("https://u:p@acme.example")}, apperror.CodeValidation},
		{"admin bypasses onboarding", "admin", false, false, domain.Patch{FullName: name}, ""},
		{"completed candidate can remove avatar without resending name", "candidate", true, true, domain.Patch{Image: null}, ""},
		{"completed recruiter can clear company fields", "recruiter", true, true, domain.Patch{CompanyName: null, CompanyWebsite: null}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := uuid.New()
			repo := &profileRepository{current: domain.Profile{
				ID: id, Role: tc.role, FullName: "Original", CompanyName: company.Value,
				CompanyWebsite: website.Value, OnboardingRoleSelected: tc.selected, OnboardingCompleted: tc.completed,
			}}
			_, err := NewService(repo).Update(context.Background(), id, tc.role, tc.patch)
			if tc.code != "" {
				var appErr *apperror.AppError
				if !errors.As(err, &appErr) || appErr.Code != tc.code || repo.updateCalls != 0 {
					t.Fatalf("error=%v updates=%d; want %s without writes", err, repo.updateCalls, tc.code)
				}
				return
			}
			if err != nil || repo.getCalls != 1 || repo.updateCalls != 1 || repo.id != id || repo.role != tc.role {
				t.Fatalf("repo=%+v error=%v", repo, err)
			}
			if repo.patch.FullName.Value != nil && *repo.patch.FullName.Value != "Alex" {
				t.Fatalf("name was not normalized: %q", *repo.patch.FullName.Value)
			}
			if repo.patch.CompanyName.Value != nil && *repo.patch.CompanyName.Value != "Acme" {
				t.Fatalf("company was not normalized: %q", *repo.patch.CompanyName.Value)
			}
			if repo.patch.CompanyWebsite.Value != nil && *repo.patch.CompanyWebsite.Value != "https://acme.example" {
				t.Fatalf("website was not normalized: %q", *repo.patch.CompanyWebsite.Value)
			}
		})
	}
}

func TestUpdatePropagatesPersistenceErrors(t *testing.T) {
	sentinel := errors.New("database unavailable")
	for _, stage := range []string{"read", "write"} {
		t.Run(stage, func(t *testing.T) {
			repo := &profileRepository{current: domain.Profile{Role: "candidate", OnboardingRoleSelected: true}}
			wantUpdates := 0
			if stage == "read" {
				repo.getErr = sentinel
			} else {
				repo.updateErr = sentinel
				wantUpdates = 1
			}
			_, err := NewService(repo).Update(context.Background(), uuid.New(), "candidate", domain.Patch{FullName: change("Alex")})
			if !errors.Is(err, sentinel) || repo.updateCalls != wantUpdates {
				t.Fatalf("error=%v updates=%d; want persistence error and %d updates", err, repo.updateCalls, wantUpdates)
			}
		})
	}
}
