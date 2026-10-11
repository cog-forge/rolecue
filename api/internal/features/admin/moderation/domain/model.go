package domain

import (
	"time"

	"github.com/google/uuid"
)

// PostingStatus mirrors the frozen job_posting_status enum. Admin approval moves pending to open.
type PostingStatus string

const (
	PostingStatusPending      PostingStatus = "pending"
	PostingStatusOpen         PostingStatus = "open"
	PostingStatusIntakeClosed PostingStatus = "intake_closed"
	PostingStatusClosed       PostingStatus = "closed"
	PostingStatusRejected     PostingStatus = "rejected"
)

func (s PostingStatus) Valid() bool {
	switch s {
	case PostingStatusPending, PostingStatusOpen, PostingStatusIntakeClosed, PostingStatusClosed, PostingStatusRejected:
		return true
	}
	return false
}

const MaxRejectReasonRunes = 1000

type Recruiter struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Email string    `json:"email"`
}

// NamedRef is a referenced row shown by name only, e.g. the posting's required voice.
type NamedRef struct {
	ID   uuid.UUID `json:"id"`
	Name *string   `json:"name"`
}

// PostingSummary is one row of the moderation queue.
type PostingSummary struct {
	ID        uuid.UUID     `json:"id"`
	Title     string        `json:"title"`
	Status    PostingStatus `json:"status"`
	Recruiter Recruiter     `json:"recruiter"`
	CreatedAt time.Time     `json:"created_at"`
}

// ReviewPosting is the read-only review view; the Admin can only decide, never edit it. The Admin
// moderates the JD content candidates will read, reached through ContentPath.
type ReviewPosting struct {
	ID                uuid.UUID     `json:"id"`
	Title             string        `json:"title"`
	Status            PostingStatus `json:"status"`
	RejectReason      *string       `json:"reject_reason"`
	Recruiter         Recruiter     `json:"recruiter"`
	Location          *string       `json:"location"`
	EmploymentType    *string       `json:"employment_type"`
	SalaryMin         *int32        `json:"salary_min"`
	SalaryMax         *int32        `json:"salary_max"`
	InterviewSlot     *int32        `json:"interview_slot"`
	InterviewDuration *int32        `json:"interview_duration"`
	RequiredAvatar    *NamedRef     `json:"required_avatar"`
	RequiredVoice     *NamedRef     `json:"required_voice"`
	CreatedAt         time.Time     `json:"created_at"`
	// ContentPath is the URL of the JD content (job_descriptions.content_path), the text the Admin reviews.
	ContentPath string `json:"content_path"`
	// CoreQuestionCount tells the Admin whether the posting can be approved (it needs at least one).
	CoreQuestionCount int64 `json:"core_question_count"`
}

// ListFilter narrows the queue. A nil field means "no filter"; the date window is [From, To).
type ListFilter struct {
	Status      *PostingStatus
	RecruiterID *uuid.UUID
	From        *time.Time
	To          *time.Time
	Limit       int32
	Offset      int32
}
