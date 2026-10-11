package service

import (
	"context"
	"errors"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/admin/moderation/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// fakeRepository behaves like the SQL guard: Decide only changes a pending posting.
type fakeRepository struct {
	posting   *domain.ReviewPosting
	listErr   error
	getErr    error
	decideErr error

	listCalls, getCalls, decideCalls int
	filter                           domain.ListFilter
	decidedStatus                    domain.PostingStatus
	decidedReason                    *string
}

func (f *fakeRepository) List(_ context.Context, filter domain.ListFilter) ([]domain.PostingSummary, int64, error) {
	f.listCalls++
	f.filter = filter
	if f.listErr != nil {
		return nil, 0, f.listErr
	}
	return []domain.PostingSummary{{ID: uuid.New()}}, 1, nil
}

func (f *fakeRepository) Get(_ context.Context, _ uuid.UUID) (domain.ReviewPosting, error) {
	f.getCalls++
	if f.getErr != nil {
		return domain.ReviewPosting{}, f.getErr
	}
	if f.posting == nil {
		return domain.ReviewPosting{}, apperror.New(apperror.CodeJobPostingNotFound, "job posting not found")
	}
	return *f.posting, nil
}

func (f *fakeRepository) Decide(_ context.Context, _ uuid.UUID, status domain.PostingStatus, reason *string) error {
	f.decideCalls++
	if f.decideErr != nil {
		return f.decideErr
	}
	if f.posting == nil || f.posting.Status != domain.PostingStatusPending {
		return domain.ErrNotPending
	}
	f.decidedStatus, f.decidedReason = status, reason
	f.posting.Status, f.posting.RejectReason = status, reason
	return nil
}

func newService(t *testing.T, repo Repository) *Service {
	t.Helper()
	s, err := New(repo, nil)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// newObservedService records log entries so tests can assert the audit line.
func newObservedService(t *testing.T, repo Repository) (*Service, *observer.ObservedLogs) {
	t.Helper()
	core, logs := observer.New(zap.InfoLevel)
	s, err := New(repo, &logger.Logger{Logger: zap.New(core)})
	if err != nil {
		t.Fatal(err)
	}
	return s, logs
}

func postingIn(status domain.PostingStatus) *fakeRepository {
	return &fakeRepository{posting: &domain.ReviewPosting{ID: uuid.New(), Status: status}}
}

func assertCode(t *testing.T, err error, code apperror.Code) {
	t.Helper()
	var got *apperror.AppError
	if !errors.As(err, &got) || got.Code != code {
		t.Fatalf("error = %v, want code %s", err, code)
	}
}

func TestNewRequiresRepository(t *testing.T) {
	if _, err := New(nil, nil); err == nil {
		t.Fatal("New accepted a nil repository")
	}
}

func TestNonAdminsAreForbidden(t *testing.T) {
	for _, role := range []string{"recruiter", "candidate", ""} {
		t.Run(role, func(t *testing.T) {
			repo := postingIn(domain.PostingStatusPending)
			s := newService(t, repo)
			ctx, id := context.Background(), uuid.New()
			_, _, err := s.List(ctx, role, domain.ListFilter{Limit: 20})
			assertCode(t, err, apperror.CodeForbidden)
			_, err = s.Get(ctx, role, id)
			assertCode(t, err, apperror.CodeForbidden)
			_, err = s.Approve(ctx, uuid.New(), role, id)
			assertCode(t, err, apperror.CodeForbidden)
			_, err = s.Reject(ctx, uuid.New(), role, id, "Duplicate posting")
			assertCode(t, err, apperror.CodeForbidden)
			if repo.listCalls+repo.getCalls+repo.decideCalls != 0 {
				t.Fatal("a non-admin request reached the repository")
			}
		})
	}
}
