package domain_test

import (
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/roadmap/domain"
)

func f64(v float64) *float64 { return &v }

func TestClassifyCore(t *testing.T) {
	tests := []struct {
		name        string
		facts       domain.CoreFacts
		wantOutcome domain.CoreOutcome
		wantScore   *float64
	}{
		{"answered and graded keeps its score", domain.CoreFacts{Delivered: true, Answered: true, QuestionScore: f64(72.5)}, domain.CoreScored, f64(72.5)},
		{"answered and graded 0 stays a real 0", domain.CoreFacts{Delivered: true, Answered: true, QuestionScore: f64(0)}, domain.CoreScored, f64(0)},
		{"answer over REST, never delivered live, is still scored", domain.CoreFacts{Answered: true, QuestionScore: f64(60)}, domain.CoreScored, f64(60)},
		{"delivered and not answered is a skip and counts 0", domain.CoreFacts{Delivered: true}, domain.CoreSkipped, f64(0)},
		{"never delivered and not answered is not asked", domain.CoreFacts{}, domain.CoreNotAsked, nil},
		{"answered but ungraded is not counted and not 0", domain.CoreFacts{Delivered: true, Answered: true}, domain.CoreUngraded, nil},
		{"answered over REST but ungraded is not counted", domain.CoreFacts{Answered: true}, domain.CoreUngraded, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.ClassifyCore(tc.facts)
			if got.Outcome != tc.wantOutcome {
				t.Fatalf("outcome = %s, want %s", got.Outcome, tc.wantOutcome)
			}
			switch {
			case tc.wantScore == nil && got.Score != nil:
				t.Fatalf("score = %v, want nil", *got.Score)
			case tc.wantScore != nil && (got.Score == nil || *got.Score != *tc.wantScore):
				t.Fatalf("score = %v, want %v", got.Score, *tc.wantScore)
			}
			if got.Counted() != (tc.wantScore != nil) {
				t.Fatalf("Counted() = %v, want %v", got.Counted(), tc.wantScore != nil)
			}
		})
	}
}

func TestSessionEligible(t *testing.T) {
	done := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		s    domain.SessionFacts
		want bool
	}{
		{"completed practice with a successful evaluation", domain.SessionFacts{Type: "practice", CompletedAt: &done, EvaluationStatus: "completed"}, true},
		{"recruitment never feeds the Roadmap", domain.SessionFacts{Type: "recruitment", CompletedAt: &done, EvaluationStatus: "completed"}, false},
		{"unfinished session", domain.SessionFacts{Type: "practice", EvaluationStatus: "completed"}, false},
		{"evaluation pending", domain.SessionFacts{Type: "practice", CompletedAt: &done, EvaluationStatus: "pending"}, false},
		{"evaluation failed", domain.SessionFacts{Type: "practice", CompletedAt: &done, EvaluationStatus: "failed"}, false},
		{"injection-flagged evaluation is not trusted evidence", domain.SessionFacts{Type: "practice", CompletedAt: &done, EvaluationStatus: "completed", InjectionFlagged: true}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.s.Eligible(); got != tc.want {
				t.Fatalf("Eligible() = %v, want %v", got, tc.want)
			}
		})
	}
}
