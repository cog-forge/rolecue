package domain

import (
	"time"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// ErrRequirementsLocked: a JD's requirements and refinement note are the snapshot its question
// bank was generated from, so they can't change once the bank exists.
var ErrRequirementsLocked = apperror.New(apperror.CodeJDRequirementsLocked, "requirements can't change after the question bank is generated")

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
	// EvidenceText is the Candidate's quote or note from the JD backing this skill. Only
	// reviewed input may set it; extraction output never does.
	EvidenceText *string `json:"evidenceText,omitempty"`
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
	ID             uuid.UUID
	RawText        string
	StructuredJD   StructuredJD
	RefinementNote *string
	// QuestionBankReady says whether the hidden question bank exists. It carries no questions.
	QuestionBankReady bool
	CreatedAt         time.Time
}

// Record is what the repository stores and returns. ContentPath is the storage key of the
// original JD text; the service turns it back into RawText.
type Record struct {
	ID             uuid.UUID
	ContentPath    string
	Title          string
	Seniority      Seniority
	RefinementNote *string
	Requirements   []RequirementRow
	// HasQuestionBank is true once core questions exist; the requirements are then locked.
	HasQuestionBank bool
	CreatedAt       time.Time
}

// RequirementRow is one row of the requirements table.
type RequirementRow struct {
	Name         string
	Target       string
	Type         Requirement
	EvidenceText *string
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

// CreateInput is a Candidate's confirmation: the original JD text, the reviewed structure and
// an optional refinement note. Confirming is what persists a JD; nothing is stored before it.
type CreateInput struct {
	RawText        string
	StructuredJD   StructuredJD
	RefinementNote string
}

// UpdateInput contains only fields which may change after confirmation, until the question bank
// is generated.
type UpdateInput struct {
	StructuredJD   StructuredJD
	RefinementNote string
}
