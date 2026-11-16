package service

import (
	"context"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
)

// How the daily LLM budget reads an evaluation (KAN-383, F3): the unit reserved when the session started
// pays for the first run, so a first claim takes nothing; every later claim (a failed run claimed again) takes
// one. The repeats of a single grading call are inside Evaluate, which has no budget, so they take nothing.
func TestEvaluationClaimsTakeBudgetOnlyFromTheSecondAttempt(t *testing.T) {
	evaluator := newFakeEvaluator()
	repo := &fakeRepository{completeSession: practiceSession(), completeEnded: true, completeCap: domain.EvaluationMaxAttempts}
	svc := newTestService(t, repo, Settings{})
	svc.SetEvaluator(evaluator)
	budget := &stubBudget{left: 10}
	svc.SetLLMBudget(budget)

	for attempt := int32(1); attempt <= domain.EvaluationMaxAttempts; attempt++ {
		if _, err := svc.Complete(context.Background(), candidateID, "candidate", sessionID); err != nil {
			t.Fatalf("Complete %d: %v", attempt, err)
		}
		select {
		case <-evaluator.calls:
		case <-time.After(5 * time.Second):
			t.Fatalf("evaluation %d was not started", attempt)
		}
		if want := int(attempt) - 1; budget.calls != want {
			t.Fatalf("after claim %d the budget was asked %d times, want %d", attempt, budget.calls, want)
		}
	}
}
