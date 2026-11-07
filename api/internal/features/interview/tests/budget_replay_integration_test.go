//go:build integration

package tests

import (
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/llmbudget"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

func TestStartReplayWorksAfterUsingTheLastBudgetCall(t *testing.T) {
	for _, kind := range []string{"practice", "recruitment"} {
		t.Run(kind, func(t *testing.T) {
			ctx, pool := openPool(t)
			s := newSeed(t, ctx, pool)
			candidate := s.user("candidate")
			s.wallet(candidate, 20)
			svc := newService(t, pool, 2)
			budget := llmbudget.New(llmbudget.NewPostgresStore(pool), 1)
			svc.SetLLMBudget(budget)
			profile, _ := s.confirmedProfile(candidate, 2)
			input := practiceInput(profile)
			if kind == "recruitment" {
				_, posting, _ := s.openPosting(slotOf(1), 2)
				input = recruitmentInput(s.eligibleApplication(posting, candidate))
			} else {
				input.IdempotencyKey = "last-budget-call"
			}
			first, created, err := svc.Create(ctx, candidate, "candidate", input)
			if err != nil || !created {
				t.Fatalf("first start: created=%v err=%v", created, err)
			}
			again, created, err := svc.Create(ctx, candidate, "candidate", input)
			if err != nil || created || again.ID != first.ID {
				t.Fatalf("replay after exhaustion: id=%s created=%v err=%v", again.ID, created, err)
			}
			if kind == "practice" {
				input.IdempotencyKey = "another-key"
				again, created, err = svc.Create(ctx, candidate, "candidate", input)
				if err != nil || created || again.ID != first.ID {
					t.Fatalf("unfinished practice: created=%v err=%v", created, err)
				}
			}
			if kind == "practice" {
				s.exec(`UPDATE public.interviews SET completed_at = now() WHERE id = $1`, first.ID)
				input.IdempotencyKey = "last-budget-call"
				again, created, err = svc.Create(ctx, candidate, "candidate", input)
				if err != nil || created || again.ID != first.ID {
					t.Fatalf("completed replay: created=%v err=%v", created, err)
				}
			}
			stranger := s.user("candidate")
			if err := budget.Consume(ctx, stranger); err != nil {
				t.Fatal(err)
			}
			_, _, err = svc.Create(ctx, stranger, "candidate", input)
			wantCode := apperror.CodeJDNotFound
			if kind == "recruitment" {
				wantCode = apperror.CodeApplicationNotFound
			}
			if !apperror.IsCode(err, wantCode) {
				t.Fatalf("foreign replay: %v, want %s", err, wantCode)
			}
			other, _ := s.confirmedProfile(candidate, 2)
			_, _, err = svc.Create(ctx, candidate, "candidate", domain.CreateInput{Type: domain.TypePractice, RoleProfileID: &other})
			if !apperror.IsCode(err, apperror.CodeLLMBudgetExceeded) {
				t.Fatalf("new start after exhaustion: %v", err)
			}
			if got := s.count(`SELECT calls FROM public.llm_usage WHERE user_id = $1`, candidate); got != 1 {
				t.Fatalf("budget calls=%d, want 1", got)
			}
		})
	}
}
