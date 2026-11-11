package service

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"math/big"
	"slices"
	"strconv"

	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/google/uuid"
)

// Pure rubric scoring (KAN-332, RB-08): criterion points -> metric score in each core question -> metric
// score of the session -> weighted overall. There is no I/O, no AI and no clock here, and nothing in the
// live scoring path calls it yet (that is RB-10 and RB-11).
//
// All arithmetic is exact: points are integers and every score is a rational number (math/big), so a value
// that is exactly halfway at the last decimal rounds up the same way on every machine; binary floating point
// would turn 12.345 into 12.3449999... Floats appear only in the returned values.

var (
	// ErrInvalidRubricScore wraps every refusal of the input: a bad point value, a duplicate criterion or
	// question, an inconsistent criterion, bad weights. Nothing is scored from invalid input.
	ErrInvalidRubricScore = errors.New("invalid rubric score input")
	// ErrInvalidScoringPolicy is returned for a policy the calculator cannot apply.
	ErrInvalidScoringPolicy = errors.New("invalid scoring policy")
)

func invalidScore(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidRubricScore, fmt.Sprintf(format, args...))
}

// CalculateRubricScore applies the spec's formulas (docs/SPEC-scoring-rubrics.md, section 2.5):
//
//	question_metric(q, m) = 100 x sum earned(q, c) / sum max(c)      over the criteria of metric m in q
//	question_total(q)     = 100 x sum earned(q, c) / sum max(c)      over all criteria of q (display only)
//	session_metric(m)     = mean over the scored questions that assess m, each question once
//	overall               = round(sum session_metric(m) x weight(m) / sum weight(m))
//
// A question that does not assess a metric is not a zero for it. The question total is never averaged
// into the overall. Follow-ups do not appear in the input at all: their evidence changes the points of their
// core question, so a repeated question position is refused. The policy decides the points settled on 11/11 (precision,
// missing metric coverage, partial sessions); see domain.DefaultScoringPolicy.
func CalculateRubricScore(in domain.RubricScoreInput, policy domain.ScoringPolicy) (domain.RubricScore, error) {
	if err := validatePolicy(policy); err != nil {
		return domain.RubricScore{}, err
	}
	weights, err := parseWeights(in.Weights)
	if err != nil {
		return domain.RubricScore{}, err
	}
	questions, err := validateQuestions(in)
	if err != nil {
		return domain.RubricScore{}, err
	}

	scale := math.Pow10(policy.MetricDecimals)
	result := domain.RubricScore{ScoredQuestions: int32(len(questions)), AssignedQuestions: in.AssignedQuestions}
	sums := make(map[uuid.UUID]*big.Rat)
	counts := make(map[uuid.UUID]int32)
	for _, q := range questions {
		scored, exact := scoreQuestion(q, policy.MetricDecimals, scale)
		result.Questions = append(result.Questions, scored)
		for metric, ratio := range exact {
			if sums[metric] == nil {
				sums[metric] = new(big.Rat)
			}
			sums[metric].Add(sums[metric], ratio)
			counts[metric]++
		}
	}

	// Session metrics are stored at the policy precision, and the overall is calculated from those stored values.
	stored := make(map[uuid.UUID]int64, len(sums))
	for _, metric := range sortedIDs(sums) {
		mean := new(big.Rat).Quo(sums[metric], new(big.Rat).SetInt64(int64(counts[metric])))
		stored[metric] = roundHalfUp(mean, policy.MetricDecimals)
		weight := 0.0
		if w, ok := weights.byMetric[metric]; ok {
			weight = w.raw
		}
		result.Metrics = append(result.Metrics, domain.RubricMetricScore{
			MetricID: metric, Score: float64(stored[metric]) / scale, Weight: weight, Questions: counts[metric],
		})
	}

	included, missing := weights.cover(stored)
	result.MissingMetrics = missing
	partialSession := in.AssignedQuestions > 0 && result.ScoredQuestions < in.AssignedQuestions
	reason := withheldReason(policy, len(missing) > 0, len(included) == 0, partialSession)
	if reason != domain.WithheldNone {
		result.Outcome, result.Reason = domain.RubricWithheld, reason
		return result, nil
	}
	overall := weightedOverall(included, stored, weights, policy.MetricDecimals)
	result.Overall = &overall
	result.Outcome = domain.RubricComplete
	if partialSession || len(missing) > 0 {
		result.Outcome = domain.RubricPartial
	}
	return result, nil
}

func validatePolicy(policy domain.ScoringPolicy) error {
	if policy.MetricDecimals < 0 || policy.MetricDecimals > domain.MaxMetricDecimals {
		return fmt.Errorf("%w: metric decimals %d is outside 0..%d", ErrInvalidScoringPolicy, policy.MetricDecimals, domain.MaxMetricDecimals)
	}
	switch policy.MissingCoverage {
	case domain.MissingCoverageWithhold, domain.MissingCoverageRenormalize:
	default:
		return fmt.Errorf("%w: unknown missing coverage policy %q", ErrInvalidScoringPolicy, policy.MissingCoverage)
	}
	switch policy.PartialSession {
	case domain.PartialSessionScoreIfCovered, domain.PartialSessionWithhold:
	default:
		return fmt.Errorf("%w: unknown partial session policy %q", ErrInvalidScoringPolicy, policy.PartialSession)
	}
	return nil
}

// withheldReason decides whether the overall is given. Missing coverage comes first. Under renormalizing it
// only blocks the overall when no weighted metric is covered at all.
func withheldReason(policy domain.ScoringPolicy, hasMissing, nothingIncluded, partialSession bool) domain.RubricWithheldReason {
	if hasMissing && (policy.MissingCoverage == domain.MissingCoverageWithhold || nothingIncluded) {
		return domain.WithheldMissingCoverage
	}
	if partialSession && policy.PartialSession == domain.PartialSessionWithhold {
		return domain.WithheldPartialSession
	}
	return domain.WithheldNone
}

type weightEntry struct {
	exact *big.Rat // the decimal the caller wrote (33.33 is 3333/100, not its binary approximation)
	raw   float64
}

type weightSet struct {
	byMetric map[uuid.UUID]weightEntry
}

// parseWeights checks the posting weights: finite, not negative, each metric once, and a positive sum. They
// are relative (the overall divides by their sum), so they need not add up to 100.
func parseWeights(weights []domain.ScoredMetric) (weightSet, error) {
	if len(weights) == 0 {
		return weightSet{}, invalidScore("no metric weights")
	}
	set := weightSet{byMetric: make(map[uuid.UUID]weightEntry, len(weights))}
	sum := 0.0
	for _, weight := range weights {
		switch {
		case weight.MetricID == uuid.Nil:
			return weightSet{}, invalidScore("a weight has no metric id")
		case math.IsNaN(weight.Weight) || math.IsInf(weight.Weight, 0) || weight.Weight < 0:
			return weightSet{}, invalidScore("weight of metric %s is not a finite non-negative number", weight.MetricID)
		}
		if _, dup := set.byMetric[weight.MetricID]; dup {
			return weightSet{}, invalidScore("metric %s is weighted twice", weight.MetricID)
		}
		exact, ok := new(big.Rat).SetString(strconv.FormatFloat(weight.Weight, 'f', -1, 64))
		if !ok {
			return weightSet{}, invalidScore("weight of metric %s cannot be read", weight.MetricID)
		}
		set.byMetric[weight.MetricID] = weightEntry{exact: exact, raw: weight.Weight}
		sum += weight.Weight
	}
	if sum <= 0 {
		return weightSet{}, invalidScore("metric weights add up to zero")
	}
	return set, nil
}

// cover splits the metrics with a positive weight into those some scored question assessed (included in the
// overall) and those none did (missing). Both are in ascending id order.
func (w weightSet) cover(stored map[uuid.UUID]int64) (included, missing []uuid.UUID) {
	for _, id := range sortedIDs(w.byMetric) {
		if w.byMetric[id].exact.Sign() <= 0 {
			continue
		}
		if _, assessed := stored[id]; assessed {
			included = append(included, id)
		} else {
			missing = append(missing, id)
		}
	}
	return included, missing
}

// weightedOverall is round(sum stored(m) x weight(m) / sum weight(m)) over the included metrics, half-up to a
// whole number (interviews.score is an integer).
func weightedOverall(included []uuid.UUID, stored map[uuid.UUID]int64, weights weightSet, decimals int) int32 {
	numerator, denominator := new(big.Rat), new(big.Rat)
	for _, metric := range included {
		weight := weights.byMetric[metric].exact
		numerator.Add(numerator, new(big.Rat).Mul(new(big.Rat).SetInt64(stored[metric]), weight))
		denominator.Add(denominator, weight)
	}
	// stored values carry `decimals` implied decimals.
	denominator.Mul(denominator, new(big.Rat).SetInt(pow10(decimals)))
	return int32(roundHalfUp(new(big.Rat).Quo(numerator, denominator), 0))
}

type checkedQuestion struct {
	position int32
	criteria []domain.CriterionPoints
}

// validateQuestions refuses anything that cannot be scored and returns the questions in ascending position.
// A criterion is defined once per rubric, so the same criterion id must carry the same metric and maximum in
// every question that uses it.
func validateQuestions(in domain.RubricScoreInput) ([]checkedQuestion, error) {
	if len(in.Questions) == 0 {
		return nil, invalidScore("no scored questions")
	}
	if in.AssignedQuestions < 0 {
		return nil, invalidScore("assigned question count %d is negative", in.AssignedQuestions)
	}
	if in.AssignedQuestions > 0 && int(in.AssignedQuestions) < len(in.Questions) {
		return nil, invalidScore("%d questions are scored but only %d were assigned", len(in.Questions), in.AssignedQuestions)
	}
	type definition struct {
		metric uuid.UUID
		max    int32
	}
	definitions := make(map[uuid.UUID]definition)
	positions := make(map[int32]bool, len(in.Questions))
	questions := make([]checkedQuestion, 0, len(in.Questions))
	for _, q := range in.Questions {
		if positions[q.Position] {
			return nil, invalidScore("question position %d appears twice; follow-up evidence belongs to its core question", q.Position)
		}
		positions[q.Position] = true
		if len(q.Criteria) == 0 {
			return nil, invalidScore("question %d has no criteria", q.Position)
		}
		seen := make(map[uuid.UUID]bool, len(q.Criteria))
		for _, c := range q.Criteria {
			switch {
			case c.CriterionID == uuid.Nil || c.MetricID == uuid.Nil:
				return nil, invalidScore("question %d has a criterion without an id or a metric", q.Position)
			case c.MaxPoints <= 0:
				return nil, invalidScore("criterion %s has maximum %d, want more than 0", c.CriterionID, c.MaxPoints)
			case c.Points < 0 || c.Points > c.MaxPoints:
				return nil, invalidScore("criterion %s in question %d has %d points, want 0..%d", c.CriterionID, q.Position, c.Points, c.MaxPoints)
			case seen[c.CriterionID]:
				return nil, invalidScore("criterion %s is scored twice in question %d", c.CriterionID, q.Position)
			}
			seen[c.CriterionID] = true
			if known, ok := definitions[c.CriterionID]; ok && (known.metric != c.MetricID || known.max != c.MaxPoints) {
				return nil, invalidScore("criterion %s changes its metric or maximum between questions", c.CriterionID)
			}
			definitions[c.CriterionID] = definition{metric: c.MetricID, max: c.MaxPoints}
		}
		questions = append(questions, checkedQuestion{position: q.Position, criteria: q.Criteria})
	}
	slices.SortFunc(questions, func(a, b checkedQuestion) int { return int(a.position) - int(b.position) })
	return questions, nil
}

// scoreQuestion returns the display result of one question and, for each metric it assesses, the exact
// (unrounded) question metric score the session mean is built from.
func scoreQuestion(q checkedQuestion, decimals int, scale float64) (domain.RubricQuestionScore, map[uuid.UUID]*big.Rat) {
	type points struct{ earned, max int64 }
	byMetric := make(map[uuid.UUID]*points)
	var all points
	for _, c := range q.criteria {
		p := byMetric[c.MetricID]
		if p == nil {
			p = &points{}
			byMetric[c.MetricID] = p
		}
		p.earned += int64(c.Points)
		p.max += int64(c.MaxPoints)
		all.earned += int64(c.Points)
		all.max += int64(c.MaxPoints)
	}
	percent := func(p points) *big.Rat { return big.NewRat(100*p.earned, p.max) }
	scored := domain.RubricQuestionScore{
		Position: q.position,
		Earned:   int32(all.earned),
		Max:      int32(all.max),
		Total:    float64(roundHalfUp(percent(all), decimals)) / scale,
	}
	exact := make(map[uuid.UUID]*big.Rat, len(byMetric))
	for _, metric := range sortedIDs(byMetric) {
		p := byMetric[metric]
		exact[metric] = percent(*p)
		scored.Metrics = append(scored.Metrics, domain.RubricQuestionMetricScore{
			MetricID: metric,
			Earned:   int32(p.earned),
			Max:      int32(p.max),
			Score:    float64(roundHalfUp(exact[metric], decimals)) / scale,
		})
	}
	return scored, exact
}

// roundHalfUp returns r x 10^decimals rounded half-up to an integer. r is never negative here, so rounding
// half up and half away from zero agree.
func roundHalfUp(r *big.Rat, decimals int) int64 {
	scaled := new(big.Int).Mul(r.Num(), pow10(decimals))
	scaled.Lsh(scaled, 1)
	scaled.Add(scaled, r.Denom())
	scaled.Quo(scaled, new(big.Int).Lsh(r.Denom(), 1))
	return scaled.Int64()
}

func pow10(n int) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil)
}

func compareIDs(a, b uuid.UUID) int { return bytes.Compare(a[:], b[:]) }

func sortedIDs[V any](m map[uuid.UUID]V) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, compareIDs)
	return ids
}
