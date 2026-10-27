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

// InterviewRecord is one interview with the owners the report visibility rules need. PostingID and
// RecruiterID are set for recruitment sessions only.
type InterviewRecord struct {
	ID               uuid.UUID
	Type             string
	StartedAt        time.Time
	CompletedAt      *time.Time
	EvaluationStatus string
	Score            *int32
	Feedback         *string
	CandidateID      uuid.UUID
	PostingID        *uuid.UUID
	RecruiterID      *uuid.UUID
}

// TurnRecord is one stored question of an interview with its answer and the feedback once evaluated.
type TurnRecord struct {
	Position int32
	Question string
	Answer   *string
	Feedback *string
}

// ScoreInput is what the Scorer sees: the answered questions of the session and the metrics to score,
// each with its weight. Turns are ordered by position.
type ScoreInput struct {
	InterviewID uuid.UUID
	Type        string
	Turns       []ScoreTurn
	Metrics     []ScoredMetric
}

// ScoreTurn is one answered question as it was asked and answered.
type ScoreTurn struct {
	Position int32
	Question string
	Answer   string
}

// ScoredMetric is one metric the answers are graded on. Weight is relative: the overall score divides
// by the sum of the weights, so the weights need not add up to 100.
type ScoredMetric struct {
	MetricID uuid.UUID
	Name     string
	Weight   float64
}

// ScoreOutput is what the Scorer returns: one score per metric, a summary and optional feedback per
// question. The evaluation service checks it before anything is stored.
type ScoreOutput struct {
	Scores    []MetricScore
	Summary   string
	Questions []QuestionFeedback
}

// MetricScore is the 0..100 score of one metric.
type MetricScore struct {
	MetricID uuid.UUID
	Score    int32
}

// QuestionFeedback is the feedback for the answer at one position.
type QuestionFeedback struct {
	Position int32
	Feedback string
}

// Evaluation is the validated result of a scored interview, written in one transaction. Summary is nil
// when the Scorer gave none.
type Evaluation struct {
	OverallScore int32
	Summary      *string
	Scores       []MetricScore
	Questions    []QuestionFeedback
}
