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
	InterviewID  uuid.UUID         `json:"interview_id"`
	Type         string            `json:"type" enums:"practice,recruitment"`
	Status       string            `json:"status" enums:"in_progress,pending,completed,failed"`
	OverallScore *int32            `json:"overall_score"`
	StartedAt    time.Time         `json:"started_at"`
	CompletedAt  *time.Time        `json:"completed_at"`
	Summary      *string           `json:"summary"`
	Breakdown    []ReportBreakdown `json:"breakdown"`
	Questions    []ReportQuestion  `json:"questions"`
}

// ReportBreakdown is the score of one metric in the report.
type ReportBreakdown struct {
	MetricID uuid.UUID `json:"metric_id"`
	Name     string    `json:"name"`
	Score    int32     `json:"score"`
}

// ReportQuestion is one question of the interview with the Candidate's answer and its feedback.
type ReportQuestion struct {
	Position int32   `json:"position"`
	Question string  `json:"question"`
	Answer   *string `json:"answer"`
	Feedback *string `json:"feedback"`
}

// ErrInterviewNotFound hides an interview from every viewer who may not see it, so its existence does not
// leak.
var ErrInterviewNotFound = apperror.New(apperror.CodeInterviewNotFound, "interview not found")
