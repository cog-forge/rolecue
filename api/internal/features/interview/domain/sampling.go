package domain

import (
	"errors"
	"math/bits"

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
// The rule, in order:
//  1. A bank in which no question has rubric criteria is sampled at random, exactly as before Barem. The
//     same holds when no metric is active. Nothing breaks for a bank that has no rubrics yet.
//  2. Otherwise every active metric must be covered within the limit. The cover is built by a randomized
//     greedy choice (the question covering the most still-uncovered metrics, ties broken at random).
//     Greedy can overshoot the limit although a cover exists, so an exact search over metric sets runs
//     before a bank is refused: a bank is refused only when it truly cannot cover within the limit.
//  3. The cover comes first, ordered by how much new coverage each question brings, then random questions
//     fill the remaining slots. A session that ends early has therefore met every metric as soon as the
//     cover (the first k positions) has been asked.
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
type SamplingQuestion struct {
	ID      uuid.UUID
	Metrics []uuid.UUID
}

// ErrQuestionCoverageInsufficient is returned when the bank cannot cover every active metric within the
// question limit. Its message is generic on purpose: it names no metric, rubric or count, because a
// Candidate sees it.
var ErrQuestionCoverageInsufficient = apperror.New(apperror.CodeQuestionBankCoverageInsufficient,
	"this interview cannot be started right now")

const (
	// maxActiveMetrics is how many active metrics a bitmask can hold. The metric catalog is a handful of
	// rows, so this is far above any real value.
	maxActiveMetrics = 62
	// maxCoverStates bounds the exact search. With the metric counts above it is never reached.
	maxCoverStates = 1 << 18
)

var (
	errTooManyMetrics = errors.New("interview sampling: too many active metrics")
	errSearchTooLarge = errors.New("interview sampling: cover search too large")
)

// SampleQuestions picks the core questions of a session: at most limit distinct bank questions, in the
// order they will be asked. active is the set of active metric ids (duplicates count once). It returns
// ErrQuestionBankNotReady for an empty bank or a limit below 1, as the sampling it replaces did, and
// ErrQuestionCoverageInsufficient when the bank cannot cover every active metric within limit.
func SampleQuestions(bank []SamplingQuestion, active []uuid.UUID, limit int, random RandomSource) ([]uuid.UUID, error) {
	if len(bank) == 0 || limit <= 0 {
		return nil, ErrQuestionBankNotReady
	}
	bit, err := metricBits(active)
	if err != nil {
		return nil, err
	}
	masks := make([]uint64, len(bank))
	hasRubric := false
	for i, question := range bank {
		if len(question.Metrics) > 0 {
			hasRubric = true
		}
		for _, metric := range question.Metrics {
			if b, ok := bit[metric]; ok {
				masks[i] |= 1 << b
			}
		}
	}

	order := shuffled(len(bank), random)
	var cover []int
	if hasRubric && len(bit) > 0 {
		full := uint64(1)<<len(bit) - 1
		cover = greedyCover(masks, order, full, limit)
		if cover == nil {
			cover, err = exactCover(masks, order, full, limit)
			if err != nil {
				return nil, err
			}
		}
		if cover == nil {
			return nil, ErrQuestionCoverageInsufficient
		}
		cover = orderByNewCoverage(masks, cover)
	}

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
	return picked, nil
}

// metricBits numbers the distinct active metrics 0..n-1.
func metricBits(active []uuid.UUID) (map[uuid.UUID]int, error) {
	bit := make(map[uuid.UUID]int, len(active))
	for _, metric := range active {
		if _, ok := bit[metric]; !ok {
			bit[metric] = len(bit)
		}
	}
	if len(bit) > maxActiveMetrics {
		return nil, errTooManyMetrics
	}
	return bit, nil
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

// greedyCover picks, one at a time, the question that covers the most still-uncovered metrics; among
// equals it takes the first in the shuffled order, which is a uniform random pick. It returns nil when the
// metrics are not all covered within limit questions.
func greedyCover(masks []uint64, order []int, full uint64, limit int) []int {
	var cover []int
	var covered uint64
	taken := make(map[int]bool)
	for covered != full {
		if len(cover) >= limit {
			return nil
		}
		best, bestGain := -1, 0
		for _, i := range order {
			if taken[i] {
				continue
			}
			if gain := bits.OnesCount64(masks[i] &^ covered); gain > bestGain {
				best, bestGain = i, gain
			}
		}
		if best < 0 {
			return nil
		}
		taken[best] = true
		covered |= masks[best]
		cover = append(cover, best)
	}
	return cover
}

// coverStep records how a set of covered metrics was first reached.
type coverStep struct {
	from     uint64
	question int
}

// exactCover finds a cover of at most limit questions, a smallest one, or nil when none exists. It is a
// breadth-first search over the sets of metrics covered so far: every reachable set is visited once, so the
// cost depends on the metric catalog, not on the bank size. Which smallest cover is found follows the
// shuffled order, so it is random among the ones the search meets first.
func exactCover(masks []uint64, order []int, full uint64, limit int) ([]int, error) {
	reached := map[uint64]coverStep{0: {}}
	frontier := []uint64{0}
	for depth := 1; depth <= limit && len(frontier) > 0; depth++ {
		var next []uint64
		for _, state := range frontier {
			for _, i := range order {
				merged := state | masks[i]
				if merged == state {
					continue
				}
				if _, seen := reached[merged]; seen {
					continue
				}
				if len(reached) >= maxCoverStates {
					return nil, errSearchTooLarge
				}
				reached[merged] = coverStep{from: state, question: i}
				if merged == full {
					return coverPath(reached, full), nil
				}
				next = append(next, merged)
			}
		}
		frontier = next
	}
	return nil, nil
}

// coverPath walks the recorded steps back from the full set to the empty one.
func coverPath(reached map[uint64]coverStep, full uint64) []int {
	var path []int
	for state := full; state != 0; {
		step := reached[state]
		path = append(path, step.question)
		state = step.from
	}
	return path
}

// orderByNewCoverage orders a cover so each question brings the most metrics not yet covered by the ones
// before it; a tie keeps the order it had.
func orderByNewCoverage(masks []uint64, cover []int) []int {
	remaining := append([]int(nil), cover...)
	ordered := make([]int, 0, len(cover))
	var covered uint64
	for len(remaining) > 0 {
		best := 0
		for k := 1; k < len(remaining); k++ {
			if bits.OnesCount64(masks[remaining[k]]&^covered) > bits.OnesCount64(masks[remaining[best]]&^covered) {
				best = k
			}
		}
		covered |= masks[remaining[best]]
		ordered = append(ordered, remaining[best])
		remaining = append(remaining[:best], remaining[best+1:]...)
	}
	return ordered
}
