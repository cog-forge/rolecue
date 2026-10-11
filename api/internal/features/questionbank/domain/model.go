package domain

import "github.com/google/uuid"

// PostingPending is the only job-posting status in which a Recruiter may change the bank.
const PostingPending = "pending"

// MaxQuestionRunes is a technical guard on one question, not a product rule.
const MaxQuestionRunes = 1000

// CoreQuestion is one row of a JD's current core-question bank.
type CoreQuestion struct {
	ID      uuid.UUID `json:"id"`
	Content string    `json:"content"`
}

// Posting is the part of a Job Posting the bank needs: its JD and its lifecycle status.
type Posting struct {
	JDID   uuid.UUID
	Status string
}

// ConfirmedRequirement is one confirmed requirement of a JD, used only as generation input.
type ConfirmedRequirement struct {
	Name         string  `json:"name"`
	Target       string  `json:"target"`
	Type         string  `json:"type"`
	EvidenceText *string `json:"evidence_text"`
}

// SeniorityLevel is the JD's level (job_descriptions.level_id -> levels). It describes the whole
// job description, so it is not part of a single requirement.
type SeniorityLevel struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

// GenerationInput is everything the generator may use to write the bank. The seniority level and
// the refinement note both come from the JD itself.
type GenerationInput struct {
	Seniority      SeniorityLevel
	Requirements   []ConfirmedRequirement
	RefinementNote string
}

// GenerationCandidate is an untrusted proposal from the model. Only service validation turns it
// into stored questions.
type GenerationCandidate struct {
	Questions []string `json:"questions"`
}
