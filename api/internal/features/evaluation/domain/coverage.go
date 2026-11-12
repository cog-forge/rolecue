package domain

import (
	"errors"
	"math/bits"

	"github.com/google/uuid"
)

// Metric coverage of a question bank (KAN-377 RB-17, KAN-378 RB-18, Brain Decision 24 section 3.1).
//
// A metric is active for a posting when its saved weight is positive (no saved weights: the whole catalog).
// A session asks at most `limit` core questions, so it can be graded on every active metric only if some
// set of at most `limit` bank questions has, among its rubric criteria, one for each active metric. This
// file is the one pure rule for that. The interview sampler (KAN-333) uses it to pick a session's
// questions, and the Recruiter's bank screen, posting submission, Admin approval and practice generation
// use it to ask the same question earlier, while the bank can still change. A bank that passes here is
// exactly a bank the sampler accepts.
//
// The search is exact. The metric catalog is a handful of rows, so a breadth-first search over the sets of
// metrics covered so far (at most 2^metrics states, one per set) finds the widest cover reachable with at
// most `limit` questions, and the smallest of those.

// DefaultQuestionLimit is how many core questions a session asks when INTERVIEW_QUESTION_LIMIT is unset.
// The interview start and every coverage check read the limit through ResolveQuestionLimit, so they can
// never disagree.
const DefaultQuestionLimit = 8

// ResolveQuestionLimit applies the default to a configured question limit (zero or less means unset).
func ResolveQuestionLimit(configured int) int {
	if configured <= 0 {
		return DefaultQuestionLimit
	}
	return configured
}

const (
	// MaxCoverageMetrics is how many active metrics a bitmask can hold. The metric catalog is a handful of
	// rows, so this is far above any real value.
	MaxCoverageMetrics = 62
	// maxCoverStates bounds the exact search. With the metric counts above it is never reached.
	maxCoverStates = 1 << 18
)

var (
	// ErrTooManyMetrics is returned for more active metrics than the search can hold.
	ErrTooManyMetrics = errors.New("coverage: too many active metrics")
	// ErrCoverSearchTooLarge is returned when the exact search would visit more sets than allowed.
	ErrCoverSearchTooLarge = errors.New("coverage: cover search too large")
)

// CoverageQuestion is one question of a bank as the coverage rule sees it: its id and the distinct metrics
// its rubric has criteria for (empty for a question without a rubric or whose rubric has no criteria).
// Archived rubrics count: a question keeps the rubric it was authored with and grading uses it.
type CoverageQuestion struct {
	ID      uuid.UUID
	Metrics []uuid.UUID
}

// Coverage is the verdict for a bank.
type Coverage struct {
	// Passes is the binary gate: every active metric is covered by some set of at most `limit` questions.
	// A bank that has no rubric data at all passes without being assessed, as the sampler samples it at
	// random (a bank from before Barem), and so does a posting with no active metric. An empty bank or a
	// limit below 1 never passes.
	Passes bool
	// Assessed is false for the passes that assessed nothing (no rubric data, no active metric) and for an
	// empty bank. Screens show a coverage warning only for an assessed bank.
	Assessed bool
	// Total is the number of distinct active metrics. Covered is how many of them the widest cover of at
	// most `limit` questions reaches; it equals Total exactly when the bank passes. These two feed the
	// display "n of m metrics"; only Passes gates anything.
	Total   int
	Covered int
	// Missing is every active metric that no question of the bank covers at all, in the order of `active`.
	// Only a change to the bank (a question whose rubric has the metric) can fix those.
	Missing []uuid.UUID
	// ExceedsLimit is set when nothing is missing from the bank as a whole but no `limit` questions can
	// cover everything (questions overlap too little); MinQuestions is then how many questions it would
	// take.
	ExceedsLimit bool
	MinQuestions int
	// Cover holds the indices into the bank of the widest cover found, ordered so each question brings the
	// most metrics not yet covered by the ones before it. When the bank passes it is a smallest cover.
	Cover []int
}

// CheckCoverage is AnalyzeCoverage with the bank order as the search order: deterministic, for callers
// that only want the verdict.
func CheckCoverage(bank []CoverageQuestion, active []uuid.UUID, limit int) (Coverage, error) {
	return AnalyzeCoverage(bank, active, limit, nil)
}

// AnalyzeCoverage returns the verdict for a bank. order is a permutation of the bank indices that sets which
// of several equally good covers is found (the sampler passes a random one); nil means the bank order. The
// order never changes Passes, Covered, Missing or ExceedsLimit.
func AnalyzeCoverage(bank []CoverageQuestion, active []uuid.UUID, limit int, order []int) (Coverage, error) {
	bit, distinct, err := metricBits(active)
	if err != nil {
		return Coverage{}, err
	}
	result := Coverage{Total: len(distinct)}
	if len(bank) == 0 || limit <= 0 {
		result.Missing = distinct
		return result, nil
	}
	masks := make([]uint64, len(bank))
	hasRubric := false
	var union uint64
	for i, question := range bank {
		if len(question.Metrics) > 0 {
			hasRubric = true
		}
		for _, metric := range question.Metrics {
			if b, ok := bit[metric]; ok {
				masks[i] |= 1 << b
			}
		}
		union |= masks[i]
	}
	if !hasRubric || len(distinct) == 0 {
		result.Passes = true
		result.Missing = distinct
		return result, nil
	}
	result.Assessed = true
	for i, metric := range distinct {
		if union&(1<<i) == 0 {
			result.Missing = append(result.Missing, metric)
		}
	}
	if order == nil {
		order = make([]int, len(bank))
		for i := range order {
			order[i] = i
		}
	}

	full := uint64(1)<<len(distinct) - 1
	cover, covered, err := widestCover(masks, order, full, limit)
	if err != nil {
		return Coverage{}, err
	}
	result.Cover = orderByNewCoverage(masks, cover)
	result.Covered = bits.OnesCount64(covered)
	result.Passes = covered == full
	if !result.Passes && union == full {
		result.ExceedsLimit = true
		smallest, _, err := widestCover(masks, order, full, len(bank))
		if err != nil {
			return Coverage{}, err
		}
		result.MinQuestions = len(smallest)
	}
	return result, nil
}

// metricBits numbers the distinct active metrics 0..n-1 and returns them in first-seen order.
func metricBits(active []uuid.UUID) (map[uuid.UUID]int, []uuid.UUID, error) {
	bit := make(map[uuid.UUID]int, len(active))
	distinct := make([]uuid.UUID, 0, len(active))
	for _, metric := range active {
		if _, ok := bit[metric]; !ok {
			bit[metric] = len(distinct)
			distinct = append(distinct, metric)
		}
	}
	if len(distinct) > MaxCoverageMetrics {
		return nil, nil, ErrTooManyMetrics
	}
	return bit, distinct, nil
}

// widestCover returns the questions (bank indices) of a cover that reaches the most metrics of full with at
// most limit questions, the smallest such cover, and the metrics it reaches. It first tries a randomized
// greedy choice, which is cheap and spreads picks evenly among equal questions, and only searches exactly
// when greedy does not reach every metric: greedy can overshoot the limit although a cover exists.
func widestCover(masks []uint64, order []int, full uint64, limit int) ([]int, uint64, error) {
	if cover, ok := greedyCover(masks, order, full, limit); ok {
		return cover, full, nil
	}
	return exactWidestCover(masks, order, full, limit)
}

// greedyCover picks, one at a time, the question that covers the most still-uncovered metrics; among equals
// it takes the first in order. It reports false when the metrics are not all covered within limit.
func greedyCover(masks []uint64, order []int, full uint64, limit int) ([]int, bool) {
	var cover []int
	var covered uint64
	taken := make(map[int]bool)
	for covered != full {
		if len(cover) >= limit {
			return nil, false
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
			return nil, false
		}
		taken[best] = true
		covered |= masks[best]
		cover = append(cover, best)
	}
	return cover, true
}

// coverStep records how a set of covered metrics was first reached.
type coverStep struct {
	from     uint64
	question int
}

// exactWidestCover is a breadth-first search over the sets of metrics covered so far: every set reachable
// with at most limit questions is visited once, at its smallest question count, so the cost depends on the
// metric catalog, not on the bank size. It returns the reachable set with the most metrics (the first one
// met on a tie, which is the smallest and, among those, follows the search order) and the questions that
// reach it.
func exactWidestCover(masks []uint64, order []int, full uint64, limit int) ([]int, uint64, error) {
	reached := map[uint64]coverStep{0: {}}
	frontier := []uint64{0}
	var best uint64
	for depth := 1; depth <= limit && len(frontier) > 0 && best != full; depth++ {
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
					return nil, 0, ErrCoverSearchTooLarge
				}
				reached[merged] = coverStep{from: state, question: i}
				if bits.OnesCount64(merged) > bits.OnesCount64(best) {
					best = merged
				}
				next = append(next, merged)
			}
		}
		frontier = next
	}
	return coverPath(reached, best), best, nil
}

// coverPath walks the recorded steps back from a reached set to the empty one.
func coverPath(reached map[uint64]coverStep, state uint64) []int {
	var path []int
	for state != 0 {
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
