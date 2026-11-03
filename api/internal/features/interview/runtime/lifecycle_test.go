package runtime

import (
	"context"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/features/interview/service"
	"github.com/gorilla/websocket"
)

// Leaving ends the interview (decision 2026-11-03). These tests run the engine over the real service, so
// the end path is the one REST and the socket share, and the hub is the service's end notifier.

func lifecycleHarness(t *testing.T, repo *memoryRepository) (*harness, *service.Service) {
	t.Helper()
	svc, err := service.New(repo, service.Settings{})
	if err != nil {
		t.Fatal(err)
	}
	h := serve(t, svc)
	svc.SetEndNotifier(h.engine.Hub())
	return h, svc
}

// The Candidate leaves over REST while the room's socket is open: the socket reports the end and closes
// with 1000 instead of waiting for an answer that will never come.
func TestLeavingOverRESTClosesTheOpenSocketWithTheEnd(t *testing.T) {
	repo := newMemoryRepository("Q1", "Q2", "Q3")
	h, svc := lifecycleHarness(t, repo)
	c, _, turn := h.connect()
	c.listen("played-1", turn.Position)

	session, err := svc.Complete(context.Background(), testCandidate, "candidate", testSession)
	if err != nil {
		t.Fatalf("leave: %v", err)
	}
	if session.CompletedAt == nil {
		t.Fatal("the session must be ended")
	}
	var sync SessionSync
	c.expect(TypeSessionSync, &sync)
	if sync.State != StateCompleted || sync.Position != nil || sync.EvaluationStatus == nil {
		t.Fatalf("sync after leave = %+v", sync)
	}
	c.expect(TypeSessionCompleted, nil)
	c.expectClose(websocket.CloseNormalClosure)
}

// A reconnect after leaving is refused the interview: it is told the session completed and closed, and
// no question is delivered again.
func TestReconnectAfterLeaveIsRefused(t *testing.T) {
	repo := newMemoryRepository("Q1", "Q2")
	h, svc := lifecycleHarness(t, repo)
	if _, err := svc.Complete(context.Background(), testCandidate, "candidate", testSession); err != nil {
		t.Fatalf("leave: %v", err)
	}
	c := h.dial()
	var sync SessionSync
	c.expect(TypeSessionSync, &sync)
	if sync.State != StateCompleted {
		t.Fatalf("sync = %+v", sync)
	}
	c.expect(TypeSessionCompleted, nil)
	c.expectClose(websocket.CloseNormalClosure)
	for _, msg := range c.seen {
		if msg.Type == TypeInterviewerTurn {
			t.Fatal("a question was delivered after leaving")
		}
	}
}

// An answer in flight when the session ends elsewhere is refused and the socket reports the end; the
// session ends once.
func TestAnswerAfterLeaveReportsTheEnd(t *testing.T) {
	sessions := newFakeSessions(domain.TypePractice, "Q1", "Q2")
	h := newHarness(t, sessions)
	c, _, turn := h.connect()
	c.listen("played-1", turn.Position)
	sessions.set(func(f *fakeSessions) { f.completed = true })

	c.send(TypeAnswerText, "late", AnswerText{Position: turn.Position, Text: "too late"})
	c.expectState(StateProcessing)
	c.expect(TypeSessionSync, nil)
	c.expect(TypeSessionCompleted, nil)
	c.expectClose(websocket.CloseNormalClosure)
	if answers, _, completes := sessions.counts(); answers != 0 || completes != 0 {
		t.Fatalf("answers=%d completes=%d after the session ended elsewhere", answers, completes)
	}
}

// SessionEnded for a session with no socket in this process does nothing.
func TestSessionEndedWithoutASocketIsANoOp(t *testing.T) {
	hub := NewHub()
	hub.SessionEnded(testSession)
	l, err := hub.attach(testSession)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-l.ended:
		t.Fatal("a socket that attached after the end must learn it from the service, not the hub")
	default:
	}
	hub.SessionEnded(testSession)
	hub.SessionEnded(testSession)
	<-l.ended
	hub.release(l)
}
