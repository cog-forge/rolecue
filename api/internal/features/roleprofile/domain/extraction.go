package domain

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
	Target       string        `json:"target,omitempty"`
	EvidenceText *string       `json:"evidenceText,omitempty"`
	Name         string        `json:"name"`
	Category     SkillCategory `json:"category"`
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
