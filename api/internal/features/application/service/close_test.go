package service

import (
	"context"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

func TestCloseIntakeDelegatesForRecruiter(t *testing.T) {
	want := domain.IntakeClosure{JobPostingID: postingID, Status: "intake_closed", RejectedCount: 2}
	repo := &fakeRepository{closure: want}
	got, err := newService(t, repo).CloseIntake(context.Background(), recruiterID, "recruiter", postingID)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("CloseIntake() = %+v, want %+v", got, want)
	}
	if repo.closeCalls != 1 || repo.closeRecruiter != recruiterID || repo.closePosting != postingID {
		t.Fatalf("repository called %d times with recruiter %s posting %s, want the caller's IDs",
			repo.closeCalls, repo.closeRecruiter, repo.closePosting)
	}
}

func TestOnlyRecruitersCloseIntake(t *testing.T) {
	for _, role := range []string{"candidate", "admin", "", "Recruiter"} {
		repo := &fakeRepository{}
		_, err := newService(t, repo).CloseIntake(context.Background(), recruiterID, role, postingID)
		assertCode(t, err, apperror.CodeForbidden)
		if repo.closeCalls != 0 {
			t.Fatalf("role %q reached the repository", role)
		}
	}
}

func TestCloseIntakePassesRepositoryErrorsThrough(t *testing.T) {
	for _, want := range []*apperror.AppError{
		apperror.New(apperror.CodeJobPostingNotFound, "job posting not found"),
		domain.ErrIntakeNotOpen,
	} {
		_, err := newService(t, &fakeRepository{err: want}).CloseIntake(context.Background(), recruiterID, "recruiter", postingID)
		assertCode(t, err, want.Code)
		if err != want {
			t.Fatalf("error = %v, want the repository error %v unchanged", err, want)
		}
	}
}
