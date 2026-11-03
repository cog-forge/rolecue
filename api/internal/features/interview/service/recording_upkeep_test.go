package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/google/uuid"
)

type fakeUpkeepRepository struct {
	noneBefore, uploadingBefore time.Time
	changes                     []domain.RecordingChange
	decidedBefore               time.Time
	expired                     []domain.StoredRecording
	cleared                     map[uuid.UUID]string
	clearErr                    error
}

func (r *fakeUpkeepRepository) FailStaleRecordings(_ context.Context, noneBefore, uploadingBefore time.Time) ([]domain.RecordingChange, error) {
	r.noneBefore, r.uploadingBefore = noneBefore, uploadingBefore
	return r.changes, nil
}

func (r *fakeUpkeepRepository) ListExpiredRecordings(_ context.Context, decidedBefore time.Time, _ int32) ([]domain.StoredRecording, error) {
	r.decidedBefore = decidedBefore
	var left []domain.StoredRecording
	for _, recording := range r.expired {
		if _, ok := r.cleared[recording.InterviewID]; !ok {
			left = append(left, recording)
		}
	}
	return left, nil
}

func (r *fakeUpkeepRepository) ClearDeletedRecording(_ context.Context, id uuid.UUID, key string) (bool, error) {
	if r.clearErr != nil {
		return false, r.clearErr
	}
	if r.cleared == nil {
		r.cleared = map[uuid.UUID]string{}
	}
	r.cleared[id] = key
	return true, nil
}

// fakeDeleter fails the keys in failing once each, and reports the keys it deleted.
type fakeDeleter struct {
	deleted []string
	failing map[string]error
}

func (d *fakeDeleter) Delete(_ context.Context, key string) error {
	if err, ok := d.failing[key]; ok {
		delete(d.failing, key)
		return err
	}
	d.deleted = append(d.deleted, key)
	return nil
}

func TestFailStaleUsesTheGraceAndTheGrantLifetime(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	id := uuid.New()
	repo := &fakeUpkeepRepository{changes: []domain.RecordingChange{{InterviewID: id, From: domain.RecordNone, To: domain.RecordFailed}}}
	upkeep, err := NewRecordingUpkeep(repo, &fakeDeleter{}, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	ctx, logs := observedContext()
	failed, err := upkeep.FailStale(ctx, now)
	if err != nil || failed != 1 {
		t.Fatalf("failed=%d err=%v", failed, err)
	}
	if !repo.noneBefore.Equal(now.Add(-5*time.Minute)) || !repo.uploadingBefore.Equal(now.Add(-15*time.Minute)) {
		t.Fatalf("thresholds none=%v uploading=%v", repo.noneBefore, repo.uploadingBefore)
	}
	fields := onlyEntry(t, logs, "recording status changed").ContextMap()
	if fields["interview_id"] != id.String() || fields["from_status"] != "none" || fields["to_status"] != "failed" {
		t.Fatalf("log fields = %v", fields)
	}
}

func TestPurgeExpiredDeletesTheObjectThenClearsTheKey(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	first, second := uuid.New(), uuid.New()
	repo := &fakeUpkeepRepository{expired: []domain.StoredRecording{
		{InterviewID: first, Key: "recordings/secret-key-1"},
		{InterviewID: second, Key: "recordings/secret-key-2"},
	}}
	store := &fakeDeleter{}
	upkeep, err := NewRecordingUpkeep(repo, store, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	ctx, logs := observedContext()
	purged, err := upkeep.PurgeExpired(ctx, now)
	if err != nil || purged != 2 {
		t.Fatalf("purged=%d err=%v", purged, err)
	}
	if !repo.decidedBefore.Equal(now.Add(-90 * 24 * time.Hour)) {
		t.Fatalf("retention cutoff = %v", repo.decidedBefore)
	}
	if len(store.deleted) != 2 || repo.cleared[first] != "recordings/secret-key-1" {
		t.Fatalf("deleted %v cleared %v", store.deleted, repo.cleared)
	}
	if strings.Contains(loggedText(logs), "secret-key") {
		t.Fatal("an object key reached the log")
	}
	// A second run finds nothing left to purge.
	if purged, err := upkeep.PurgeExpired(ctx, now); purged != 0 || err != nil {
		t.Fatalf("second run purged=%d err=%v", purged, err)
	}
}

// A delete failure keeps the row for the next run, which deletes and clears it; the other recordings of
// the failing run are purged anyway.
func TestPurgeExpiredRetriesAFailedDeleteOnTheNextRun(t *testing.T) {
	failing, healthy := uuid.New(), uuid.New()
	repo := &fakeUpkeepRepository{expired: []domain.StoredRecording{
		{InterviewID: failing, Key: "recordings/a"},
		{InterviewID: healthy, Key: "recordings/b"},
	}}
	store := &fakeDeleter{failing: map[string]error{"recordings/a": errors.New("storage unavailable")}}
	upkeep, err := NewRecordingUpkeep(repo, store, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	purged, err := upkeep.PurgeExpired(context.Background(), time.Now())
	if err == nil || purged != 1 {
		t.Fatalf("first run purged=%d err=%v", purged, err)
	}
	if _, ok := repo.cleared[failing]; ok {
		t.Fatal("a recording whose object was not deleted must keep its key")
	}
	purged, err = upkeep.PurgeExpired(context.Background(), time.Now())
	if err != nil || purged != 1 || repo.cleared[failing] != "recordings/a" {
		t.Fatalf("retry purged=%d err=%v cleared=%v", purged, err, repo.cleared)
	}
}

// An object already gone (a crash after the delete) counts as deleted, so the row is cleared.
func TestPurgeExpiredTreatsAMissingObjectAsDeleted(t *testing.T) {
	id := uuid.New()
	repo := &fakeUpkeepRepository{expired: []domain.StoredRecording{{InterviewID: id, Key: "recordings/gone"}}}
	store := &fakeDeleter{failing: map[string]error{"recordings/gone": storage.ErrNotFound}}
	upkeep, err := NewRecordingUpkeep(repo, store, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if purged, err := upkeep.PurgeExpired(context.Background(), time.Now()); purged != 1 || err != nil {
		t.Fatalf("purged=%d err=%v", purged, err)
	}
}

func TestNewRecordingUpkeepNeedsItsDependencies(t *testing.T) {
	if _, err := NewRecordingUpkeep(nil, &fakeDeleter{}, time.Minute); err == nil {
		t.Fatal("a nil repository must be refused")
	}
	if _, err := NewRecordingUpkeep(&fakeUpkeepRepository{}, nil, time.Minute); err == nil {
		t.Fatal("a nil store must be refused")
	}
	if _, err := NewRecordingUpkeep(&fakeUpkeepRepository{}, &fakeDeleter{}, 0); err == nil {
		t.Fatal("a zero grace must be refused")
	}
}
