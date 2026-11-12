package domain

import "github.com/google/uuid"

// Rubric scoring (KAN-332, RB-08). The model is: criterion points -> metric score inside one core question
// -> metric score of the session -> weighted overall. The calculator that applies it is pure: it reads these
// types and does no I/O. The evaluation service applies it to the validated criterion grades of a session.

// CriterionPoints is the outcome of one rubric criterion for one core question. MaxPoints is the criterion's
// maximum and Points the whole number the candidate earned, 0..MaxPoints.
type CriterionPoints struct {
	CriterionID uuid.UUID
	MetricID    uuid.UUID
	MaxPoints   int32
	Points      int32
}

// QuestionPoints holds every criterion result of one core question, identified by Position. Evidence from a
// follow-up belongs to its parent core question: the follow-up changes the points here, never adds a question
// or a criterion, so it cannot grow the denominator. Two entries with the same Position are therefore invalid.
type QuestionPoints struct {
	Position int32
	Criteria []CriterionPoints
}

// RubricScoreInput is what the calculator needs. Weights are the posting's metric weights (or the equal-split
// defaults the caller resolved); they are relative, so they need not add up to 100. AssignedQuestions is how
// many core questions the session was assigned; 0 means unknown and disables the partial-session check.
type RubricScoreInput struct {
	Questions         []QuestionPoints
	Weights           []ScoredMetric
	AssignedQuestions int32
}

// MissingCoveragePolicy says what happens when a metric with a positive weight was assessed by none of the
// scored questions. A question that does not assess a metric is never a zero for it.
type MissingCoveragePolicy string

const (
	// MissingCoverageWithhold gives no overall score and lists the uncovered metrics. The remaining weights
	// are not rescaled to hide the gap. Settled on 11/11 as the policy.
	MissingCoverageWithhold MissingCoveragePolicy = "withhold_overall"
	// MissingCoverageRenormalize scores over the covered metrics only and marks the result partial. It is not
	// the settled policy (11/11 chose withholding); it stays as a tested alternative.
	MissingCoverageRenormalize MissingCoveragePolicy = "renormalize_weights"
)

// PartialSessionPolicy says what happens when fewer core questions were scored than were assigned (an
// abandoned or left session, a skipped or failed question). Unscored questions are never counted as zero.
type PartialSessionPolicy string

const (
	// PartialSessionScoreIfCovered scores the questions that have evidence and marks the result partial, as
	// long as every weighted metric is still covered. Settled on 11/11 as the policy.
	PartialSessionScoreIfCovered PartialSessionPolicy = "score_if_covered"
	// PartialSessionWithhold gives no overall score for any session with unscored questions.
	PartialSessionWithhold PartialSessionPolicy = "withhold_overall"
)

// MaxMetricDecimals bounds ScoringPolicy.MetricDecimals. Rounding is always half-up (a value exactly
// halfway goes up; scores are never negative) and is not configurable.
const MaxMetricDecimals = 6

// ScoringPolicy groups the scoring decisions the spec left open. They were settled on 11/11 and
// DefaultScoringPolicy holds the settled values. Change a value here, not in the arithmetic.
type ScoringPolicy struct {
	// MetricDecimals is the precision of question scores, session metric scores and the overall (settled 11/11:
	// 2, stored in numeric(5,2) columns). The overall is calculated from the rounded session metrics and then
	// rounded half-up at the same precision.
	MetricDecimals  int
	MissingCoverage MissingCoveragePolicy
	PartialSession  PartialSessionPolicy
}

// DefaultScoringPolicy is the policy settled on 11/11: two decimals for metric scores and the overall, withhold the overall when a weighted metric has no evidence, score a partial session (marked
// partial) when every weighted metric is still covered.
func DefaultScoringPolicy() ScoringPolicy {
	return ScoringPolicy{
		MetricDecimals:  2,
		MissingCoverage: MissingCoverageWithhold,
		PartialSession:  PartialSessionScoreIfCovered,
	}
}

// RubricOutcome tells how far the overall can be trusted.
type RubricOutcome string

const (
	// RubricComplete: every assigned question is scored and every weighted metric is covered.
	RubricComplete RubricOutcome = "complete"
	// RubricPartial: an overall exists but rests on fewer questions or fewer metrics than planned.
	RubricPartial RubricOutcome = "partial"
	// RubricWithheld: no overall; Reason says why.
	RubricWithheld RubricOutcome = "withheld"
)

// RubricWithheldReason explains a withheld overall.
type RubricWithheldReason string

const (
	WithheldNone            RubricWithheldReason = ""
	WithheldMissingCoverage RubricWithheldReason = "missing_metric_coverage"
	WithheldPartialSession  RubricWithheldReason = "partial_session"
)

// RubricScore is the calculator's result. Questions are in ascending Position, metrics in ascending
// MetricID, so equal input gives an identical result.
type RubricScore struct {
	Outcome RubricOutcome
	Reason  RubricWithheldReason
	// Overall is nil when the outcome is withheld.
	Overall *float64
	// Questions carries the per-question detail. Total is for display only and is never averaged into the
	// overall.
	Questions []RubricQuestionScore
	// Metrics are the session metric scores, one per assessed metric. A metric without a posting weight is
	// listed with Weight 0 and does not count towards the overall.
	Metrics []RubricMetricScore
	// MissingMetrics are weighted metrics no scored question assessed.
	MissingMetrics    []uuid.UUID
	ScoredQuestions   int32
	AssignedQuestions int32
}

// RubricQuestionScore is one core question. Total is 100 x Earned / Max over all its criteria.
type RubricQuestionScore struct {
	Position int32
	Earned   int32
	Max      int32
	Total    float64
	Metrics  []RubricQuestionMetricScore
}

// RubricQuestionMetricScore is the score of one metric inside one question: 100 x earned / max over the
// question's criteria of that metric.
type RubricQuestionMetricScore struct {
	MetricID uuid.UUID
	Earned   int32
	Max      int32
	Score    float64
}

// RubricMetricScore is the mean of a metric over the scored questions that assess it. Questions is how many
// did; Weight is the posting weight as given (0 when the metric is not weighted).
type RubricMetricScore struct {
	MetricID  uuid.UUID
	Score     float64
	Weight    float64
	Questions int32
}
