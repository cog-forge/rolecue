package domain

import (
	"time"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// ApplicationStatus is the single Application lifecycle field (applications.status).
type ApplicationStatus string

const (
	StatusPending           ApplicationStatus = "pending"
	StatusInterviewEligible ApplicationStatus = "interview_eligible"
	StatusInterviewed       ApplicationStatus = "interviewed"
	StatusApproved          ApplicationStatus = "approved"
	StatusRejected          ApplicationStatus = "rejected"
)

// InterviewWindow is how long a CV-approved Candidate has to complete the recruitment interview.
// The deadline itself is computed by the database clock (see ApproveCV in the queries).
const InterviewWindow = 24 * time.Hour

// MaxRejectReasonLength bounds the optional note a Recruiter leaves when rejecting a CV.
const MaxRejectReasonLength = 500

// IntakeClosedRejectReason is stored on every pending Application that intake closure rejects (KTD4).
// Clients display it and never match on it.
const IntakeClosedRejectReason = "Intake closed before this CV was screened."

// IntakeClosure is the posting after a Close Intake call and how many pending Applications it rejected.
type IntakeClosure struct {
	JobPostingID  uuid.UUID `json:"job_posting_id"`
	Status        string    `json:"status"`
	RejectedCount int64     `json:"rejected_count"`
}

// Screening is an Application right after a CV screening decision.
type Screening struct {
	ID           uuid.UUID         `json:"id"`
	JobPostingID uuid.UUID         `json:"job_posting_id"`
	Status       ApplicationStatus `json:"status"`
	// JobPostingStatus is the posting status after the decision. An approval that takes the last
	// interview slot makes it intake_closed in the same transaction (R9).
	JobPostingStatus string `json:"job_posting_status"`
	// InterviewDeadline is set on CV approval: approval time + 24 hours.
	InterviewDeadline *time.Time `json:"interview_deadline"`
	RejectReason      *string    `json:"reject_reason"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// ListFilter narrows the Recruiter's Application list. Every filter is optional.
type ListFilter struct {
	// JobPostingID limits the list to one posting. The Recruiter must own it, or the list is not found.
	JobPostingID *uuid.UUID
	Status       *ApplicationStatus
	// Search is an escaped LIKE pattern over the Candidate's name and email.
	Search *string
	Limit  int32
	Offset int32
}

// ListCandidate is the Candidate as the Application list shows them.
type ListCandidate struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Email string    `json:"email"`
	Image *string   `json:"image"`
}

// ListItem is one row of the Recruiter's Application list: what Application Detail needs to be
// opened, and nothing from the CV, transcript or recording.
type ListItem struct {
	ID              uuid.UUID         `json:"id"`
	JobPostingID    uuid.UUID         `json:"job_posting_id"`
	JobPostingTitle string            `json:"job_posting_title"`
	Status          ApplicationStatus `json:"status"`
	SubmittedAt     time.Time         `json:"submitted_at"`
	Candidate       ListCandidate     `json:"candidate"`
	// InterviewEligible is true while the Candidate is CV-approved and the 24-hour deadline has not passed.
	InterviewEligible bool       `json:"interview_eligible"`
	InterviewDeadline *time.Time `json:"interview_deadline"`
	RejectReason      *string    `json:"reject_reason"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// DetailRecord is one Application as the Application Detail read finds it. CVPath is the storage
// key: the service signs it and it is never part of a response.
type DetailRecord struct {
	Item          ListItem
	CVPath        *string
	PostingStatus string
	InterviewSlot *int32
	CVApprovals   int64
	Interview     *InterviewSummary
	// OpensAt is the posting's latest interview deadline (FinalReview.OpensAt), nil when none.
	OpensAt *time.Time
}

// DetailJobPosting is the posting block of Application Detail: the capacity the approval is checked
// against and the count that uses it.
type DetailJobPosting struct {
	ID            uuid.UUID `json:"id"`
	Title         string    `json:"title"`
	Status        string    `json:"status"`
	InterviewSlot *int32    `json:"interview_slot"`
	CVApprovals   int64     `json:"cv_approvals"`
}

// DetailCV is a short-lived link to the Application's CV. It is null when no CV is stored.
type DetailCV struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Detail is one Application for the Recruiter's Application Detail screen. It holds the list row,
// the posting block and the CV link, and nothing from the transcript, recording or evaluation.
type Detail struct {
	ListItem
	JobPosting DetailJobPosting `json:"job_posting"`
	CV         *DetailCV        `json:"cv"`
	// Interview is the Application's interview result, or null before one starts (KAN-114).
	Interview   *InterviewSummary `json:"interview"`
	FinalReview FinalReview       `json:"final_review"`
}

// ValidStatus reports whether value is an Application status.
func ValidStatus(value ApplicationStatus) bool {
	switch value {
	case StatusPending, StatusInterviewEligible, StatusInterviewed, StatusApproved, StatusRejected:
		return true
	}
	return false
}

// InterviewCapacity is the posting state a CV approval is checked against, read under the posting
// row lock.
type InterviewCapacity struct {
	PostingStatus string
	// InterviewSlot is nil until the Recruiter funds interview capacity (KAN-116).
	InterviewSlot *int32
	// CVApprovals counts every Application ever CV-approved on the posting, including ones later
	// rejected or decided: a Candidate who misses the deadline does not free a slot (decided
	// 2026-10-18).
	CVApprovals int64
}

// CheckApproval says whether one more Candidate may be approved for interview.
func (c InterviewCapacity) CheckApproval() error {
	if c.PostingStatus != "open" {
		return ErrIntakeNotOpen
	}
	if c.InterviewSlot == nil {
		return ErrCapacityNotFunded
	}
	if c.CVApprovals >= int64(*c.InterviewSlot) {
		return ErrCapacityFull
	}
	return nil
}

// Eligibility is what a recruitment interview start (KAN-50/KAN-117) needs from the Application.
type Eligibility struct {
	ApplicationID     uuid.UUID
	JobPostingID      uuid.UUID
	InterviewDeadline time.Time
}

// EligibilityRecord is the Candidate's Application as stored; WithinDeadline is computed by the
// database clock so it agrees with the deadline written on approval.
type EligibilityRecord struct {
	ApplicationID     uuid.UUID
	JobPostingID      uuid.UUID
	Status            ApplicationStatus
	InterviewDeadline *time.Time
	WithinDeadline    bool
}

var (
	ErrApplicationNotFound    = apperror.New(apperror.CodeApplicationNotFound, "application not found")
	ErrApplicationNotPending  = apperror.New(apperror.CodeApplicationNotPending, "application was already screened")
	ErrApplicationNotEligible = apperror.New(apperror.CodeApplicationNotEligible,
		"application is not eligible for a recruitment interview")
	ErrIntakeNotOpen     = apperror.New(apperror.CodeIntakeNotOpen, "job posting is not accepting CV approvals")
	ErrCapacityNotFunded = apperror.New(apperror.CodeInterviewCapacityNotFunded, "job posting has no funded interview capacity")
	ErrCapacityFull      = apperror.New(apperror.CodeInterviewCapacityFull, "all interview slots of this job posting are taken")
)
