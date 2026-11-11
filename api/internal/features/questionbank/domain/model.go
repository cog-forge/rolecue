package domain

import (
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

// A Recruiter may change the bank only while preparing the posting: pending, or rejected and being
// corrected for resubmission.
const (
	PostingPending  = "pending"
	PostingRejected = "rejected"
)

// MaxQuestionRunes is a technical guard on one question, not a product rule.
const MaxQuestionRunes = 1000

// Bounds on a generated expected answer. The prompt asks for at most ExpectedAnswerPromptWords words; the
// server accepts up to MaxExpectedAnswerWords because models drift about a fifth over a word budget and every
// rejection costs a retry. MaxExpectedAnswerRunes backs the word count for text without spaces (CJK) and
// keeps a bank of 20 answers small.
const (
	ExpectedAnswerPromptWords = 80
	MaxExpectedAnswerWords    = 100
	MaxExpectedAnswerRunes    = 1000
)

// Bounds on the rubric catalog placed in the generation prompt (KAN-329). The catalog is the allowed set the
// model may choose from; it is capped so a Recruiter with many rubrics cannot inflate the prompt.
const (
	MaxPromptRubrics         = 20
	MaxPromptRubricCriteria  = 8
	MaxPromptRubricTextRunes = 1200
)

// CoreQuestion is one row of a JD's current core-question bank. It is the shape the interview runtime reads
// through CurrentBank and carries no grading data, so nothing a Candidate-facing path touches can leak it.
type CoreQuestion struct {
	ID      uuid.UUID `json:"id"`
	Content string    `json:"content"`
}

// Grading issues a question can have (KAN-331). A question with no issue has a rubric the Recruiter may
// still use and a valid expected answer; a posting can be submitted and approved only when every question is
// like that.
const (
	IssueMissingRubric         = "missing_rubric"
	IssueRubricArchived        = "rubric_archived"
	IssueRubricUnavailable     = "rubric_unavailable"
	IssueMissingExpectedAnswer = "missing_expected_answer"
	IssueExpectedAnswerInvalid = "expected_answer_invalid"
)

// RubricState says whether a question's rubric can still be used: Active is false once the rubric is archived,
// Permitted is false when it is not a system rubric or one of the posting owner's own.
type RubricState struct {
	Active    bool
	Permitted bool
}

// BankQuestion is one question as the Recruiter sees it: the same id and content as CoreQuestion plus its
// grading setup (KAN-329, KAN-331). RubricID and ExpectedAnswer are null for questions generated before
// rubrics existed, added by hand, or generated when no allowed rubric fitted. GradingIssues lists why the
// question is not ready (empty when it is); GradingIncomplete is true while there is any. The server never
// treats an incomplete question as confirmed. Recruiter-only: never place it in a Candidate response.
type BankQuestion struct {
	ID                uuid.UUID  `json:"id" format:"uuid"`
	Content           string     `json:"content"`
	RubricID          *uuid.UUID `json:"rubric_id" format:"uuid"`
	ExpectedAnswer    *string    `json:"expected_answer"`
	GradingIncomplete bool       `json:"grading_incomplete"`
	GradingIssues     []string   `json:"grading_issues"`
}

// ExpectedAnswerValid reports whether a stored expected answer is within the bounds a bank must meet.
func ExpectedAnswerValid(answer string) bool {
	text := strings.TrimSpace(answer)
	return text != "" && utf8.RuneCountInString(answer) <= MaxExpectedAnswerRunes && len(strings.Fields(text)) <= MaxExpectedAnswerWords
}

// GradingIssuesOf derives the issues of a question in one place; the SQL readiness predicates of the posting
// submit and the Admin approval apply the same rules and are tested against this function.
func GradingIssuesOf(rubricID *uuid.UUID, state RubricState, expectedAnswer *string) []string {
	issues := []string{}
	switch {
	case rubricID == nil:
		issues = append(issues, IssueMissingRubric)
	case !state.Permitted:
		issues = append(issues, IssueRubricUnavailable)
	case !state.Active:
		issues = append(issues, IssueRubricArchived)
	}
	switch {
	case expectedAnswer == nil || strings.TrimSpace(*expectedAnswer) == "":
		issues = append(issues, IssueMissingExpectedAnswer)
	case !ExpectedAnswerValid(*expectedAnswer):
		issues = append(issues, IssueExpectedAnswerInvalid)
	}
	return issues
}

// NewBankQuestion builds the Recruiter's view of a question, deriving its grading issues.
func NewBankQuestion(id uuid.UUID, content string, rubricID *uuid.UUID, state RubricState, expectedAnswer *string) BankQuestion {
	issues := GradingIssuesOf(rubricID, state, expectedAnswer)
	return BankQuestion{ID: id, Content: content, RubricID: rubricID, ExpectedAnswer: expectedAnswer,
		GradingIncomplete: len(issues) > 0, GradingIssues: issues}
}

// Bank is a Recruiter's view of a posting's bank. Editable and the limits come from the server, so
// the screen never guesses whether the bank is locked or how long a question may be.
type Bank struct {
	Questions         []BankQuestion `json:"questions"`
	PostingStatus     string         `json:"posting_status"`
	Editable          bool           `json:"editable"`
	MaxQuestionLength int            `json:"max_question_length"`
	// GenerationSize is how many questions Generate writes; 0 means generation is not configured.
	GenerationSize int `json:"generation_size"`
	// ReviewSubmitted is true once the Recruiter sent the posting to the Admin queue. Changing a question after
	// that withdraws the submission, so the Recruiter submits again (KAN-331).
	ReviewSubmitted bool `json:"review_submitted"`
	// GradingReady is true when the bank is not empty and every question is complete; only then can the posting
	// be submitted. GradingIncompleteCount is how many questions are not.
	GradingReady           bool `json:"grading_ready"`
	GradingIncompleteCount int  `json:"grading_incomplete_count"`
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

// RubricOptionCriterion is one criterion of an allowed rubric, summarized for the prompt.
type RubricOptionCriterion struct {
	Name      string `json:"name"`
	Metric    string `json:"metric"`
	MaxPoints int32  `json:"max_points"`
}

// RubricOption is one rubric the model may choose for a question: an active system template or an active
// rubric of the posting's owner. Its text is written by Admins and Recruiters, so it is untrusted data.
type RubricOption struct {
	ID           uuid.UUID
	Name         string
	Description  string
	QuestionType string
	Criteria     []RubricOptionCriterion
}

// GenerationInput is everything the generator may use to write the bank. The seniority level and
// the refinement note both come from the JD itself. Grading is true for a Recruiter's posting: the model
// then also returns a rubric choice and an expected answer per question, and Rubrics is the only set of
// rubric ids it may choose from (possibly empty). Grading is false for a Candidate's practice bank, which
// keeps the question-only contract.
type GenerationInput struct {
	Seniority      SeniorityLevel
	Requirements   []ConfirmedRequirement
	RefinementNote string
	Grading        bool
	Rubrics        []RubricOption
}

// GeneratedQuestion is one proposed question. RubricID and ExpectedAnswer are set only in grading mode;
// a nil RubricID means "no suitable rubric", a nil ExpectedAnswer means none is stored (the provider always
// sets a non-nil value for what the model returned; validation clears it when screening rejects the text).
type GeneratedQuestion struct {
	Content        string
	RubricID       *uuid.UUID
	ExpectedAnswer *string
}

// GenerationCandidate is an untrusted proposal from the model. Only service validation turns it
// into stored questions.
type GenerationCandidate struct {
	Questions []GeneratedQuestion
}

// Contents lists the question texts, in order.
func (c GenerationCandidate) Contents() []string {
	out := make([]string, 0, len(c.Questions))
	for _, q := range c.Questions {
		out = append(out, q.Content)
	}
	return out
}

// CandidateOf wraps plain question texts as a candidate without grading data.
func CandidateOf(contents ...string) GenerationCandidate {
	c := GenerationCandidate{Questions: make([]GeneratedQuestion, 0, len(contents))}
	for _, content := range contents {
		c.Questions = append(c.Questions, GeneratedQuestion{Content: content})
	}
	return c
}
