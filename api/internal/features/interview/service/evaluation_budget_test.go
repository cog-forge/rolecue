package service

import (
	"context"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

// The session's AI budget is reserved when it starts, so a spent budget refuses the start before any coin is
// charged or slot used.
func TestCreateIsRefusedBeforeAnythingIsChargedWhenTheBudgetIsSpent(t *testing.T) {
	repo := &fakeRepository{createSession: practiceSession(), createCreated: true}
	svc := newTestService(t, repo, Settings{})
	budget := &stubBudget{left: 0}
	svc.SetLLMBudget(budget)

	_, _, err := svc.Create(context.Background(), candidateID, "candidate", domain.CreateInput{Type: domain.TypePractice, RoleProfileID: &profileID})
	assertCode(t, err, apperror.CodeLLMBudgetExceeded)
	if repo.createCalls != 0 {
		t.Fatalf("repository Create ran %d times for a refused start", repo.createCalls)
	}
}

func TestCreateKeepsOneReservationForANewSessionAndReleasesItOtherwise(t *testing.T) {
	for _, tt := range []struct {
		name         string
		created      bool
		createErr    error
		wantConsumed int
	}{
		{"a new session keeps its reservation", true, nil, 1},
		{"a retry that finds the same session gives it back", false, nil, 0},
		{"a failed start gives it back", false, apperror.New(apperror.CodeInternal, "boom"), 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepository{createSession: practiceSession(), createCreated: tt.created, createErr: tt.createErr}
			svc := newTestService(t, repo, Settings{})
			budget := &stubBudget{left: 5}
			svc.SetLLMBudget(budget)

			_, _, _ = svc.Create(context.Background(), candidateID, "candidate", domain.CreateInput{Type: domain.TypePractice, RoleProfileID: &profileID})
			if consumed := 5 - budget.left; consumed != tt.wantConsumed {
				t.Fatalf("consumed %d calls, want %d", consumed, tt.wantConsumed)
			}
		})
	}
}

// An accepted session is scored even when the budget ran out after it started, whoever ends it.
func TestAcceptedSessionIsScoredAfterTheBudgetRunsOut(t *testing.T) {
	evaluator := newFakeEvaluator()
	repo := &fakeRepository{completeSession: practiceSession(), completeFirst: true, completeEnded: true}
	svc := newTestService(t, repo, Settings{})
	svc.SetEvaluator(evaluator)
	budget := &stubBudget{left: 0}
	svc.SetLLMBudget(budget)

	if _, err := svc.Complete(context.Background(), candidateID, "candidate", sessionID); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	select {
	case <-evaluator.calls:
	case <-time.After(5 * time.Second):
		t.Fatal("the accepted session was not scored")
	}
	if budget.calls != 0 {
		t.Fatalf("the first evaluation charged the budget %d times; the start already paid for it", budget.calls)
	}
}

func TestAbandonedSessionIsScoredWithoutChargingAgain(t *testing.T) {
	evaluator := newFakeEvaluator()
	repo := &fakeRepository{
		completeSession: practiceSession(), completeFirst: true, completeEnded: true,
		abandonRefs: []domain.SessionRef{{ID: sessionID, CandidateID: candidateID}},
	}
	svc := newTestService(t, repo, Settings{AbandonTimeout: time.Minute})
	svc.SetEvaluator(evaluator)
	budget := &stubBudget{left: 0}
	svc.SetLLMBudget(budget)

	if _, err := svc.AbandonIdle(context.Background(), time.Now()); err != nil {
		t.Fatalf("AbandonIdle: %v", err)
	}
	select {
	case <-evaluator.calls:
	case <-time.After(5 * time.Second):
		t.Fatal("the abandoned session was not scored")
	}
	if budget.calls != 0 {
		t.Fatalf("budget charged %d times for an abandoned session", budget.calls)
	}
}

// A retried evaluation is new LLM spend: it takes a call, and when none is left the claim is given back.
func TestRetriedEvaluationIsChargedAndRefusedWhenTheBudgetIsSpent(t *testing.T) {
	t.Run("charged", func(t *testing.T) {
		evaluator := newFakeEvaluator()
		repo := &fakeRepository{completeSession: practiceSession(), completeFirst: true, completeAttemptNo: 2}
		svc := newTestService(t, repo, Settings{})
		svc.SetEvaluator(evaluator)
		budget := &stubBudget{left: 3}
		svc.SetLLMBudget(budget)

		if _, err := svc.Complete(context.Background(), candidateID, "candidate", sessionID); err != nil {
			t.Fatal(err)
		}
		select {
		case <-evaluator.calls:
		case <-time.After(5 * time.Second):
			t.Fatal("the retry did not run")
		}
		if budget.calls != 1 {
			t.Fatalf("budget calls = %d, want 1 for the retry", budget.calls)
		}
	})
	t.Run("refused", func(t *testing.T) {
		evaluator := newFakeEvaluator()
		repo := &fakeRepository{completeSession: practiceSession(), completeFirst: true, completeAttemptNo: 2}
		svc := newTestService(t, repo, Settings{})
		svc.SetEvaluator(evaluator)
		svc.SetLLMBudget(&stubBudget{left: 0})

		_, err := svc.Complete(context.Background(), candidateID, "candidate", sessionID)
		assertCode(t, err, apperror.CodeLLMBudgetExceeded)
		if len(repo.released) != 1 || len(evaluator.calls) != 0 {
			t.Fatalf("released %d claims, evaluator calls %d; want the claim given back and no run", len(repo.released), len(evaluator.calls))
		}
	})
}

// Repeating a completion that claims nothing never touches the budget.
func TestRepeatedCompleteDoesNotChargeTheBudget(t *testing.T) {
	repo := &fakeRepository{completeSession: practiceSession()}
	svc := newTestService(t, repo, Settings{})
	svc.SetEvaluator(newFakeEvaluator())
	budget := &stubBudget{left: 3}
	svc.SetLLMBudget(budget)

	for i := 0; i < 3; i++ {
		if _, err := svc.Complete(context.Background(), candidateID, "candidate", sessionID); err != nil {
			t.Fatal(err)
		}
	}
	if budget.calls != 0 {
		t.Fatalf("budget calls = %d, want 0", budget.calls)
	}
}
