package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// --- KAN-296: recruiter access audit ---

func TestRecruiterRecordingReadWritesExactlyOneAuditRow(t *testing.T) {
	repo := &fakeRecordingRepository{owned: true, state: readyRecordingWithETag(strPtr(`"v1"`))}
	svc := newTestRecordingService(t, repo, &fakeRecordingStore{headInfo: webmInfo(`"v1"`)})
	recruiter, application := uuid.New(), uuid.New()

	if _, err := svc.Recording(context.Background(), recruiter, "recruiter", uuid.New(), application); err != nil {
		t.Fatalf("recording: %v", err)
	}
	if len(repo.access) != 1 {
		t.Fatalf("audit rows = %d, want 1", len(repo.access))
	}
	row := repo.access[0]
	if row.action != domain.AccessRecordingView || row.recruiter != recruiter || row.application != application ||
		row.interview != repo.state.InterviewID {
		t.Fatalf("audit row = %+v", row)
	}
}

func TestRecruiterTranscriptReadWritesExactlyOneAuditRow(t *testing.T) {
	state := recruitmentRecordingState(domain.RecordNone, nil)
	state.Completed = true
	repo := &fakeRecordingRepository{owned: true, state: state}
	svc := newTestRecordingService(t, repo, &fakeRecordingStore{})

	if _, err := svc.Transcript(context.Background(), uuid.New(), "recruiter", uuid.New(), uuid.New()); err != nil {
		t.Fatalf("transcript: %v", err)
	}
	if len(repo.access) != 1 || repo.access[0].action != domain.AccessTranscriptView {
		t.Fatalf("audit rows = %+v", repo.access)
	}
}

func TestFailedReadsWriteNoAuditRow(t *testing.T) {
	notCompleted := recruitmentRecordingState(domain.RecordNone, nil)
	cases := map[string]func(t *testing.T) (*fakeRecordingRepository, error){
		"foreign application": func(t *testing.T) (*fakeRecordingRepository, error) {
			repo := &fakeRecordingRepository{owned: false}
			_, err := newTestRecordingService(t, repo, &fakeRecordingStore{}).Recording(context.Background(), uuid.New(), "recruiter", uuid.New(), uuid.New())
			return repo, err
		},
		"wrong role": func(t *testing.T) (*fakeRecordingRepository, error) {
			repo := &fakeRecordingRepository{owned: true, state: readyRecordingWithETag(nil)}
			_, err := newTestRecordingService(t, repo, &fakeRecordingStore{}).Transcript(context.Background(), uuid.New(), "candidate", uuid.New(), uuid.New())
			return repo, err
		},
		"recording not ready has no link": func(t *testing.T) (*fakeRecordingRepository, error) {
			repo := &fakeRecordingRepository{owned: true, state: recruitmentRecordingState(domain.RecordUploading, recordingPath(testRecordingKey))}
			_, err := newTestRecordingService(t, repo, &fakeRecordingStore{}).Recording(context.Background(), uuid.New(), "recruiter", uuid.New(), uuid.New())
			if err != nil {
				t.Fatalf("a status read must succeed: %v", err)
			}
			return repo, nil
		},
		"changed object": func(t *testing.T) (*fakeRecordingRepository, error) {
			repo := &fakeRecordingRepository{owned: true, failChangedOK: true, state: readyRecordingWithETag(strPtr(`"v1"`))}
			_, err := newTestRecordingService(t, repo, &fakeRecordingStore{headInfo: webmInfo(`"v2"`)}).Recording(context.Background(), uuid.New(), "recruiter", uuid.New(), uuid.New())
			return repo, err
		},
		"storage unavailable": func(t *testing.T) (*fakeRecordingRepository, error) {
			repo := &fakeRecordingRepository{owned: true, state: readyRecordingWithETag(strPtr(`"v1"`))}
			_, err := newTestRecordingService(t, repo, &fakeRecordingStore{headErr: errors.New("down")}).Recording(context.Background(), uuid.New(), "recruiter", uuid.New(), uuid.New())
			return repo, err
		},
		"transcript of unfinished interview": func(t *testing.T) (*fakeRecordingRepository, error) {
			repo := &fakeRecordingRepository{owned: true, state: notCompleted}
			_, err := newTestRecordingService(t, repo, &fakeRecordingStore{}).Transcript(context.Background(), uuid.New(), "recruiter", uuid.New(), uuid.New())
			return repo, err
		},
	}
	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			repo, _ := run(t)
			if len(repo.access) != 0 {
				t.Fatalf("a read that issued nothing wrote %d audit rows", len(repo.access))
			}
		})
	}
}

// If the audit row cannot be written, nothing is returned: no read without an audit.
func TestReadIsRefusedWhenTheAuditRowCannotBeWritten(t *testing.T) {
	repo := &fakeRecordingRepository{owned: true, state: readyRecordingWithETag(strPtr(`"v1"`)), accessErr: errors.New("db down")}
	svc := newTestRecordingService(t, repo, &fakeRecordingStore{headInfo: webmInfo(`"v1"`)})
	recording, err := svc.Recording(context.Background(), uuid.New(), "recruiter", uuid.New(), uuid.New())
	if err == nil || recording.URL != nil {
		t.Fatalf("recording=%+v err=%v; want no link and an error", recording, err)
	}

	state := recruitmentRecordingState(domain.RecordNone, nil)
	state.Completed = true
	repo = &fakeRecordingRepository{owned: true, state: state, accessErr: errors.New("db down"), turns: []domain.TranscriptTurn{{Position: 1}}}
	transcript, err := newTestRecordingService(t, repo, &fakeRecordingStore{}).Transcript(context.Background(), uuid.New(), "recruiter", uuid.New(), uuid.New())
	if err == nil || len(transcript.Turns) != 0 {
		t.Fatalf("transcript=%+v err=%v; want nothing and an error", transcript, err)
	}
}

func TestAccessAuditIsAdminOnlyAndPaginated(t *testing.T) {
	for _, role := range []string{"candidate", "recruiter", ""} {
		repo := &fakeRecordingRepository{}
		_, err := newTestRecordingService(t, repo, &fakeRecordingStore{}).AccessAudit(context.Background(), role, domain.AccessAuditFilter{})
		assertCode(t, err, apperror.CodeForbidden)
	}

	repo := &fakeRecordingRepository{}
	svc := newTestRecordingService(t, repo, &fakeRecordingStore{})
	page, err := svc.AccessAudit(context.Background(), "admin", domain.AccessAuditFilter{})
	if err != nil || page.Page != 1 || page.PageSize != domain.DefaultAccessAuditPageSize {
		t.Fatalf("defaults: page=%+v err=%v", page, err)
	}
	for _, filter := range []domain.AccessAuditFilter{{Page: -1}, {PageSize: domain.MaxAccessAuditPageSize + 1}, {PageSize: -5}} {
		_, err := svc.AccessAudit(context.Background(), "admin", filter)
		assertCode(t, err, apperror.CodeValidation)
	}
	recruiter := uuid.New()
	if _, err := svc.AccessAudit(context.Background(), "admin", domain.AccessAuditFilter{RecruiterID: &recruiter, Page: 2, PageSize: 10}); err != nil {
		t.Fatal(err)
	}
	if repo.auditFilter.RecruiterID == nil || *repo.auditFilter.RecruiterID != recruiter || repo.auditFilter.Page != 2 {
		t.Fatalf("filter not passed through: %+v", repo.auditFilter)
	}
}

// --- KAN-297: recording consent ---

func TestGrantUploadStoresTheConsentVersionWithTheGrant(t *testing.T) {
	repo := &fakeRecordingRepository{uploadOK: true, state: recruitmentRecordingState(domain.RecordNone, nil)}
	svc := newTestRecordingService(t, repo, &fakeRecordingStore{grantKey: testRecordingKey})
	input := validUpload()
	input.ConsentVersion = "2026-11-v1"
	if _, err := svc.GrantUpload(context.Background(), uuid.New(), "candidate", uuid.New(), input); err != nil {
		t.Fatalf("grant: %v", err)
	}
	if repo.consentSent != "2026-11-v1" {
		t.Fatalf("stored consent = %q", repo.consentSent)
	}
}

func TestGrantUploadWithoutConsentIsRefusedOnlyWhenEnforced(t *testing.T) {
	newService := func(required bool, state domain.RecordingState) (*RecordingService, *fakeRecordingRepository, *fakeRecordingStore) {
		repo := &fakeRecordingRepository{uploadOK: true, state: state}
		store := &fakeRecordingStore{grantKey: testRecordingKey}
		svc := newTestRecordingService(t, repo, store)
		svc.SetConsentRequired(required)
		return svc, repo, store
	}

	// Not enforced (the default): a grant without consent works, as before.
	svc, _, _ := newService(false, recruitmentRecordingState(domain.RecordNone, nil))
	if _, err := svc.GrantUpload(context.Background(), uuid.New(), "candidate", uuid.New(), validUpload()); err != nil {
		t.Fatalf("unenforced grant: %v", err)
	}

	// Enforced: no consent sent and none stored means no grant, no storage call, no write.
	svc, repo, store := newService(true, recruitmentRecordingState(domain.RecordNone, nil))
	_, err := svc.GrantUpload(context.Background(), uuid.New(), "candidate", uuid.New(), validUpload())
	assertCode(t, err, apperror.CodeRecordingConsentRequired)
	if repo.uploadCalls != 0 || store.putCalls != 0 {
		t.Fatalf("a refused grant must touch nothing: writes=%d presigns=%d", repo.uploadCalls, store.putCalls)
	}

	// Enforced, consent sent: granted.
	input := validUpload()
	input.ConsentVersion = "2026-11-v1"
	if _, err := svc.GrantUpload(context.Background(), uuid.New(), "candidate", uuid.New(), input); err != nil {
		t.Fatalf("consented grant: %v", err)
	}

	// Enforced, consent stored by an earlier grant: a retry without it is granted.
	state := recruitmentRecordingState(domain.RecordFailed, nil)
	state.ConsentVersion = strPtr("2026-11-v1")
	svc, _, _ = newService(true, state)
	if _, err := svc.GrantUpload(context.Background(), uuid.New(), "candidate", uuid.New(), validUpload()); err != nil {
		t.Fatalf("retry with stored consent: %v", err)
	}
}

func TestGrantUploadRejectsAMalformedConsentVersion(t *testing.T) {
	svc := newTestRecordingService(t, &fakeRecordingRepository{}, &fakeRecordingStore{})
	for _, version := range []string{"has space", "semi;colon", strings.Repeat("v", domain.MaxConsentVersionLength+1)} {
		input := validUpload()
		input.ConsentVersion = version
		_, err := svc.GrantUpload(context.Background(), uuid.New(), "candidate", uuid.New(), input)
		assertCode(t, err, apperror.CodeValidation)
	}
}

// --- KAN-281: retention ---

type fakeCV struct {
	id        uuid.UUID
	key       string
	decidedAt time.Time
	cleared   bool
}

type fakeTranscript struct {
	completedAt time.Time
	turns       int // turns still holding an answer
}

// fakeRetentionRepository mimics the SQL: strictly older than the cutoff, only what still holds data,
// batch-limited, and a pointer is cleared only by ClearExpiredCV.
type fakeRetentionRepository struct {
	cvs         []*fakeCV
	transcripts []*fakeTranscript
	cvCutoff    time.Time
	trCutoff    time.Time
}

func (r *fakeRetentionRepository) ListExpiredCVs(_ context.Context, before time.Time, limit int32) ([]domain.StoredCV, error) {
	r.cvCutoff = before
	var out []domain.StoredCV
	for _, cv := range r.cvs {
		if !cv.cleared && cv.decidedAt.Before(before) && int32(len(out)) < limit {
			out = append(out, domain.StoredCV{ApplicationID: cv.id, Key: cv.key})
		}
	}
	return out, nil
}

func (r *fakeRetentionRepository) ClearExpiredCV(_ context.Context, id uuid.UUID, key string) (bool, error) {
	for _, cv := range r.cvs {
		if cv.id == id && cv.key == key && !cv.cleared {
			cv.cleared = true
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeRetentionRepository) ClearExpiredTranscripts(_ context.Context, before time.Time, limit int32) (int, error) {
	r.trCutoff = before
	cleared, interviews := 0, int32(0)
	for _, tr := range r.transcripts {
		if tr.turns > 0 && tr.completedAt.Before(before) && interviews < limit {
			cleared += tr.turns
			tr.turns = 0
			interviews++
		}
	}
	return cleared, nil
}

func newRetentionUpkeep(t *testing.T, repo *fakeRetentionRepository, store *fakeDeleter) *RecordingUpkeep {
	t.Helper()
	upkeep, err := NewRecordingUpkeep(&fakeUpkeepRepository{}, store, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return upkeep.WithRetention(repo, RetentionPolicy{CVDays: 180, TranscriptDays: 365})
}

func TestRetentionIsInertWithoutPolicy(t *testing.T) {
	upkeep, err := NewRecordingUpkeep(&fakeUpkeepRepository{}, &fakeDeleter{}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if n, err := upkeep.PurgeExpiredCVs(context.Background(), now); n != 0 || err != nil {
		t.Fatalf("cv: %d %v", n, err)
	}
	if n, err := upkeep.ClearExpiredTranscripts(context.Background(), now); n != 0 || err != nil {
		t.Fatalf("transcripts: %d %v", n, err)
	}
}

func TestCVRetentionBoundary(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	cutoff := now.Add(-180 * 24 * time.Hour)
	before := &fakeCV{id: uuid.New(), key: "cv/before", decidedAt: cutoff.Add(-time.Second)}
	exactly := &fakeCV{id: uuid.New(), key: "cv/exactly", decidedAt: cutoff}
	after := &fakeCV{id: uuid.New(), key: "cv/after", decidedAt: cutoff.Add(time.Second)}
	repo := &fakeRetentionRepository{cvs: []*fakeCV{before, exactly, after}}
	store := &fakeDeleter{}
	upkeep := newRetentionUpkeep(t, repo, store)

	purged, err := upkeep.PurgeExpiredCVs(context.Background(), now)
	if err != nil || purged != 1 {
		t.Fatalf("purged=%d err=%v", purged, err)
	}
	if !repo.cvCutoff.Equal(cutoff) {
		t.Fatalf("cutoff = %v, want %v", repo.cvCutoff, cutoff)
	}
	if !before.cleared || exactly.cleared || after.cleared {
		t.Fatalf("cleared before=%v exactly=%v after=%v", before.cleared, exactly.cleared, after.cleared)
	}
	if len(store.deleted) != 1 || store.deleted[0] != "cv/before" {
		t.Fatalf("deleted %v", store.deleted)
	}
	// One second later the boundary CV is old enough; nothing earlier is touched twice (idempotent).
	purged, err = upkeep.PurgeExpiredCVs(context.Background(), now.Add(time.Second))
	if err != nil || purged != 1 || !exactly.cleared || after.cleared {
		t.Fatalf("second run purged=%d err=%v exactly=%v after=%v", purged, err, exactly.cleared, after.cleared)
	}
	if purged, err := upkeep.PurgeExpiredCVs(context.Background(), now.Add(time.Second)); purged != 0 || err != nil {
		t.Fatalf("third run purged=%d err=%v", purged, err)
	}
}

// A storage failure keeps the pointer; the next run deletes the object and clears it.
func TestCVRetentionRetriesAfterAStorageFailure(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	old := now.Add(-181 * 24 * time.Hour)
	failing := &fakeCV{id: uuid.New(), key: "cv/secret-failing", decidedAt: old}
	fine := &fakeCV{id: uuid.New(), key: "cv/fine", decidedAt: old}
	repo := &fakeRetentionRepository{cvs: []*fakeCV{failing, fine}}
	store := &fakeDeleter{failing: map[string]error{"cv/secret-failing": errors.New("storage down")}}
	upkeep := newRetentionUpkeep(t, repo, store)
	ctx, logs := observedContext()

	purged, err := upkeep.PurgeExpiredCVs(ctx, now)
	if err == nil || purged != 1 {
		t.Fatalf("purged=%d err=%v; want 1 and an error", purged, err)
	}
	if failing.cleared || !fine.cleared {
		t.Fatalf("the failed CV must keep its pointer: failing=%v fine=%v", failing.cleared, fine.cleared)
	}
	if text := loggedText(logs); strings.Contains(text, "secret-failing") || strings.Contains(text, failing.id.String()) {
		t.Fatalf("a key or id reached the log: %s", text)
	}
	purged, err = upkeep.PurgeExpiredCVs(ctx, now)
	if err != nil || purged != 1 || !failing.cleared {
		t.Fatalf("retry purged=%d err=%v cleared=%v", purged, err, failing.cleared)
	}
}

func TestCVRetentionTreatsAMissingObjectAsDeleted(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	cv := &fakeCV{id: uuid.New(), key: "cv/gone", decidedAt: now.Add(-200 * 24 * time.Hour)}
	repo := &fakeRetentionRepository{cvs: []*fakeCV{cv}}
	store := &fakeDeleter{failing: map[string]error{"cv/gone": storage.ErrNotFound}}
	if purged, err := newRetentionUpkeep(t, repo, store).PurgeExpiredCVs(context.Background(), now); err != nil || purged != 1 || !cv.cleared {
		t.Fatalf("purged=%d err=%v cleared=%v", purged, err, cv.cleared)
	}
}

func TestCVRetentionIsBatchLimited(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	repo := &fakeRetentionRepository{}
	for i := 0; i < purgeBatch+7; i++ {
		repo.cvs = append(repo.cvs, &fakeCV{id: uuid.New(), key: "cv/" + uuid.NewString(), decidedAt: now.Add(-400 * 24 * time.Hour)})
	}
	upkeep := newRetentionUpkeep(t, repo, &fakeDeleter{})
	if purged, _ := upkeep.PurgeExpiredCVs(context.Background(), now); purged != purgeBatch {
		t.Fatalf("first run purged %d, want %d", purged, purgeBatch)
	}
	if purged, _ := upkeep.PurgeExpiredCVs(context.Background(), now); purged != 7 {
		t.Fatalf("second run purged %d, want 7", purged)
	}
}

func TestTranscriptRetentionBoundaryAndIdempotency(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	cutoff := now.Add(-365 * 24 * time.Hour)
	before := &fakeTranscript{completedAt: cutoff.Add(-time.Second), turns: 4}
	exactly := &fakeTranscript{completedAt: cutoff, turns: 3}
	after := &fakeTranscript{completedAt: cutoff.Add(time.Second), turns: 2}
	repo := &fakeRetentionRepository{transcripts: []*fakeTranscript{before, exactly, after}}
	upkeep := newRetentionUpkeep(t, repo, &fakeDeleter{})

	cleared, err := upkeep.ClearExpiredTranscripts(context.Background(), now)
	if err != nil || cleared != 4 {
		t.Fatalf("cleared=%d err=%v", cleared, err)
	}
	if !repo.trCutoff.Equal(cutoff) || before.turns != 0 || exactly.turns != 3 || after.turns != 2 {
		t.Fatalf("cutoff=%v before=%d exactly=%d after=%d", repo.trCutoff, before.turns, exactly.turns, after.turns)
	}
	if cleared, err := upkeep.ClearExpiredTranscripts(context.Background(), now); cleared != 0 || err != nil {
		t.Fatalf("repeat run cleared=%d err=%v", cleared, err)
	}
	if cleared, _ := upkeep.ClearExpiredTranscripts(context.Background(), now.Add(time.Second)); cleared != 3 {
		t.Fatalf("one second later cleared=%d, want 3", cleared)
	}
}
