package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/google/uuid"
)

type recordingNotifier struct{ ended []uuid.UUID }

func (n *recordingNotifier) SessionEnded(id uuid.UUID) { n.ended = append(n.ended, id) }

// A Candidate's end goes through the one end path with the Candidate cause; the live socket is told
// once, by the call that ended the session, and never by a repeat (double click, second tab).
func TestCompleteEndsThroughTheSharedPathAndNotifiesOnce(t *testing.T) {
	repo := &fakeRepository{completeSession: practiceSession(), completeEnded: true}
	notifier := &recordingNotifier{}
	svc := newTestService(t, repo, Settings{})
	svc.SetEndNotifier(notifier)

	if _, err := svc.Complete(context.Background(), candidateID, "candidate", sessionID); err != nil {
		t.Fatal(err)
	}
	repo.completeEnded = false
	if _, err := svc.Complete(context.Background(), candidateID, "candidate", sessionID); err != nil {
		t.Fatal(err)
	}
	if len(repo.endParams) != 2 || repo.endParams[0].Cause != domain.EndByCandidate || repo.endParams[0].IdleBefore != nil ||
		repo.endParams[0].CandidateID != candidateID || repo.endParams[0].ID != sessionID {
		t.Fatalf("end params = %+v", repo.endParams)
	}
	if len(notifier.ended) != 1 || notifier.ended[0] != sessionID {
		t.Fatalf("notified %v, want exactly one", notifier.ended)
	}
}

func TestAbandonIdleEndsIdleSessionsAsTheSystem(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	first, second := uuid.New(), uuid.New()
	repo := &fakeRepository{
		completeSession: practiceSession(), completeEnded: true,
		abandonRefs: []domain.SessionRef{{ID: first, CandidateID: candidateID}, {ID: second, CandidateID: candidateID}},
		endIdle:     map[uuid.UUID]bool{second: true},
	}
	notifier := &recordingNotifier{}
	svc := newTestService(t, repo, Settings{AbandonTimeout: 15 * time.Minute})
	svc.SetEndNotifier(notifier)

	ended, err := svc.AbandonIdle(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	// The second session was active again under the lock, so only the first is ended.
	if ended != 1 {
		t.Fatalf("ended %d, want 1", ended)
	}
	wantIdle := now.Add(-15 * time.Minute)
	if !repo.abandonNow.Equal(now) || !repo.abandonIdleBefore.Equal(wantIdle) {
		t.Fatalf("listed at %v idle before %v", repo.abandonNow, repo.abandonIdleBefore)
	}
	for _, params := range repo.endParams {
		if params.Cause != domain.EndBySystem || params.IdleBefore == nil || !params.IdleBefore.Equal(wantIdle) {
			t.Fatalf("abandon end params = %+v", params)
		}
	}
	if len(notifier.ended) != 1 {
		t.Fatalf("notified %d sessions, want 1", len(notifier.ended))
	}
}

// One session that cannot be ended is reported, and the pass still ends the others.
func TestAbandonIdleIsolatesAFailingSession(t *testing.T) {
	broken, healthy := uuid.New(), uuid.New()
	repo := &fakeRepository{
		completeSession: practiceSession(), completeEnded: true,
		abandonRefs: []domain.SessionRef{{ID: broken, CandidateID: candidateID}, {ID: healthy, CandidateID: candidateID}},
		endErrs:     map[uuid.UUID]error{broken: errors.New("database unavailable")},
	}
	svc := newTestService(t, repo, Settings{AbandonTimeout: time.Minute})
	ended, err := svc.AbandonIdle(context.Background(), time.Now())
	if err == nil || ended != 1 {
		t.Fatalf("ended=%d err=%v, want 1 and an error", ended, err)
	}
}

func TestAbandonIdleWithoutATimeoutDoesNothing(t *testing.T) {
	repo := &fakeRepository{abandonRefs: []domain.SessionRef{{ID: sessionID, CandidateID: candidateID}}}
	svc := newTestService(t, repo, Settings{})
	if ended, err := svc.AbandonIdle(context.Background(), time.Now()); ended != 0 || err != nil || repo.completeCalls != 0 {
		t.Fatalf("ended=%d err=%v calls=%d", ended, err, repo.completeCalls)
	}
	if _, err := New(repo, Settings{AbandonTimeout: -time.Second}); err == nil {
		t.Fatal("a negative abandon timeout must be refused")
	}
}

func TestAbandonIdlePassesAListErrorThrough(t *testing.T) {
	repo := &fakeRepository{abandonErr: errors.New("database unavailable")}
	svc := newTestService(t, repo, Settings{AbandonTimeout: time.Minute})
	if _, err := svc.AbandonIdle(context.Background(), time.Now()); err == nil {
		t.Fatal("expected the list error")
	}
}
