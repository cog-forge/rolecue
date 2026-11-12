package domain

import (
	evaluationdomain "github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// Metric-coverage sampling (KAN-333, RB-09, Brain Decision 24 section 3.1).
//
// A session asks at most `limit` core questions, drawn from the bank of its Job Description. The scoring
// policy never treats a metric nobody assessed as 0 and never rescales weights to hide it, so the sample
// itself must give every active metric (positive weight) at least one question whose rubric has a
// criterion for it. SampleQuestions is the pure rule for that: it has no I/O and no clock, and its only
// randomness is the injected RandomSource, so a test can make it deterministic.
//
// Which sets of questions cover the active metrics is decided by evaluation/domain.AnalyzeCoverage, the one
// rule that the Recruiter's bank screen, the submission, the Admin approval and practice generation also
// use, so a bank those gates accept is a bank this sampler accepts. The rule, in order:
//  1. A bank in which no question has rubric criteria is sampled at random, exactly as before Barem. The
//     same holds when no metric is active. Nothing breaks for a bank that has no rubrics yet.
//  2. Otherwise every active metric must be covered within the limit. The cover is a smallest one found by a
//     randomized greedy choice and, when greedy overshoots the limit although a cover exists, by an exact
//     search. SampleQuestions refuses a bank that truly cannot cover within the limit.
//  3. The cover comes first, ordered by how much new coverage each question brings, then random questions
//     fill the remaining slots. A session that ends early has therefore met every coverable metric as soon
//     as the cover (the first k positions) has been asked.
//
// A question without a rubric, or whose rubric has no criteria, covers nothing but may fill a slot.

// RandomSource is the one source of randomness of the sampling. *rand.Rand of math/rand/v2 satisfies it;
// so does the package-level generator behind a small adapter. It is injected so tests can fix the result.
type RandomSource interface {
	// IntN returns a random int in [0, n). n is always positive.
	IntN(n int) int
}

// SamplingQuestion is one question of a Job Description's bank as the sampler sees it: its id and the
// distinct metrics its rubric has criteria for (empty without a rubric or when the rubric has none).
type SamplingQuestion = evaluationdomain.CoverageQuestion

// ErrQuestionCoverageInsufficient is returned when the bank cannot cover every active metric within the
// question limit. Its message is generic on purpose: it names no metric, rubric or count, because a
// Candidate sees it.
var ErrQuestionCoverageInsufficient = apperror.New(apperror.CodeQuestionBankCoverageInsufficient,
	"this interview cannot be started right now")

// SampleQuestions picks the core questions of a recruitment session: at most limit distinct bank
// questions, in the order they will be asked. active is the set of active metric ids (duplicates count
// once). It returns ErrQuestionBankNotReady for an empty bank or a limit below 1, as the sampling it
// replaces did, and ErrQuestionCoverageInsufficient when the bank cannot cover every active metric within
// limit.
func SampleQuestions(bank []SamplingQuestion, active []uuid.UUID, limit int, random RandomSource) ([]uuid.UUID, error) {
	if len(bank) == 0 || limit <= 0 {
		return nil, ErrQuestionBankNotReady
	}
	order := shuffled(len(bank), random)
	coverage, err := evaluationdomain.AnalyzeCoverage(bank, active, limit, order)
	if err != nil {
		return nil, err
	}
	if !coverage.Passes {
		return nil, ErrQuestionCoverageInsufficient
	}
	return coverThenFill(bank, coverage.Cover, order, limit), nil
}

// coverThenFill returns the questions of the cover (bank indices, already ordered by new coverage) followed by
// random questions, in the shuffled order, until limit are picked.
func coverThenFill(bank []SamplingQuestion, cover, order []int, limit int) []uuid.UUID {
	picked := make([]uuid.UUID, 0, min(limit, len(bank)))
	taken := make([]bool, len(bank))
	for _, i := range cover {
		taken[i] = true
		picked = append(picked, bank[i].ID)
	}
	for _, i := range order {
		if len(picked) >= limit {
			break
		}
		if !taken[i] {
			picked = append(picked, bank[i].ID)
		}
	}
	return picked
}

// shuffled returns 0..n-1 in random order (Fisher-Yates).
func shuffled(n int, random RandomSource) []int {
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	for i := n - 1; i > 0; i-- {
		j := random.IntN(i + 1)
		order[i], order[j] = order[j], order[i]
	}
	return order
}
