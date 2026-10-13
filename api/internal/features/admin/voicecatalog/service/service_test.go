package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/admin/voicecatalog/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// fakeRepository behaves like the SQL: Remove deactivates a referenced voice, deletes an unused
// one, and a deleted voice is gone afterwards.
type fakeRepository struct {
	voices    map[uuid.UUID]*domain.VoiceProfile
	listErr   error
	removeErr error

	listCalls, removeCalls int
	filter                 domain.VoiceListFilter
	afterRemove            func()
}

func newRepo(voices ...domain.VoiceProfile) *fakeRepository {
	f := &fakeRepository{voices: map[uuid.UUID]*domain.VoiceProfile{}}
	for i := range voices {
		v := voices[i]
		f.voices[v.ID] = &v
	}
	return f
}

func (f *fakeRepository) List(_ context.Context, filter domain.VoiceListFilter) ([]domain.VoiceProfile, int64, error) {
	f.listCalls++
	f.filter = filter
	if f.listErr != nil {
		return nil, 0, f.listErr
	}
	return []domain.VoiceProfile{{ID: uuid.New(), Name: "Ava"}}, 1, nil
}

func (f *fakeRepository) Get(_ context.Context, id uuid.UUID) (domain.VoiceProfile, error) {
	v, ok := f.voices[id]
	if !ok {
		return domain.VoiceProfile{}, domain.ErrVoiceNotFound
	}
	return *v, nil
}

func (f *fakeRepository) Remove(_ context.Context, id uuid.UUID) (domain.VoiceDeleteOutcome, error) {
	f.removeCalls++
	if f.removeErr != nil {
		return "", f.removeErr
	}
	v, ok := f.voices[id]
	if !ok {
		return "", domain.ErrVoiceNotFound
	}
	if f.afterRemove != nil {
		defer f.afterRemove()
	}
	if v.InUse {
		v.IsActive = false
		return domain.VoiceDeactivated, nil
	}
	delete(f.voices, id)
	return domain.VoiceDeleted, nil
}

func newService(t *testing.T, repo Repository, log *logger.Logger) *Service {
	t.Helper()
	svc, err := New(repo, log)
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func assertCode(t *testing.T, err error, code apperror.Code) {
	t.Helper()
	var got *apperror.AppError
	if !errors.As(err, &got) || got.Code != code {
		t.Fatalf("error = %v, want code %s", err, code)
	}
}

func usedVoice() domain.VoiceProfile {
	return domain.VoiceProfile{ID: uuid.New(), Name: "Ava", IsActive: true, InterviewCount: 2, JobPostingCount: 1, InUse: true}
}

func unusedVoice() domain.VoiceProfile {
	return domain.VoiceProfile{ID: uuid.New(), Name: "Ben", IsActive: true}
}

func TestNewRequiresRepository(t *testing.T) {
	if _, err := New(nil, nil); err == nil {
		t.Fatal("a missing repository must be rejected")
	}
}

func TestOnlyAdminsReachTheCatalog(t *testing.T) {
	voice := usedVoice()
	for _, role := range []string{"candidate", "recruiter", ""} {
		repo := newRepo(voice)
		svc := newService(t, repo, nil)
		_, _, err := svc.List(context.Background(), role, domain.VoiceListFilter{Limit: 20})
		assertCode(t, err, apperror.CodeForbidden)
		_, err = svc.Get(context.Background(), role, voice.ID)
		assertCode(t, err, apperror.CodeForbidden)
		_, err = svc.Remove(context.Background(), uuid.New(), role, voice.ID)
		assertCode(t, err, apperror.CodeForbidden)
		if repo.listCalls != 0 || repo.removeCalls != 0 || !repo.voices[voice.ID].IsActive {
			t.Fatalf("role %q reached the repository", role)
		}
	}
}

func TestListNormalizesAndForwardsTheFilter(t *testing.T) {
	repo := newRepo()
	items, total, err := newService(t, repo, nil).List(context.Background(), "admin",
		domain.VoiceListFilter{Status: domain.VoiceStatusInactive, Query: "  Ava  ", Limit: 50, Offset: 100})
	if err != nil || len(items) != 1 || total != 1 {
		t.Fatalf("items=%d total=%d err=%v", len(items), total, err)
	}
	want := domain.VoiceListFilter{Status: domain.VoiceStatusInactive, Query: "Ava", Limit: 50, Offset: 100}
	if repo.filter != want {
		t.Fatalf("filter = %+v, want %+v", repo.filter, want)
	}

	_, _, _ = newService(t, repo, nil).List(context.Background(), "admin", domain.VoiceListFilter{Limit: 20})
	if repo.filter.Status != domain.VoiceStatusAll {
		t.Fatalf("an empty status must mean all, got %q", repo.filter.Status)
	}
}

func TestListRejectsInvalidFilter(t *testing.T) {
	for _, tc := range []struct {
		name   string
		filter domain.VoiceListFilter
	}{
		{"unknown status", domain.VoiceListFilter{Status: "archived", Limit: 20}},
		{"zero limit", domain.VoiceListFilter{Limit: 0}},
		{"limit over 100", domain.VoiceListFilter{Limit: 101}},
		{"negative offset", domain.VoiceListFilter{Limit: 20, Offset: -1}},
		{"search too long", domain.VoiceListFilter{Limit: 20, Query: strings.Repeat("é", domain.MaxVoiceQueryRunes+1)}},
		{"invalid UTF-8", domain.VoiceListFilter{Limit: 20, Query: string([]byte{0xff})}},
		{"null character", domain.VoiceListFilter{Limit: 20, Query: "a\x00b"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newRepo()
			_, _, err := newService(t, repo, nil).List(context.Background(), "admin", tc.filter)
			assertCode(t, err, apperror.CodeValidation)
			if repo.listCalls != 0 {
				t.Fatal("an invalid filter reached the repository")
			}
		})
	}
}

func TestRepositoryErrorsPassThrough(t *testing.T) {
	repo := newRepo()
	repo.listErr = apperror.New(apperror.CodeInternal, "voice catalog persistence failed")
	_, _, err := newService(t, repo, nil).List(context.Background(), "admin", domain.VoiceListFilter{Limit: 20})
	assertCode(t, err, apperror.CodeInternal)

	_, err = newService(t, repo, nil).Get(context.Background(), "admin", uuid.New())
	assertCode(t, err, apperror.CodeVoiceNotFound)

	voice := unusedVoice()
	repo = newRepo(voice)
	repo.removeErr = apperror.New(apperror.CodeInternal, "voice catalog persistence failed")
	_, err = newService(t, repo, nil).Remove(context.Background(), uuid.New(), "admin", voice.ID)
	assertCode(t, err, apperror.CodeInternal)
}

func TestRemoveDeactivatesAVoiceInUse(t *testing.T) {
	voice := usedVoice()
	repo := newRepo(voice)
	core, logs := observer.New(zap.InfoLevel)
	admin := uuid.New()
	result, err := newService(t, repo, &logger.Logger{Logger: zap.New(core)}).Remove(context.Background(), admin, "admin", voice.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != domain.VoiceDeactivated || result.Voice.IsActive || result.Voice.ID != voice.ID || !result.Voice.InUse {
		t.Fatalf("result = %+v", result)
	}
	if _, kept := repo.voices[voice.ID]; !kept {
		t.Fatal("a referenced voice must never be deleted")
	}
	entries := logs.FilterMessage("voice profile removed").All()
	if len(entries) != 1 {
		t.Fatalf("got %d audit entries, want 1", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields["voice_id"] != voice.ID.String() || fields["admin_id"] != admin.String() || fields["outcome"] != "deactivated" {
		t.Fatalf("audit fields = %v", fields)
	}
}

func TestRemoveDeletesAnUnusedVoice(t *testing.T) {
	voice := unusedVoice()
	repo := newRepo(voice)
	result, err := newService(t, repo, nil).Remove(context.Background(), uuid.New(), "admin", voice.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != domain.VoiceDeleted || result.Voice.Name != "Ben" {
		t.Fatalf("result = %+v", result)
	}
	if _, kept := repo.voices[voice.ID]; kept {
		t.Fatal("an unused voice should be deleted")
	}
}

func TestRemoveMissingVoice(t *testing.T) {
	repo := newRepo()
	_, err := newService(t, repo, nil).Remove(context.Background(), uuid.New(), "admin", uuid.New())
	assertCode(t, err, apperror.CodeVoiceNotFound)
	if repo.removeCalls != 0 {
		t.Fatal("a missing voice must not reach Remove")
	}
}

func TestRemoveReturnsFreshCountsAfterDeactivating(t *testing.T) {
	voice := usedVoice()
	repo := newRepo(voice)
	repo.afterRemove = func() { repo.voices[voice.ID].InterviewCount = 5 }
	result, err := newService(t, repo, nil).Remove(context.Background(), uuid.New(), "admin", voice.ID)
	if err != nil || result.Voice.InterviewCount != 5 || result.Voice.IsActive {
		t.Fatalf("result = %+v, %v", result, err)
	}
}

func TestRemoveRepeated(t *testing.T) {
	used, unused := usedVoice(), unusedVoice()
	repo := newRepo(used, unused)
	svc := newService(t, repo, nil)
	for i := 0; i < 2; i++ {
		result, err := svc.Remove(context.Background(), uuid.New(), "admin", used.ID)
		if err != nil || result.Outcome != domain.VoiceDeactivated {
			t.Fatalf("call %d: %+v, %v", i+1, result, err)
		}
	}
	if _, err := svc.Remove(context.Background(), uuid.New(), "admin", unused.ID); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Remove(context.Background(), uuid.New(), "admin", unused.ID)
	assertCode(t, err, apperror.CodeVoiceNotFound)
}
