package domain

import (
	evaluationdomain "github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/google/uuid"
)

// Best-effort sampling for practice (KAN-378, RB-18). A Candidate cannot edit the bank of a practice session,
// so refusing a bank that cannot cover every active metric would be a dead end. The practice sampler takes
// the widest cover that fits in the question limit instead: the most metrics, then the fewest questions,
// random among equals (evaluation/domain.AnalyzeCoverage already finds it, whether or not the bank passes),
// puts it first and fills the rest at random. The session always starts; the metrics it cannot reach are
// withheld by grading (KAN-334), never scored 0.

// Sampler is the signature shared by SampleQuestions and SampleQuestionsBestEffort, so a start picks which
// rule applies to it.
type Sampler func(bank []SamplingQuestion, active []uuid.UUID, limit int, random RandomSource) ([]uuid.UUID, error)

// SampleQuestionsBestEffort picks the core questions of a practice session. It is SampleQuestions except
// that it never refuses for coverage: when the bank cannot cover every active metric within limit it takes
// the widest cover that fits and fills the rest at random. It still returns ErrQuestionBankNotReady for an
// empty bank or a limit below 1. It does not fail on a coverage search error either (more active metrics
// than the search holds): it samples at random instead, because a Candidate cannot repair a bank.
func SampleQuestionsBestEffort(bank []SamplingQuestion, active []uuid.UUID, limit int, random RandomSource) ([]uuid.UUID, error) {
	if len(bank) == 0 || limit <= 0 {
		return nil, ErrQuestionBankNotReady
	}
	order := shuffled(len(bank), random)
	coverage, err := evaluationdomain.AnalyzeCoverage(bank, active, limit, order)
	if err != nil {
		coverage = evaluationdomain.Coverage{}
	}
	return coverThenFill(bank, coverage.Cover, order, limit), nil
}
