//go:build integration

package tests

import (
	"testing"

	evaluationrepository "github.com/cog-forge/rolecue/api/internal/features/evaluation/repository"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// TestRecruitmentStartAgreesWithTheBankCoverageGate (KAN-377): the interview start and the gates of the
// question bank read the same facts (the bank's rubric criteria and the posting's active metrics) through
// their own queries, and apply the same rule. A bank the coverage check passes starts, and a bank it fails is
// refused with the Candidate-facing code, for the same posting, weights and question limit.
func TestRecruitmentStartAgreesWithTheBankCoverageGate(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	m := s.catalogMetrics()
	rA, rB := s.rubric(m[0]), s.rubric(m[1])
	rWide := s.rubric(m[0], m[1], m[2])

	cases := []struct {
		name    string
		rubrics []*uuid.UUID
		weights map[uuid.UUID]float64
		limit   int32
	}{
		{"catalog is active and the bank assesses two metrics", rubricPtrs(rA, rB, rA, rB), nil, 8},
		{"weights on the two assessed metrics", rubricPtrs(rA, rB, rA, rB), map[uuid.UUID]float64{m[0]: 50, m[1]: 50}, 8},
		{"a weight on a metric nobody assesses", rubricPtrs(rA, rB), map[uuid.UUID]float64{m[0]: 50, m[2]: 50}, 8},
		{"a zero weight on a metric nobody assesses", rubricPtrs(rA, rB), map[uuid.UUID]float64{m[0]: 50, m[1]: 50, m[2]: 0}, 8},
		{"the limit is one question too few", rubricPtrs(rA, rB), map[uuid.UUID]float64{m[0]: 50, m[1]: 50}, 1},
		{"one wide question covers three metrics at limit one", rubricPtrs(rWide, rA), map[uuid.UUID]float64{m[0]: 40, m[1]: 30, m[2]: 30}, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			posting, _ := s.postingWithBank(2, c.rubrics)
			if c.weights != nil {
				s.weigh(posting, c.weights)
			}
			report, err := evaluationrepository.PostingCoverage(ctx, pool, posting, int(c.limit))
			if err != nil {
				t.Fatal(err)
			}
			candidate := s.user("candidate")
			_, created, err := newService(t, pool, c.limit).Create(ctx, candidate, "candidate", recruitmentInput(s.eligibleApplication(posting, candidate)))
			if report.Passes {
				if err != nil || !created {
					t.Fatalf("the coverage gate passes the bank but the start gave created=%v err=%v", created, err)
				}
				return
			}
			if created || !apperror.IsCode(err, apperror.CodeQuestionBankCoverageInsufficient) {
				t.Fatalf("the coverage gate fails the bank (%+v) but the start gave created=%v err=%v", report, created, err)
			}
		})
	}
}
