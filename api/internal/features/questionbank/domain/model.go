package domain

import "github.com/google/uuid"

// A Recruiter may change the bank only while preparing the posting: pending, or rejected and being
// corrected for resubmission.
const (
	PostingPending  = "pending"
	PostingRejected = "rejected"
)

// MaxQuestionRunes is a technical guard on one question, not a product rule.
const MaxQuestionRunes = 1000

// CoreQuestion is one row of a JD's current core-question bank.
type CoreQuestion struct {
	ID      uuid.UUID `json:"id"`
	Content string    `json:"content"`
}

// Bank is a Recruiter's view of a posting's bank. Editable and the limits come from the server, so
// the screen never guesses whether the bank is locked or how long a question may be.
type Bank struct {
	Questions         []CoreQuestion `json:"questions"`
	PostingStatus     string         `json:"posting_status"`
	Editable          bool           `json:"editable"`
	MaxQuestionLength int            `json:"max_question_length"`
	// GenerationSize is how many questions Generate writes; 0 means generation is not configured.
	GenerationSize int `json:"generation_size"`
	// ReviewSubmitted is true once the Recruiter sent the posting to the Admin queue.
	ReviewSubmitted bool `json:"review_submitted"`
}

// Posting is the part of a Job Posting the bank needs: its JD and its lifecycle status.
type Posting struct {
	JDID            uuid.UUID
	Status          string
	ReviewSubmitted bool
}

// Editable reports whether the posting's bank may still change.
func (p Posting) Editable() bool { return p.Status == PostingPending || p.Status == PostingRejected }

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
