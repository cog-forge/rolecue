package service

import (
	"context"
	"errors"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/voice/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

type fakeRepository struct {
	err    error
	calls  int
	filter domain.ListFilter
}

func (f *fakeRepository) ListActive(_ context.Context, filter domain.ListFilter) ([]domain.Voice, int64, error) {
	f.calls++
	f.filter = filter
	if f.err != nil {
		return nil, 0, f.err
	}
	return []domain.Voice{{ID: uuid.New(), Name: "Ava"}}, 1, nil
}

func assertCode(t *testing.T, err error, code apperror.Code) {
	t.Helper()
	var got *apperror.AppError
	if !errors.As(err, &got) || got.Code != code {
		t.Fatalf("error = %v, want code %s", err, code)
	}
}

func newService(t *testing.T, repo *fakeRepository) *Service {
	t.Helper()
	svc, err := New(repo)
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func TestListActiveForCandidatesAndRecruiters(t *testing.T) {
	for _, role := range []string{"candidate", "recruiter"} {
		repo := &fakeRepository{}
		voices, total, err := newService(t, repo).ListActive(context.Background(), role, domain.ListFilter{Limit: 20, Offset: 40})
		if err != nil {
			t.Fatalf("%s: %v", role, err)
		}
		if len(voices) != 1 || total != 1 {
			t.Fatalf("%s: got %d voices, total %d", role, len(voices), total)
		}
		if repo.filter != (domain.ListFilter{Limit: 20, Offset: 40}) {
			t.Fatalf("%s: filter = %+v", role, repo.filter)
		}
	}
}

func TestListActiveRefusesOtherRoles(t *testing.T) {
	for _, role := range []string{"admin", "", "owner"} {
		repo := &fakeRepository{}
		_, _, err := newService(t, repo).ListActive(context.Background(), role, domain.ListFilter{Limit: 20})
		assertCode(t, err, apperror.CodeForbidden)
		if repo.calls != 0 {
			t.Fatalf("%q: repository was called", role)
		}
	}
}

func TestListActiveRejectsBadPagination(t *testing.T) {
	for _, filter := range []domain.ListFilter{{Limit: 0}, {Limit: 101}, {Limit: 20, Offset: -1}} {
		repo := &fakeRepository{}
		_, _, err := newService(t, repo).ListActive(context.Background(), "recruiter", filter)
		assertCode(t, err, apperror.CodeValidation)
		if repo.calls != 0 {
			t.Fatalf("%+v: repository was called", filter)
		}
	}
}

func TestListActivePassesRepositoryErrorsThrough(t *testing.T) {
	want := apperror.New(apperror.CodeInternal, "voice persistence failed")
	_, _, err := newService(t, &fakeRepository{err: want}).ListActive(context.Background(), "candidate", domain.ListFilter{Limit: 20})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}
