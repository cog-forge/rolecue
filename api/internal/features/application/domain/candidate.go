package domain

import (
	"time"

	"github.com/google/uuid"
)

// CandidateApplication is one of the Candidate's own Applications. It carries the status, the
// rejection reason and the interview deadline, and no transcript, audio, video or CV storage key.
type CandidateApplication struct {
	ID              uuid.UUID `json:"id"`
	JobPostingID    uuid.UUID `json:"job_posting_id"`
	JobPostingTitle string    `json:"job_posting_title"`
	// CompanyName and JobPostingLocation come from the posting and its Recruiter, for the list card.
	CompanyName        *string           `json:"company_name"`
	JobPostingLocation *string           `json:"job_posting_location"`
	Status             ApplicationStatus `json:"status"`
	RejectReason       *string           `json:"reject_reason"`
	// InterviewDeadline is set on CV approval: approval time + 24 hours.
	InterviewDeadline *time.Time `json:"interview_deadline"`
	// CanStartInterview is true only while the Application is interview_eligible and the deadline
	// has not passed. Start itself is checked again on the server when it exists.
	CanStartInterview bool `json:"can_start_interview"`
	// Interview is the score and state of this Application's interview, or null (KAN-114).
	Interview *CandidateInterview `json:"interview"`
	CreatedAt time.Time           `json:"created_at"`
}

// CanStartInterview reports whether the Candidate may start the recruitment interview now.
func CanStartInterview(status ApplicationStatus, deadline *time.Time, now time.Time) bool {
	return status == StatusInterviewEligible && deadline != nil && now.Before(*deadline)
}

// CandidateListFilter is one page of the Candidate's Applications. Page starts at 1.
type CandidateListFilter struct {
	Page     int32
	PageSize int32
}

// CandidatePage is one page of the Candidate's Applications.
type CandidatePage struct {
	Items    []CandidateApplication `json:"items"`
	Total    int64                  `json:"total"`
	Page     int32                  `json:"page"`
	PageSize int32                  `json:"page_size"`
}
