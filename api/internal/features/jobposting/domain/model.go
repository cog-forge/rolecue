package domain

import (
	"time"

	"github.com/google/uuid"
)

// Status mirrors the frozen job_posting_status enum. A posting is created pending; Admin approval
// opens intake.
type Status string

const (
	StatusPending      Status = "pending"
	StatusOpen         Status = "open"
	StatusIntakeClosed Status = "intake_closed"
	StatusClosed       Status = "closed"
	StatusRejected     Status = "rejected"
)

func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusOpen, StatusIntakeClosed, StatusClosed, StatusRejected:
		return true
	}
	return false
}

// Level is a levels row name. It describes the job, not the interview difficulty.
type Level string

const (
	Intern Level = "intern"
	Junior Level = "junior"
	Mid    Level = "mid"
	Senior Level = "senior"
	Lead   Level = "lead"
)

func (s Level) Valid() bool {
	switch s {
	case Intern, Junior, Mid, Senior, Lead:
		return true
	}
	return false
}

// RequirementType mirrors the frozen requirement_type enum.
type RequirementType string

const (
	Required  RequirementType = "required"
	Preferred RequirementType = "preferred"
)

// Requirement targets are the same ones the Candidate JD flow stores: a skill category, or one of
// the two plain-name lists.
var requirementTargets = map[string]bool{
	"programming_language": true, "framework": true, "database": true, "tool": true, "technology": true,
	"other": true, "technologies": true, "domain_knowledge": true,
}

func ValidTarget(target string) bool { return requirementTargets[target] }

// PostingRequirement is one requirement the Recruiter confirmed in review.
type PostingRequirement struct {
	Name   string          `json:"name"`
	Target string          `json:"target"`
	Type   RequirementType `json:"type"`
}

// Ref is a referenced row shown by name, e.g. the required voice.
type Ref struct {
	ID   uuid.UUID `json:"id"`
	Name *string   `json:"name"`
}

// JobPosting is the owning Recruiter's full view of a posting.
type JobPosting struct {
	ID                uuid.UUID            `json:"id"`
	Title             string               `json:"title"`
	Status            Status               `json:"status"`
	RejectReason      *string              `json:"reject_reason"`
	RawText           string               `json:"raw_text"`
	SeniorityLevel    Level                `json:"seniority_level"`
	Requirements      []PostingRequirement `json:"requirements"`
	Location          *string              `json:"location"`
	EmploymentType    *string              `json:"employment_type"`
	SalaryMin         *int32               `json:"salary_min"`
	SalaryMax         *int32               `json:"salary_max"`
	InterviewDuration *int32               `json:"interview_duration"`
	InterviewSlot     *int32               `json:"interview_slot"`
	RequiredAvatar    *Ref                 `json:"required_avatar"`
	RequiredVoice     *Ref                 `json:"required_voice"`
	// CoreQuestionCount shows whether the question bank exists; requirements lock once it does.
	CoreQuestionCount int64     `json:"core_question_count"`
	CreatedAt         time.Time `json:"created_at"`
}

// Summary is one row of the Recruiter's own posting list.
type Summary struct {
	ID             uuid.UUID `json:"id"`
	Title          string    `json:"title"`
	Status         Status    `json:"status"`
	RejectReason   *string   `json:"reject_reason"`
	Location       *string   `json:"location"`
	EmploymentType *string   `json:"employment_type"`
	InterviewSlot  *int32    `json:"interview_slot"`
	CreatedAt      time.Time `json:"created_at"`
}

// Record is what the repository stores and returns. ContentPath is the storage key of the original
// JD text; the service turns it back into RawText.
type Record struct {
	JobPosting
	ContentPath string
}

// CreateInput holds only client-controlled fields; the owner comes from the session.
type CreateInput struct {
	Title             string
	RawText           string
	SeniorityLevel    Level
	Requirements      []PostingRequirement
	Location          *string
	EmploymentType    *string
	SalaryMin         *int32
	SalaryMax         *int32
	InterviewDuration *int32
	RequiredAvatarID  uuid.UUID
	RequiredVoiceID   uuid.UUID
}

// NewPosting is a validated CreateInput plus the storage key of its text.
type NewPosting struct {
	CreateInput
	ContentPath string
}

// Suggestion is what AI extraction read from a JD, for the Recruiter to review and confirm before
// creating the posting. Nothing is stored. SeniorityLevel is nil when the JD does not say.
type Suggestion struct {
	Title          string               `json:"title"`
	SeniorityLevel *Level               `json:"seniority_level"`
	Requirements   []PostingRequirement `json:"requirements"`
}

// ListFilter narrows the Recruiter's list. A nil field means "no filter".
type ListFilter struct {
	Status *Status
	Search *string
	Limit  int32
	Offset int32
}
