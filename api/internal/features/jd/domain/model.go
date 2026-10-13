package domain

import (
	"time"

	"github.com/google/uuid"
)

// Seniority is optional: nil means the JD supplies insufficient evidence.
type Seniority string

const (
	Intern Seniority = "intern"
	Junior Seniority = "junior"
	Mid    Seniority = "mid"
	Senior Seniority = "senior"
	Lead   Seniority = "lead"
)

type SkillCategory string

const (
	ProgrammingLanguage SkillCategory = "programming_language"
	Framework           SkillCategory = "framework"
	Database            SkillCategory = "database"
	Tool                SkillCategory = "tool"
	Technology          SkillCategory = "technology"
	Other               SkillCategory = "other"
)

type Requirement string

const (
	Required  Requirement = "required"
	Preferred Requirement = "preferred"
)

type ExtractedSkill struct {
	Name     string        `json:"name"`
	Category SkillCategory `json:"category"`
	// Empty means the JD does not state whether the skill is required or preferred.
	Requirement Requirement `json:"requirement,omitempty"`
}

// ExtractionCandidate is an untrusted proposal, including when decoded successfully.
// Nil collections indicate missing/null structural data; empty arrays are valid.
type ExtractionCandidate struct {
	Title           string           `json:"title"`
	SeniorityLevel  *Seniority       `json:"seniorityLevel,omitempty"`
	Skills          []ExtractedSkill `json:"skills"`
	Technologies    []string         `json:"technologies"`
	DomainKnowledge []string         `json:"domainKnowledge"`
}

// StructuredJD is returned only after deterministic domain validation.
type StructuredJD struct {
	Title           string           `json:"title"`
	SeniorityLevel  *Seniority       `json:"seniorityLevel,omitempty"`
	Skills          []ExtractedSkill `json:"skills"`
	Technologies    []string         `json:"technologies"`
	DomainKnowledge []string         `json:"domainKnowledge"`
}

// JD is a Candidate's saved job description: the original text (read back from file storage)
// plus the reviewed structure stored across job_descriptions, role_profiles and requirements.
type JD struct {
	ID           uuid.UUID
	RawText      string
	StructuredJD StructuredJD
	CreatedAt    time.Time
}

// Record is what the repository stores and returns. ContentPath is the storage key of the
// original JD text; the service turns it back into RawText.
type Record struct {
	ID           uuid.UUID
	ContentPath  string
	Title        string
	Seniority    Seniority
	Requirements []RequirementRow
	CreatedAt    time.Time
}

// RequirementRow is one row of the requirements table.
type RequirementRow struct {
	Name   string
	Target string
	Type   Requirement
}

// Requirement targets for the two plain-name lists; skills use their SkillCategory as target.
const (
	TargetTechnologies    = "technologies"
	TargetDomainKnowledge = "domain_knowledge"
)

// ListItem contains the summary fields of a persisted job description.
type ListItem struct {
	ID             uuid.UUID `json:"id"`
	Title          string    `json:"title"`
	SeniorityLevel Seniority `json:"seniorityLevel"`
	CreatedAt      time.Time `json:"createdAt"`
}

// CreateInput contains the original JD text and its reviewed structure.
type CreateInput struct {
	RawText      string
	StructuredJD StructuredJD
}

// UpdateInput contains only fields which may change after JD creation.
type UpdateInput struct {
	StructuredJD StructuredJD
}
