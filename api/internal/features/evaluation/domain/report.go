package domain

import (
	"time"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// Report is one interview's report as the viewer may see it. Summary, Breakdown and Questions are null
// for a viewer with the limited view (a recruitment Candidate). Summary and Breakdown are also null until
// the evaluation completed.
type Report struct {
	InterviewID  uuid.UUID  `json:"interview_id"`
	Type         string     `json:"type" enums:"practice,recruitment"`
	Status       string     `json:"status" enums:"in_progress,pending,completed,failed"`
	OverallScore *int32     `json:"overall_score"`
	StartedAt    time.Time  `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at"`
	// EndReason is how the interview ended: completed, left early by the Candidate, or abandoned after
	// the inactivity timeout. Null while it runs and for interviews ended before the reason was recorded.
	EndReason *string `json:"end_reason" enums:"completed,left,abandoned"`
	// QuestionCount is the number of core questions the session asked from. Questions lists only the
	// answered ones; a core question never answered has no entry.
	QuestionCount int32             `json:"question_count"`
	Summary       *string           `json:"summary"`
	Breakdown     []ReportBreakdown `json:"breakdown"`
	Questions     []ReportQuestion  `json:"questions"`
}

// ReportBreakdown is the score of one metric in the report.
type ReportBreakdown struct {
	MetricID uuid.UUID `json:"metric_id"`
	Name     string    `json:"name"`
	Score    int32     `json:"score"`
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

// ErrInterviewNotFound hides an interview from every viewer who may not see it, so its existence does not
// leak.
var ErrInterviewNotFound = apperror.New(apperror.CodeInterviewNotFound, "interview not found")
