package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/account/domain"
	"github.com/cog-forge/rolecue/api/internal/features/account/repository"
	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

type fakeRepo struct {
	work      domain.ErasureBlockers
	keys      []string
	eraseErr  error
	erased    bool
	calls     []string
	profile   domain.ExportProfile
	postings  []domain.ExportPosting
	apps      []domain.ExportApplication
	interview []domain.ExportInterview
}

func (f *fakeRepo) Account(context.Context, uuid.UUID) (domain.Identity, error) {
	return domain.Identity{}, nil
}
func (f *fakeRepo) ActiveWork(context.Context, uuid.UUID) (domain.ErasureBlockers, error) {
	f.calls = append(f.calls, "work")
	return f.work, nil
}
func (f *fakeRepo) StorageKeys(context.Context, uuid.UUID) ([]string, error) {
	f.calls = append(f.calls, "keys")
	return f.keys, nil
}
func (f *fakeRepo) Erase(context.Context, uuid.UUID) error {
	f.calls = append(f.calls, "erase")
	if f.eraseErr == nil {
		f.erased = true
	}
	return f.eraseErr
}
func (f *fakeRepo) ExportProfile(context.Context, uuid.UUID) (domain.ExportProfile, error) {
	return f.profile, nil
}
func (f *fakeRepo) ExportRoleProfiles(context.Context, uuid.UUID) ([]domain.ExportRoleProfile, error) {
	return nil, nil
}
func (f *fakeRepo) ExportApplications(context.Context, uuid.UUID) ([]domain.ExportApplication, error) {
	return f.apps, nil
}
func (f *fakeRepo) ExportInterviews(context.Context, uuid.UUID) ([]domain.ExportInterview, error) {
	return f.interview, nil
}
func (f *fakeRepo) ExportPostings(context.Context, uuid.UUID) ([]domain.ExportPosting, error) {
	return f.postings, nil
}

type fakeStore struct {
	deleted   []string
	failOn    string
	missing   map[string]bool
	signedTTL []time.Duration
	signedKey []string
}

func (s *fakeStore) Delete(_ context.Context, key string) error {
	if key == s.failOn {
		return errors.New("storage down")
	}
	if s.missing[key] {
		return fmt.Errorf("delete: %w", storage.ErrNotFound)
	}
	s.deleted = append(s.deleted, key)
	return nil
}

func (s *fakeStore) PresignGetWithOverrides(_ context.Context, key string, ttl time.Duration, _ storage.GetOverrides) (storage.SignedRequest, error) {
	s.signedKey = append(s.signedKey, key)
	s.signedTTL = append(s.signedTTL, ttl)
	return storage.SignedRequest{URL: "https://store.test/" + key, ExpiresAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(ttl)}, nil
}

func newService(r *fakeRepo, s *fakeStore) *Service {
	return &Service{repository: r, store: s, now: func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }}
}

func user(role string) authdomain.User {
	return authdomain.User{ID: uuid.New(), Email: "Person@Example.test", Role: role}
}

func code(t *testing.T, err error, want apperror.Code) {
	t.Helper()
	if !apperror.IsCode(err, want) {
		t.Fatalf("error = %v, want code %s", err, want)
	}
}

func TestEraseRefusals(t *testing.T) {
	repo, store := &fakeRepo{keys: []string{"k1"}}, &fakeStore{}
	svc := newService(repo, store)
	code(t, svc.Erase(context.Background(), user("admin"), "DELETE"), apperror.CodeAdminCannotSelfDelete)
	code(t, svc.Erase(context.Background(), user("candidate"), "delete me"), apperror.CodeValidation)
	code(t, svc.Erase(context.Background(), user("candidate"), ""), apperror.CodeValidation)
	if len(repo.calls) != 0 || len(store.deleted) != 0 {
		t.Fatalf("refusals touched data: %v %v", repo.calls, store.deleted)
	}
}

func TestEraseRefusesActiveRecruiterWithCountsOnly(t *testing.T) {
	repo := &fakeRepo{work: domain.ErasureBlockers{OpenPostings: 2, ApplicationsInProgress: 5}, keys: []string{"k1"}}
	store := &fakeStore{}
	err := newService(repo, store).Erase(context.Background(), user("recruiter"), "DELETE")
	code(t, err, apperror.CodeAccountHasActiveWork)
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Message != "close your work before deleting the account: 2 open job postings, 5 applications in progress" {
		t.Fatalf("message = %v", err)
	}
	if len(store.deleted) != 0 || repo.erased {
		t.Fatal("an active recruiter lost data")
	}
}

func TestEraseDeletesObjectsBeforeRows(t *testing.T) {
	repo := &fakeRepo{keys: []string{"cv", "https://example.test/jd", "rec"}}
	store := &fakeStore{missing: map[string]bool{"rec": true}}
	u := user("candidate")
	if err := newService(repo, store).Erase(context.Background(), u, " person@example.TEST "); err != nil {
		t.Fatal(err)
	}
	if len(store.deleted) != 1 || store.deleted[0] != "cv" {
		t.Fatalf("deleted = %v (a web URL is skipped, a missing object counts as deleted)", store.deleted)
	}
	if !repo.erased || repo.calls[len(repo.calls)-1] != "erase" {
		t.Fatalf("rows must be deleted last: %v", repo.calls)
	}
}

func TestEraseStorageFailureKeepsTheAccountAndCanBeRetried(t *testing.T) {
	repo := &fakeRepo{keys: []string{"a", "b"}}
	store := &fakeStore{failOn: "b"}
	svc := newService(repo, store)
	u := user("candidate")
	code(t, svc.Erase(context.Background(), u, "DELETE"), apperror.CodeInternal)
	if repo.erased {
		t.Fatal("rows were deleted although storage failed")
	}
	store.failOn = ""
	store.missing = map[string]bool{"a": true} // already removed by the first attempt
	if err := svc.Erase(context.Background(), u, "DELETE"); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if !repo.erased {
		t.Fatal("retry did not erase")
	}
}

func TestEraseRaceWithNewActiveWork(t *testing.T) {
	repo := &fakeRepo{eraseErr: repository.ErrActiveWork}
	err := newService(repo, &fakeStore{}).Erase(context.Background(), user("recruiter"), "DELETE")
	// The recheck reports the counts again; here the fake reports none, so the refusal carries zeros.
	code(t, err, apperror.CodeAccountHasActiveWork)
}

func TestExportRecruiterScopeIsProfileAndPostings(t *testing.T) {
	repo := &fakeRepo{postings: []domain.ExportPosting{{ID: uuid.New(), Title: "Go dev"}}, apps: []domain.ExportApplication{{ID: uuid.New()}}}
	out, err := newService(repo, &fakeStore{}).Export(context.Background(), user("recruiter"))
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Postings) != 1 || len(out.Applications) != 0 || len(out.Interviews) != 0 {
		t.Fatalf("recruiter export = %+v", out)
	}
}

func TestExportCandidateLinksUseFifteenMinuteTTL(t *testing.T) {
	cv, rec := "cv-key", "rec-key"
	web := "https://example.test/cv.pdf"
	repo := &fakeRepo{
		apps:      []domain.ExportApplication{{ID: uuid.New(), CVKey: &cv}, {ID: uuid.New(), CVKey: &web}, {ID: uuid.New()}},
		interview: []domain.ExportInterview{{ID: uuid.New(), Type: "practice", RecordKey: &rec}, {ID: uuid.New(), Type: "practice"}},
	}
	store := &fakeStore{}
	out, err := newService(repo, store).Export(context.Background(), user("candidate"))
	if err != nil {
		t.Fatal(err)
	}
	if domain.ExportLinkTTL != 15*time.Minute || out.LinkTTLSeconds != 900 {
		t.Fatalf("ttl = %v / %d", domain.ExportLinkTTL, out.LinkTTLSeconds)
	}
	if len(store.signedKey) != 2 {
		t.Fatalf("signed %v, want the CV and the recording only", store.signedKey)
	}
	for _, ttl := range store.signedTTL {
		if ttl != 15*time.Minute {
			t.Fatalf("signed with ttl %v", ttl)
		}
	}
	if out.Applications[0].CV == nil || out.Applications[1].CV != nil || out.Applications[2].CV != nil {
		t.Fatalf("cv links = %+v", out.Applications)
	}
	if !out.Applications[0].CV.ExpiresAt.Equal(out.GeneratedAt.Add(15 * time.Minute)) {
		t.Fatalf("expires_at = %v", out.Applications[0].CV.ExpiresAt)
	}
	if out.Interviews[0].Recording == nil || out.Interviews[1].Recording != nil {
		t.Fatalf("recording links = %+v", out.Interviews)
	}
}

func TestExportRefusesAdminAndAnonymous(t *testing.T) {
	svc := newService(&fakeRepo{}, &fakeStore{})
	_, err := svc.Export(context.Background(), user("admin"))
	code(t, err, apperror.CodeForbidden)
	_, err = svc.Export(context.Background(), authdomain.User{Role: "candidate"})
	code(t, err, apperror.CodeInvalidToken)
}
