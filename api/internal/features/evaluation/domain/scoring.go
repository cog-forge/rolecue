package domain

import (
	"time"

	"github.com/google/uuid"
)

// Interview types and evaluation statuses as the evaluation reads them. They match the interview_type
// enum and interviews.evaluation_status in the schema.
const (
	InterviewTypePractice    = "practice"
	InterviewTypeRecruitment = "recruitment"

	EvaluationPending   = "pending"
	EvaluationCompleted = "completed"
	EvaluationFailed    = "failed"
)

// Withheld reasons the evaluation itself decides before the calculator runs. The calculator adds
// WithheldMissingCoverage and WithheldPartialSession (rubric_score.go). Every reason is stored in
// interviews.score_withheld_reason and is final: a retry would reach the same result, so the evaluation
// completes instead of failing.
const (
	// WithheldNoAnswers: the session ended before any question was answered.
	WithheldNoAnswers RubricWithheldReason = "no_answers"
	// WithheldNoGradableQuestions: answers exist, but none of the answered questions has a rubric with
	// criteria and an expected answer (a legacy or hand-added question).
	WithheldNoGradableQuestions RubricWithheldReason = "no_gradable_questions"
)

// InterviewRecord is one interview with the owners the report visibility rules need. PostingID and
// RecruiterID are set for recruitment sessions only.
type InterviewRecord struct {
	ID          uuid.UUID
	Type        string
	StartedAt   time.Time
	CompletedAt *time.Time
	EndReason   *string
	// QuestionCount is the number of core questions assigned to the session.
	QuestionCount    int32
	EvaluationStatus string
	// Score is the weighted overall with two decimals; nil until completed and when withheld.
	Score *float64
	// ScoreOutcome is complete, partial or withheld once a rubric evaluation completed; nil before, and for an
	// evaluation completed before rubric grading existed.
	ScoreOutcome        *string
	ScoreWithheldReason *string
	Feedback            *string
	// InjectionFlagged is set when the evaluation could not fully trust the Candidate's answers (KAN-253).
	InjectionFlagged bool
	CandidateID      uuid.UUID
	PostingID        *uuid.UUID
	RecruiterID      *uuid.UUID
}

// Turn kinds: a core question of the session, or a bounded follow-up the live interviewer asked about one
// (KAN-56).
const (
	TurnCore     = "core"
	TurnFollowUp = "follow_up"
)

// TurnRecord is one stored question of an interview with its answer and the feedback once evaluated.
// Position is its place in the conversation; QuestionNumber is the core question it belongs to, which a
// follow-up shares with its core question. QuestionNumber equals the assigned question's position
// (interview_questions.position).
type TurnRecord struct {
	Position       int32
	QuestionNumber int32
	Kind           string
	Question       string
	Answer         *string
	Feedback       *string
}

// GradingQuestion is one core question assigned to a session with its grading configuration. RubricID is
// nil for a question without a rubric; Criteria are in rubric order. ExpectedAnswer is grader input only.
type GradingQuestion struct {
	Position          int32
	RubricID          *uuid.UUID
	RubricName        string
	RubricDescription string
	ExpectedAnswer    string
	Criteria          []GradingCriterion
}

// GradingCriterion is one criterion of a rubric as the evaluation loads it.
type GradingCriterion struct {
	ID           uuid.UUID
	MetricID     uuid.UUID
	MetricName   string
	Name         string
	MaxPoints    int32
	ScoringGuide string
}

// ScoredMetric is one metric the overall is weighted over. Weight is relative: the overall divides by the
// sum of the weights, so the weights need not add up to 100.
type ScoredMetric struct {
	MetricID uuid.UUID
	Name     string
	Weight   float64
}

// TurnFeedback is the feedback stored on one conversation turn (the core turn of a graded question).
type TurnFeedback struct {
	Position int32
	Feedback string
}

// QuestionResult is one graded core question: its total (display only, never averaged into the overall)
// and its criterion results.
type QuestionResult struct {
	Position int32
	Total    float64
	Criteria []CriterionGrade
}

// MetricResult is one session metric as it is stored: Score is nil for a weighted metric no graded question
// assessed; Weight is the weight the overall used (0 for an assessed metric the overall does not weigh).
type MetricResult struct {
	MetricID  uuid.UUID
	Score     *float64
	Weight    float64
	Questions int32
}

// Evaluation is the validated result of a graded interview, written in one transaction. Overall is nil when
// the outcome is withheld; Summary is nil when no question produced one.
type Evaluation struct {
	Outcome      RubricOutcome
	Reason       RubricWithheldReason
	Overall      *float64
	Summary      *string
	Questions    []QuestionResult
	Metrics      []MetricResult
	TurnFeedback []TurnFeedback
	// InjectionFlagged is true when the model suspected an injection, an answer or a follow-up matched an
	// injection pattern, or model text echoed an instruction. The evaluation still completes.
	InjectionFlagged bool
}

// CoreResultFacts are the stored facts about one assigned core question of a session, for the Learning Roadmap
// adapter (KAN-349). They carry no question text, expected answer or rubric: only whether the question was
// delivered to the Candidate, whether it was answered, and the Barem question score when it was graded.
type CoreResultFacts struct {
	Position       int32
	CoreQuestionID uuid.UUID
	// Delivered is true when the live engine delivered the question (interview_questions.asked_at is set).
	Delivered bool
	// Answered is true when the question has a non-empty stored answer; follow-ups belong to their core.
	Answered bool
	// Score is interview_questions.score, nil for a question that was not graded.
	Score *float64
}
