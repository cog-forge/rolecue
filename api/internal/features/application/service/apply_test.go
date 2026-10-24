package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// applyRepository adds the Candidate side to the screening fake. It only records what the service
// passes in and returns the configured answers.
type applyRepository struct {
	*fakeRepository

	submission   domain.Submission
	insertErr    error
	insertCalls  int
	insertedKey  string
	insertedUser uuid.UUID
	insertedPost uuid.UUID

	items      []domain.CandidateApplication
	total      int64
	listLimit  int32
	listOffset int32
	listErr    error

	application domain.CandidateApplication
	getErr      error
}

func newApplyRepository() *applyRepository {
	return &applyRepository{fakeRepository: &fakeRepository{}}
}

func (r *applyRepository) InsertCandidateApplication(_ context.Context, candidateID, postingID uuid.UUID, cvKey string) (domain.Submission, error) {
	r.insertCalls++
	r.insertedUser, r.insertedPost, r.insertedKey = candidateID, postingID, cvKey
	if r.insertErr != nil {
		return domain.Submission{}, r.insertErr
	}
	return r.submission, nil
}

func (r *applyRepository) ListCandidateApplications(_ context.Context, _ uuid.UUID, limit, offset int32) ([]domain.CandidateApplication, int64, error) {
	r.listLimit, r.listOffset = limit, offset
	return r.items, r.total, r.listErr
}

func (r *applyRepository) GetCandidateApplication(_ context.Context, _, _ uuid.UUID) (domain.CandidateApplication, error) {
	return r.application, r.getErr
}

// applyCVStore keeps uploaded CVs in memory and records deletes.
type applyCVStore struct {
	objects   map[string][]byte
	uploads   int
	uploadErr error
	deleted   []string
}

func newApplyCVStore() *applyCVStore {
	return &applyCVStore{objects: map[string][]byte{}}
}

func (s *applyCVStore) Upload(_ context.Context, body io.ReadSeeker, contentType string) (string, error) {
	s.uploads++
	if s.uploadErr != nil {
		return "", s.uploadErr
	}
	if contentType != cvContentType {
		return "", errors.New("unexpected content type " + contentType)
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	key := uuid.NewString()
	s.objects[key] = data
	return key, nil
}

func (s *applyCVStore) Delete(_ context.Context, key string) error {
	s.deleted = append(s.deleted, key)
	delete(s.objects, key)
	return nil
}

func (s *applyCVStore) PresignGet(_ context.Context, key string, _ time.Duration) (storage.SignedRequest, error) {
	return storage.SignedRequest{URL: "https://cv.example.test/" + key, Method: "GET"}, nil
}

func applyService(t *testing.T, repo Repository, store CVLinker) *Service {
	t.Helper()
	svc, err := New(repo, store)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return svc
}

func validCV() []byte {
	return append([]byte("%PDF-1.7\n"), bytes.Repeat([]byte("x"), 64)...)
}

func TestApplyStoresCVAndInsertsPendingApplication(t *testing.T) {
	repo := newApplyRepository()
	repo.submission = domain.Submission{ID: uuid.New(), JobPostingID: postingID, Status: domain.StatusPending}
	store := newApplyCVStore()
	svc := applyService(t, repo, store)

	got, err := svc.Apply(context.Background(), candidateID, candidateRole, postingID, validCV())
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got != repo.submission {
		t.Fatalf("Apply() = %+v, want %+v", got, repo.submission)
	}
	if repo.insertCalls != 1 || repo.insertedUser != candidateID || repo.insertedPost != postingID {
		t.Fatalf("insert calls=%d candidate=%s posting=%s", repo.insertCalls, repo.insertedUser, repo.insertedPost)
	}
	stored, ok := store.objects[repo.insertedKey]
	if !ok || !bytes.Equal(stored, validCV()) {
		t.Fatal("the inserted key does not point at the uploaded CV")
	}
	if len(store.deleted) != 0 {
		t.Fatalf("a stored CV was deleted: %v", store.deleted)
	}
}

func TestApplyRejectsInvalidCVWithoutStoringAnything(t *testing.T) {
	oversize := append([]byte("%PDF-1.7\n"), make([]byte, domain.MaxCVBytes)...)
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"not a PDF", []byte("hello, this is plain text")},
		{"empty", []byte{}},
		{"nil", nil},
		{"over 5 MB", oversize},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newApplyRepository()
			store := newApplyCVStore()
			svc := applyService(t, repo, store)
			_, err := svc.Apply(context.Background(), candidateID, candidateRole, postingID, tc.data)
			if !apperror.IsCode(err, apperror.CodeValidation) {
				t.Fatalf("Apply() error = %v, want VALIDATION_ERROR", err)
			}
			if store.uploads != 0 || repo.insertCalls != 0 {
				t.Fatalf("uploads=%d inserts=%d, want none", store.uploads, repo.insertCalls)
			}
		})
	}
}

func TestApplyIsForbiddenForNonCandidates(t *testing.T) {
	for _, role := range []string{"recruiter", "admin", ""} {
		repo := newApplyRepository()
		store := newApplyCVStore()
		svc := applyService(t, repo, store)
		_, err := svc.Apply(context.Background(), candidateID, role, postingID, validCV())
		if !apperror.IsCode(err, apperror.CodeForbidden) {
			t.Fatalf("role %q: Apply() error = %v, want FORBIDDEN", role, err)
		}
		if store.uploads != 0 || repo.insertCalls != 0 {
			t.Fatalf("role %q: a non-candidate reached storage", role)
		}
	}
}

func TestApplyDeletesUploadWhenInsertFails(t *testing.T) {
	for _, insertErr := range []error{
		domain.ErrAlreadyApplied,
		domain.ErrIntakeNotOpen,
		domain.ErrJobPostingNotFound,
		errors.New("database unavailable"),
	} {
		repo := newApplyRepository()
		repo.insertErr = insertErr
		store := newApplyCVStore()
		svc := applyService(t, repo, store)

		_, err := svc.Apply(context.Background(), candidateID, candidateRole, postingID, validCV())
		if !errors.Is(err, insertErr) {
			t.Fatalf("Apply() error = %v, want %v", err, insertErr)
		}
		if len(store.deleted) != 1 || store.deleted[0] != repo.insertedKey || len(store.objects) != 0 {
			t.Fatalf("insert error %v: deleted=%v remaining=%d", insertErr, store.deleted, len(store.objects))
		}
	}
}

func TestApplyMapsPostingAnswers(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		code apperror.Code
	}{
		{"already applied", domain.ErrAlreadyApplied, "ALREADY_APPLIED"},
		{"intake closed", domain.ErrIntakeNotOpen, apperror.CodeIntakeNotOpen},
		{"posting not visible", domain.ErrJobPostingNotFound, apperror.CodeJobPostingNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newApplyRepository()
			repo.insertErr = tc.err
			svc := applyService(t, repo, newApplyCVStore())
			_, err := svc.Apply(context.Background(), candidateID, candidateRole, postingID, validCV())
			if !apperror.IsCode(err, tc.code) {
				t.Fatalf("Apply() error = %v, want %s", err, tc.code)
			}
		})
	}
}

func TestApplyStopsWhenUploadFails(t *testing.T) {
	repo := newApplyRepository()
	store := newApplyCVStore()
	store.uploadErr = errors.New("object storage unavailable")
	svc := applyService(t, repo, store)

	_, err := svc.Apply(context.Background(), candidateID, candidateRole, postingID, validCV())
	if !apperror.IsCode(err, apperror.CodeInternal) {
		t.Fatalf("Apply() error = %v, want INTERNAL_ERROR", err)
	}
	if repo.insertCalls != 0 {
		t.Fatal("an Application was inserted without a stored CV")
	}
}

func TestApplyNeedsCandidatePersistenceAndCVStorage(t *testing.T) {
	// The screening fake alone cannot apply, and a linker without upload cannot store a CV.
	svc := applyService(t, &fakeRepository{}, &fakeCVLinker{})
	if _, err := svc.Apply(context.Background(), candidateID, candidateRole, postingID, validCV()); !apperror.IsCode(err, apperror.CodeInternal) {
		t.Fatalf("Apply() without candidate persistence error = %v, want INTERNAL_ERROR", err)
	}
	svc = applyService(t, newApplyRepository(), &fakeCVLinker{})
	if _, err := svc.Apply(context.Background(), candidateID, candidateRole, postingID, validCV()); !apperror.IsCode(err, apperror.CodeInternal) {
		t.Fatalf("Apply() without CV storage error = %v, want INTERNAL_ERROR", err)
	}
}
