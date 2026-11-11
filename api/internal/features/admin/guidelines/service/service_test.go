package service

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/admin/guidelines/domain"
	interviewdomain "github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

type fakeRepository struct {
	current    domain.Stored
	found      bool
	saveCalls  int
	readCalls  int
	savedBy    uuid.UUID
	savedAfter int32
	saved      interviewdomain.BehaviorGuideline
	err        error
}

func (r *fakeRepository) Current(context.Context) (domain.Stored, bool, error) {
	r.readCalls++
	return r.current, r.found, r.err
}

func (r *fakeRepository) Save(_ context.Context, adminID uuid.UUID, expected int32, values interviewdomain.BehaviorGuideline) (domain.Stored, error) {
	r.saveCalls++
	r.savedBy, r.savedAfter, r.saved = adminID, expected, values
	if r.err != nil {
		return domain.Stored{}, r.err
	}
	if expected != r.current.Version {
		return domain.Stored{}, apperror.New(apperror.CodeGuidelineVersionConflict, "stale")
	}
	r.current = domain.Stored{Version: expected + 1, Values: values, UpdatedBy: adminID, UpdatedAt: time.Unix(1700000000, 0).UTC(), UpdatedByName: "Ada Admin"}
	r.found = true
	return r.current, nil
}

func newService(t *testing.T, repo *fakeRepository) *Service {
	t.Helper()
	s, err := New(repo, 2)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func formal() interviewdomain.BehaviorGuideline {
	g := interviewdomain.DefaultBehaviorGuideline()
	g.Tone = interviewdomain.ToneFormal
	g.AvoidTopics = []string{interviewdomain.TopicLocationNational, interviewdomain.TopicCompensation}
	return g
}

func wantCode(t *testing.T, err error, code apperror.Code) {
	t.Helper()
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Code != code {
		t.Fatalf("err = %v, want %s", err, code)
	}
}

func TestOnlyAdministratorsReadOrSave(t *testing.T) {
	for _, role := range []string{"candidate", "recruiter", "guest", ""} {
		repo := &fakeRepository{}
		s := newService(t, repo)
		_, err := s.Get(context.Background(), role)
		wantCode(t, err, apperror.CodeForbidden)
		_, err = s.Save(context.Background(), uuid.New(), role, 0, formal())
		wantCode(t, err, apperror.CodeForbidden)
		if repo.readCalls != 0 || repo.saveCalls != 0 {
			t.Fatalf("role %q reached the repository: %+v", role, repo)
		}
	}
}

func TestGetWithoutASavedVersionReturnsTheDefaults(t *testing.T) {
	settings, err := newService(t, &fakeRepository{}).Get(context.Background(), "admin")
	if err != nil {
		t.Fatal(err)
	}
	if settings.Version != 0 || settings.UpdatedAt != nil || !settings.Values.IsDefault() || !reflect.DeepEqual(settings.Values, settings.Defaults) {
		t.Fatalf("settings = %+v, want version 0 and the defaults", settings)
	}
	if settings.EffectiveFollowUpCeiling != 2 || !reflect.DeepEqual(settings.Allowed, domain.AllowedValues()) {
		t.Fatalf("ceiling/allowed = %d / %+v", settings.EffectiveFollowUpCeiling, settings.Allowed)
	}
}

func TestSaveThenGetRoundTripsAndCountsVersions(t *testing.T) {
	repo := &fakeRepository{}
	s := newService(t, repo)
	admin := uuid.New()
	saved, err := s.Save(context.Background(), admin, "admin", 0, formal())
	if err != nil {
		t.Fatal(err)
	}
	if saved.Version != 1 || saved.UpdatedByName != "Ada Admin" || saved.UpdatedAt == nil {
		t.Fatalf("saved = %+v", saved)
	}
	if repo.savedBy != admin || repo.savedAfter != 0 {
		t.Fatalf("saved by %s after %d", repo.savedBy, repo.savedAfter)
	}
	// Topics reach the repository in catalog order whatever order the Admin sent.
	if want := []string{interviewdomain.TopicCompensation, interviewdomain.TopicLocationNational}; !reflect.DeepEqual(repo.saved.AvoidTopics, want) {
		t.Fatalf("stored topics = %v, want %v", repo.saved.AvoidTopics, want)
	}
	loaded, err := s.Get(context.Background(), "admin")
	if err != nil || !reflect.DeepEqual(loaded, saved) {
		t.Fatalf("reload = %+v, %v; want %+v", loaded, err, saved)
	}
	second, err := s.Save(context.Background(), admin, "admin", 1, interviewdomain.DefaultBehaviorGuideline())
	if err != nil || second.Version != 2 || !second.Values.IsDefault() {
		t.Fatalf("reset = %+v, %v; want version 2 with the defaults", second, err)
	}
}

func TestStaleSaveIsRefusedAndWritesNothing(t *testing.T) {
	repo := &fakeRepository{current: domain.Stored{Version: 3, Values: interviewdomain.DefaultBehaviorGuideline()}, found: true}
	_, err := newService(t, repo).Save(context.Background(), uuid.New(), "admin", 2, formal())
	wantCode(t, err, apperror.CodeGuidelineVersionConflict)
	if repo.current.Version != 3 {
		t.Fatalf("version = %d, a stale save must not write", repo.current.Version)
	}
}

func TestInvalidInputNeverReachesTheRepository(t *testing.T) {
	bad := formal()
	bad.Tone = "sarcastic"
	repo := &fakeRepository{}
	s := newService(t, repo)
	_, err := s.Save(context.Background(), uuid.New(), "admin", 0, bad)
	wantCode(t, err, apperror.CodeValidation)
	_, err = s.Save(context.Background(), uuid.New(), "admin", -1, formal())
	wantCode(t, err, apperror.CodeValidation)
	if repo.saveCalls != 0 || repo.readCalls != 0 {
		t.Fatalf("invalid input reached the repository: %+v", repo)
	}
}

func TestRepositoryFailuresPassThrough(t *testing.T) {
	repo := &fakeRepository{err: errors.New("database unavailable")}
	s := newService(t, repo)
	if _, err := s.Get(context.Background(), "admin"); err == nil {
		t.Fatal("a read failure must surface")
	}
	if _, err := s.Save(context.Background(), uuid.New(), "admin", 0, formal()); err == nil {
		t.Fatal("a save failure must surface")
	}
}

func TestNewRejectsBadSetup(t *testing.T) {
	if _, err := New(nil, 1); err == nil {
		t.Fatal("a repository is required")
	}
	if _, err := New(&fakeRepository{}, -1); err == nil {
		t.Fatal("a negative ceiling is refused")
	}
}
