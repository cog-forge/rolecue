package service

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/admin/moderation/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// fakeRepository behaves like the SQL guards: Decide only changes a pending posting, and approval
// also needs at least one core question (posting.CoreQuestionCount).
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
	if status == domain.PostingStatusOpen && f.posting.CoreQuestionCount == 0 {
		return domain.ErrNotPending
	}
	f.decidedStatus, f.decidedReason = status, reason
	f.posting.Status, f.posting.RejectReason = status, reason
	return nil
}

// fakeTextStore serves objects by key; a key it doesn't hold is storage.ErrNotFound. PresignGet
// links any key.
type fakeTextStore struct {
	objects    map[string]string
	err        error
	presignErr error
	keys       []string
	presigned  []string
}

func (f *fakeTextStore) PresignGet(_ context.Context, key string, ttl time.Duration) (storage.SignedRequest, error) {
	f.presigned = append(f.presigned, key)
	if f.presignErr != nil {
		return storage.SignedRequest{}, f.presignErr
	}
	return storage.SignedRequest{URL: "https://storage.example.com/" + key + "?signed", Method: "GET", ExpiresAt: time.Unix(0, 0).Add(ttl)}, nil
}

func (f *fakeTextStore) Download(_ context.Context, key string) (storage.Object, error) {
	f.keys = append(f.keys, key)
	if f.err != nil {
		return storage.Object{}, f.err
	}
	text, ok := f.objects[key]
	if !ok {
		return storage.Object{}, storage.ErrNotFound
	}
	return storage.Object{Body: io.NopCloser(strings.NewReader(text)), Size: int64(len(text))}, nil
}

const (
	storedKey  = "job-descriptions/stored.txt"
	storedText = "Senior Go engineer. Builds and runs our payment APIs."
)

// storedTexts holds the JD text of every posting postingIn builds.
func storedTexts() *fakeTextStore {
	return &fakeTextStore{objects: map[string]string{storedKey: storedText}}
}

func newService(t *testing.T, repo Repository) *Service {
	t.Helper()
	s, err := New(repo, storedTexts(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// newObservedService records log entries so tests can assert the audit line.
func newObservedService(t *testing.T, repo Repository) (*Service, *observer.ObservedLogs) {
	t.Helper()
	core, logs := observer.New(zap.InfoLevel)
	s, err := New(repo, storedTexts(), &logger.Logger{Logger: zap.New(core)})
	if err != nil {
		t.Fatal(err)
	}
	return s, logs
}

// postingIn returns a posting with a non-empty question bank, so approval is allowed by default.
func postingIn(status domain.PostingStatus) *fakeRepository {
	return &fakeRepository{posting: &domain.ReviewPosting{ID: uuid.New(), Status: status, ContentPath: storedKey, CoreQuestionCount: 50}}
}

func assertCode(t *testing.T, err error, code apperror.Code) {
	t.Helper()
	var got *apperror.AppError
	if !errors.As(err, &got) || got.Code != code {
		t.Fatalf("error = %v, want code %s", err, code)
	}
}

func TestNewRequiresRepositoryAndTextStore(t *testing.T) {
	if _, err := New(nil, &fakeTextStore{}, nil); err == nil {
		t.Fatal("New accepted a nil repository")
	}
	if _, err := New(&fakeRepository{}, nil, nil); err == nil {
		t.Fatal("New accepted a nil text store")
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
