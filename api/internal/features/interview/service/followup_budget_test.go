package service

import (
	"context"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/llmbudget"
	"github.com/google/uuid"
)

type stubBudget struct {
	left     int
	calls    int
	released int
	user     uuid.UUID
}

func (b *stubBudget) Release(context.Context, uuid.UUID) error {
	b.released++
	b.left++
	return nil
}

func (b *stubBudget) Consume(_ context.Context, user uuid.UUID) error {
	b.calls++
	b.user = user
	if b.left <= 0 {
		return llmbudget.ErrExceeded()
	}
	b.left--
	return nil
}

// A spent LLM budget skips the follow-up (the decider is not called) and never fails the interview.
func TestFollowUpIsSkippedWhenTheLLMBudgetIsSpent(t *testing.T) {
	decider := &scriptedDecider{proposal: domain.FollowUpProposal{FollowUp: true, Question: "Which algorithm would you pick for bursts?", Reason: "r"}}
	svc, _ := followUpService(t, decider, 1)
	budget := &stubBudget{left: 1}
	svc.SetLLMBudget(budget)

	first, err := svc.FollowUp(context.Background(), candidateID, "candidate", sessionID)
	if err != nil || first.Question == nil {
		t.Fatalf("first follow-up = %+v, %v; want a question", first, err)
	}
	second, err := svc.FollowUp(context.Background(), candidateID, "candidate", sessionID)
	if err != nil || second.Question != nil {
		t.Fatalf("second follow-up = %+v, %v; want none and no error", second, err)
	}
	if decider.calls != 1 || budget.calls != 2 || budget.user != candidateID {
		t.Fatalf("decider calls = %d, budget calls = %d, user = %s", decider.calls, budget.calls, budget.user)
	}
}
