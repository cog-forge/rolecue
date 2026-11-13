package domain

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	evaluationdomain "github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
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
	Revision          string     `json:"revision"`
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
	q := BankQuestion{ID: id, Content: content, RubricID: rubricID, ExpectedAnswer: expectedAnswer,
		GradingIncomplete: len(issues) > 0, GradingIssues: issues}
	data, _ := json.Marshal(q)
	q.Revision = fmt.Sprintf("%x", sha256.Sum256(data))
	return q
}

// Bank is a Recruiter's view of a posting's bank. Editable and the limits come from the server, so
// the screen never guesses whether the bank is locked or how long a question may be.
type Bank struct {
	MaxExpectedAnswerLength     int            `json:"max_expected_answer_length"`
	MaxExpectedAnswerWords      int            `json:"max_expected_answer_words"`
	ExpectedAnswerGuidanceWords int            `json:"expected_answer_guidance_words"`
	MaxBulkQuestions            int            `json:"max_bulk_questions"`
	Questions                   []BankQuestion `json:"questions"`
	PostingStatus               string         `json:"posting_status"`
	Editable                    bool           `json:"editable"`
	MaxQuestionLength           int            `json:"max_question_length"`
	// GenerationSize is how many questions Generate writes; 0 means generation is not configured.
	GenerationSize int `json:"generation_size"`
	// ReviewSubmitted is true once the Recruiter sent the posting to the Admin queue. Changing a question after
	// that withdraws the submission, so the Recruiter submits again (KAN-331).
	ReviewSubmitted bool `json:"review_submitted"`
	// GradingReady is true when the bank is not empty and every question is complete; only then can the posting
	// be submitted. GradingIncompleteCount is how many questions are not.
	GradingReady           bool `json:"grading_ready"`
	GradingIncompleteCount int  `json:"grading_incomplete_count"`
	// Coverage says whether the bank can cover every active metric within the session question limit
	// (KAN-377); the posting cannot be submitted or approved while it does not pass.
	Coverage *evaluationdomain.CoverageReport `json:"coverage"`
}

// BankMutation is what a bank-wide write returns (generate, bulk rubric): the questions it wrote and the
// coverage of the bank afterwards. Coverage is null only when it could not be read after the write; the
// write itself succeeded, and the bank view carries the coverage.
type BankMutation struct {
	Questions []BankQuestion                   `json:"questions"`
	Coverage  *evaluationdomain.CoverageReport `json:"coverage"`
}

// QuestionMutation is what a single-question write returns: the question, flattened as before, plus the
// coverage of the bank after the write (null when it could not be read).
type QuestionMutation struct {
	BankQuestion
	Coverage *evaluationdomain.CoverageReport `json:"coverage"`
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
	// MetricID backs the coverage check of a generated bank (KAN-377); it is never placed in the prompt.
	MetricID uuid.UUID `json:"-"`
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

// MetricIDs lists the distinct metrics the rubric has criteria for.
func (r RubricOption) MetricIDs() []uuid.UUID {
	out := make([]uuid.UUID, 0, len(r.Criteria))
	seen := make(map[uuid.UUID]bool, len(r.Criteria))
	for _, criterion := range r.Criteria {
		if criterion.MetricID != uuid.Nil && !seen[criterion.MetricID] {
			seen[criterion.MetricID] = true
			out = append(out, criterion.MetricID)
		}
	}
	return out
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
	// ActiveMetrics are the metrics the generated bank must be able to cover (KAN-377, KAN-378): the active
	// metrics of the posting being generated for, or the whole catalog for a practice bank. The service
	// reads them; they are not part of the prompt.
	ActiveMetrics []uuid.UUID
	// Topics are the Roadmap topics a practice bank must be mapped to (KAN-344): every generated question
	// names exactly one of them, and each needs at least MinQuestionsPerTopic distinct questions. Empty for a
	// Recruiter's posting bank and for a bank without a Roadmap, which keep the topic-free contract.
	Topics []RoadmapTopic
	// Size, when positive, is how many questions this call writes instead of the configured bank size (a top-up of
	// one topic). Existing are bank questions already stored, given to the model as data so it writes new ones; a
	// proposed question equal to one of them is dropped by validation.
	Size     int
	Existing []string
}

// ErrBankInUse means the practice bank cannot be replaced because the profile already has an interview: its
// questions are the history of those sessions.
var ErrBankInUse = errors.New("the practice question bank is used by an interview")

// MinQuestionsPerTopic is the product rule that a Roadmap topic needs five distinct questions in the bank to
// be achievable (Learning Roadmap spec, rule 8).
const MinQuestionsPerTopic = 5

// RoadmapTopic is one topic of the Roadmap, with the id the backend assigned. Quota is how many questions the
// model is asked to write for it in this call. Title and Description were generated from an untrusted job
// description, so the prompt treats them as data.
type RoadmapTopic struct {
	ID          uuid.UUID
	Title       string
	Description string
	Quota       int
}

// PracticeBankSize is how many questions a practice bank is generated with: the configured size, raised to
// MinQuestionsPerTopic per topic when that is more, so a bank is always large enough for the distribution rule
// to be satisfiable. The configured size alone guarantees nothing per topic.
func PracticeBankSize(configured, topics int) int {
	return max(configured, MinQuestionsPerTopic*topics)
}

// BalancedQuotas spreads size questions over the topics as evenly as possible; the earlier (more important)
// topics take the remainder. The topics are copied.
func BalancedQuotas(topics []RoadmapTopic, size int) []RoadmapTopic {
	out := append([]RoadmapTopic(nil), topics...)
	if len(out) == 0 {
		return out
	}
	for i := range out {
		out[i].Quota = size / len(out)
		if i < size%len(out) {
			out[i].Quota++
		}
	}
	return out
}

// BatchQuotas picks the topics and counts of the next batch of at most size questions: topics still short of
// their Quota (have counts the questions already written per topic), most missing first, one question at a
// time. It returns only topics with a positive allotment, the allotment in Quota; the sum is at most size.
func BatchQuotas(topics []RoadmapTopic, have map[uuid.UUID]int, size int) []RoadmapTopic {
	out := make([]RoadmapTopic, 0, len(topics))
	allot := make([]int, len(topics))
	missing := make([]int, len(topics))
	for i, topic := range topics {
		missing[i] = topic.Quota - have[topic.ID]
	}
	for given := 0; given < size; {
		best := -1
		for i := range topics {
			if missing[i]-allot[i] > 0 && (best < 0 || missing[i]-allot[i] > missing[best]-allot[best]) {
				best = i
			}
		}
		if best < 0 {
			break
		}
		allot[best]++
		given++
	}
	for i, topic := range topics {
		if allot[i] > 0 {
			topic.Quota = allot[i]
			out = append(out, topic)
		}
	}
	return out
}

// CountByTopic counts questions per topic id; questions without a topic are ignored.
func CountByTopic(questions []GeneratedQuestion) map[uuid.UUID]int {
	counts := make(map[uuid.UUID]int)
	for _, question := range questions {
		if question.TopicID != nil {
			counts[*question.TopicID]++
		}
	}
	return counts
}

// GeneratedQuestion is one proposed question. RubricID and ExpectedAnswer are set only in grading mode;
// a nil RubricID means "no suitable rubric", a nil ExpectedAnswer means none is stored (the provider always
// sets a non-nil value for what the model returned; validation clears it when screening rejects the text).
type GeneratedQuestion struct {
	Content        string
	RubricID       *uuid.UUID
	ExpectedAnswer *string
	// TopicID is the one Roadmap topic the question belongs to; set only when the input carries topics.
	TopicID *uuid.UUID
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

// QuestionPatch distinguishes omitted fields from explicit null clears. The revision is a
// content digest, not a stored version; compare it while holding the posting lock.
type QuestionPatch struct {
	Content           *string
	SetRubric         bool
	RubricID          *uuid.UUID
	SetExpectedAnswer bool
	ExpectedAnswer    *string
	PreviousRevision  string
}

const MaxBulkQuestions = 100

type BulkRubricRequest struct {
	QuestionIDs       []uuid.UUID          `json:"question_ids"`
	RubricID          uuid.UUID            `json:"rubric_id"`
	PreviousRevisions map[uuid.UUID]string `json:"previous_revisions"`
}
