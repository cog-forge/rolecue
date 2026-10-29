package domain

import (
	"time"

	"github.com/google/uuid"
)

// BoardPosting is one open Job Posting on the Candidate job board. It deliberately has no field for
// the question bank, evaluation weights, interview capacity or funding, the reject reason or the
// required avatar/voice: Candidates never see them.
type BoardPosting struct {
	ID             uuid.UUID  `json:"id"`
	Title          string     `json:"title"`
	CompanyName    *string    `json:"company_name"`
	Location       *string    `json:"location"`
	EmploymentType *string    `json:"employment_type"`
	SalaryMin      *int32     `json:"salary_min"`
	SalaryMax      *int32     `json:"salary_max"`
	SeniorityLevel string     `json:"seniority_level" enums:"intern,junior,mid,senior,lead"`
	CreatedAt      time.Time  `json:"created_at"`
	PublishedAt    *time.Time `json:"published_at"`
}

// BoardRequirement is one requirement the Recruiter confirmed for the posting.
type BoardRequirement struct {
	Name   string `json:"name"`
	Target string `json:"target"`
	Type   string `json:"type" enums:"required,preferred"`
}

// BoardPostingDetail is the public content of an open Job Posting. RawText is the JD text.
// AlreadyApplied tells the Candidate whether they have an Application for it.
type BoardPostingDetail struct {
	BoardPosting
	CompanyWebsite    *string            `json:"company_website"`
	InterviewDuration *int32             `json:"interview_duration"`
	RawText           string             `json:"raw_text"`
	Requirements      []BoardRequirement `json:"requirements"`
	AlreadyApplied    bool               `json:"already_applied"`
}

// BoardRecord is what the repository returns for one posting. A posting created before PDF upload
// has no StoredText: its ContentPath holds the JD as a plain-text object, which the service reads.
type BoardRecord struct {
	BoardPostingDetail
	ContentPath string
	StoredText  *string
}

// BoardFilter narrows the job board. A nil field means "no filter". Search and Location are matched
// as substrings of the title and location; EmploymentType must match the whole value. Case is
// ignored everywhere.
type BoardFilter struct {
	PublishedFrom  *time.Time
	PublishedTo    *time.Time
	Search         *string
	Location       *string
	EmploymentType *string
	Limit          int32
	Offset         int32
}
