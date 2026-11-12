package domain

import (
	"time"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// Report is one interview's report as the viewer may see it. Summary, Scoring, Breakdown, Questions and
// GradedQuestions are null for a viewer with the limited view (a recruitment Candidate). Summary, Scoring,
// Breakdown and GradedQuestions are also null until the evaluation completed.
type Report struct {
	InterviewID uuid.UUID `json:"interview_id"`
	Type        string    `json:"type" enums:"practice,recruitment"`
	Status      string    `json:"status" enums:"in_progress,pending,completed,failed"`
	// OverallScore is the weighted overall with two decimals (KAN-335). It is left out of the limited view (a
	// recruitment Candidate), whose result is the Recruiter's to announce, and it is null until the evaluation
	// completed and when the overall is withheld (see Scoring).
	OverallScore *float64  `json:"overall_score,omitempty"`
	StartedAt    time.Time `json:"started_at"`
	// CompletedAt is when the interview ended. Null while it runs.
	CompletedAt *time.Time `json:"completed_at"`
	// EndReason is how the interview ended: completed, left early by the Candidate, or abandoned after
	// the inactivity timeout. Null while it runs and for interviews ended before the reason was recorded.
	EndReason *string `json:"end_reason" enums:"completed,left,abandoned"`
	// QuestionCount is the number of core questions the session was assigned. Questions lists only the
	// answered turns; GradedQuestions lists every assigned core question.
	QuestionCount int32   `json:"question_count"`
	Summary       *string `json:"summary"`
	// SummarySource says who wrote the summary. It is always "ai": the text is model output, not a
	// Recruiter's judgement. Omitted in the limited view.
	SummarySource string `json:"summary_source,omitempty" enums:"ai"`
	// InjectionFlagged is true when the evaluation suspected the Candidate's answers tried to steer it; the
	// Recruiter should review the answers. Omitted in the limited view.
	InjectionFlagged *bool `json:"injection_flagged,omitempty"`
	// Scoring explains how the overall was reached. Null in the limited view and until completed.
	Scoring *ReportScoring `json:"scoring"`
	// Breakdown is one entry per session metric: every weighted metric (with a null score when no graded
	// question assessed it) and every assessed metric. Null in the limited view and until completed.
	Breakdown []ReportBreakdown `json:"breakdown"`
	// Questions are the stored conversation turns with the Candidate's answers and the feedback.
	Questions []ReportQuestion `json:"questions"`
	// GradedQuestions are the assigned core questions with their criterion results. Null in the limited view
	// and until completed.
	GradedQuestions []ReportGradedQuestion `json:"graded_questions"`
}

// Scoring methods. rubric: graded criterion by criterion (KAN-334). legacy: an evaluation completed before
// rubric grading, which has metric scores and an overall but no criterion results.
const (
	ScoringMethodRubric = "rubric"
	ScoringMethodLegacy = "legacy"
)

// ReportScoring is how the overall of a completed evaluation was calculated: criterion points -> question
// metric (100 x earned / max over the question's criteria of that metric) -> session metric (the mean over the
// graded questions that assess it) -> overall (the weighted mean of the session metrics). Question totals are
// for reading only and are never averaged into the overall.
type ReportScoring struct {
	Method string `json:"method" enums:"rubric,legacy"`
	// Outcome is complete (every assigned question graded, every weighted metric covered), partial (an
	// overall resting on fewer questions than assigned) or withheld (no overall; see WithheldReason). Null
	// for the legacy method.
	Outcome *string `json:"outcome" enums:"complete,partial,withheld"`
	// WithheldReason: missing_metric_coverage (a weighted metric no graded question assessed; it is never
	// counted as 0 and the other weights are not rescaled), no_answers, no_gradable_questions, or
	// partial_session (reserved by the policy, not used by the settled policy).
	WithheldReason *string `json:"withheld_reason" enums:"missing_metric_coverage,partial_session,no_answers,no_gradable_questions"`
	// AssignedQuestions is the number of core questions of the session; GradedQuestions how many were graded.
	AssignedQuestions int32 `json:"assigned_questions"`
	GradedQuestions   int32 `json:"graded_questions"`
	// WeightTotal is the sum of the weights the overall divides by (weights are relative).
	WeightTotal float64 `json:"weight_total"`
	// Decimals is the precision of every score (2).
	Decimals int `json:"decimals"`
}

// ReportBreakdown is one session metric of the report.
type ReportBreakdown struct {
	MetricID uuid.UUID `json:"metric_id"`
	Name     string    `json:"name"`
	// Score is the session metric with two decimals; null for a weighted metric no graded question assessed.
	Score *float64 `json:"score"`
	// Weight is the weight the overall used; 0 for an assessed metric the overall does not weigh. Null for
	// the legacy method.
	Weight *float64 `json:"weight"`
	// QuestionCount is how many graded questions assessed the metric. Null for the legacy method.
	QuestionCount *int32 `json:"question_count"`
	// Evidence is the legacy verbatim quote of a metric score; always null for rubric grading, whose evidence
	// is per criterion.
	Evidence *string `json:"evidence"`
}

// Graded question statuses.
const (
	QuestionGraded      = "graded"
	QuestionNotAnswered = "not_answered"
	QuestionNotGradable = "not_gradable"
)

// ReportGradedQuestion is one assigned core question. QuestionNumber matches ReportQuestion.QuestionNumber.
type ReportGradedQuestion struct {
	QuestionNumber int32  `json:"question_number"`
	Status         string `json:"status" enums:"graded,not_answered,not_gradable"`
	// RubricName is the rubric the question was graded with. Null when it has none, and for a Candidate when
	// the rubric is not a system template.
	RubricName *string `json:"rubric_name"`
	// Total is 100 x earned / max over all criteria of the question, for reading only. Null unless graded.
	Total  *float64 `json:"total"`
	Earned int32    `json:"earned"`
	Max    int32    `json:"max"`
	// Metrics are the question metric scores: 100 x earned / max over the question's criteria of the metric.
	Metrics  []ReportQuestionMetric `json:"metrics"`
	Criteria []ReportCriterion      `json:"criteria"`
}

// ReportQuestionMetric is one metric inside one graded question.
type ReportQuestionMetric struct {
	MetricID uuid.UUID `json:"metric_id"`
	Name     string    `json:"name"`
	Earned   int32     `json:"earned"`
	Max      int32     `json:"max"`
	Score    float64   `json:"score"`
}

// ReportCriterion is one graded criterion: the points over its maximum, the quote it rests on and why.
type ReportCriterion struct {
	CriterionID uuid.UUID `json:"criterion_id"`
	// Name is the criterion as the rubric states it. Empty for a Candidate when the rubric is not a system
	// template.
	Name       string    `json:"name"`
	MetricID   uuid.UUID `json:"metric_id"`
	MetricName string    `json:"metric_name"`
	Points     int32     `json:"points"`
	MaxPoints  int32     `json:"max_points"`
	// Evidence is a verbatim quote from an answer to this question, checked against the stored answers;
	// EvidencePosition is the conversation position of that answer. Both null for 0 points with nothing to
	// quote.
	Evidence         *string `json:"evidence"`
	EvidencePosition *int32  `json:"evidence_position"`
	Feedback         *string `json:"feedback"`
}

// ReportQuestion is one question of the interview with the Candidate's answer and its feedback. Position is
// its place in the conversation; QuestionNumber is the core question it belongs to, so a follow-up
// (kind follow_up) is shown under its core question.
type ReportQuestion struct {
	Position       int32   `json:"position"`
	QuestionNumber int32   `json:"question_number"`
	Kind           string  `json:"kind" enums:"core,follow_up"`
	Question       string  `json:"question"`
	Answer         *string `json:"answer"`
	Feedback       *string `json:"feedback"`
}

// StoredResults is what a completed evaluation stored, as the report reads it.
type StoredResults struct {
	Metrics   []StoredMetric
	Questions []StoredQuestion
	Criteria  []StoredCriterion
}

// StoredMetric is one score_details row. Weight and Questions are nil for a legacy evaluation.
type StoredMetric struct {
	MetricID  uuid.UUID
	Name      string
	Score     *float64
	Weight    *float64
	Questions *int32
	Evidence  *string
}

// StoredQuestion is one assigned core question with its stored total and its rubric (if any).
type StoredQuestion struct {
	Position     int32
	Score        *float64
	HasRubric    bool
	RubricName   *string
	SystemRubric bool
}

// StoredCriterion is one interview_criterion_scores row with its criterion.
type StoredCriterion struct {
	QuestionPosition int32
	CriterionID      uuid.UUID
	Name             string
	MetricID         uuid.UUID
	MetricName       string
	MaxPoints        int32
	Points           int32
	Evidence         *string
	EvidencePosition *int32
	Feedback         *string
}

// SummarySourceAI marks a summary written by the model.
const SummarySourceAI = "ai"

// ErrInterviewNotFound hides an interview from every viewer who may not see it, so its existence does not
// leak.
var ErrInterviewNotFound = apperror.New(apperror.CodeInterviewNotFound, "interview not found")
