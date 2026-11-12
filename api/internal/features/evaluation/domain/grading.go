package domain

import "github.com/google/uuid"

// Rubric grading (KAN-334, RB-10). The AI grades every criterion of the rubric of one core question against
// that question's expected answer, using only the Candidate's answers to that question and to its
// follow-ups. The backend validates the reply and computes every aggregate itself (CalculateRubricScore):
// the model never sees weights and never decides a metric score or the overall.

// GradeCriterion is one rubric criterion as the grader sees it. Key is the short label (C1, C2, ...) the
// model answers with; the server maps it back to ID, so a model never has to copy a UUID.
type GradeCriterion struct {
	ID           uuid.UUID
	Key          string
	MetricID     uuid.UUID
	MetricName   string
	Name         string
	MaxPoints    int32
	ScoringGuide string
}

// GradeTurn is one answered turn of a core question: the core question itself or a follow-up about it.
type GradeTurn struct {
	Position int32
	Kind     string
	Question string
	Answer   string
}

// QuestionGradeInput is everything one grading call may see: one core question, its rubric and expected
// answer, and the Candidate's answers to it and to its follow-ups. Rubric text, the expected answer, the
// answers and model-written follow-ups are untrusted data. The expected answer is never shown to the
// Candidate and never logged.
type QuestionGradeInput struct {
	// QuestionPosition is the core question's place in the session (interview_questions.position).
	QuestionPosition  int32
	RubricName        string
	RubricDescription string
	ExpectedAnswer    string
	Criteria          []GradeCriterion
	// Turns are the answered turns of this question in conversation order; the first is usually the core
	// question. Turns with an empty answer are never sent.
	Turns []GradeTurn
	// RedactNames are the Candidate's own names, replaced before text goes to a provider (KAN-298).
	RedactNames []string
}

// CriterionGradeCandidate is one criterion of the model's reply, already decoded and mapped from its key to
// the criterion id. It is not trusted until ValidateQuestionGrade accepts it.
type CriterionGradeCandidate struct {
	CriterionID uuid.UUID
	Points      int32
	Evidence    string
	Feedback    string
}

// QuestionGradeCandidate is the decoded reply of one grading call.
type QuestionGradeCandidate struct {
	Criteria           []CriterionGradeCandidate
	Feedback           string
	Summary            string
	InjectionSuspected bool
}

// CriterionGrade is a validated criterion result. EvidencePosition is the conversation position of the turn
// the quote was found in; it is nil only when Points is 0 and there is nothing to quote.
type CriterionGrade struct {
	CriterionID      uuid.UUID
	MetricID         uuid.UUID
	MaxPoints        int32
	Points           int32
	Evidence         string
	EvidencePosition *int32
	Feedback         string
}

// QuestionGrade is the validated result of one core question: one entry per rubric criterion, in rubric
// order. Feedback is stored on the core turn; Summary is one short sentence that joins the session summary.
type QuestionGrade struct {
	QuestionPosition int32
	Criteria         []CriterionGrade
	Feedback         string
	Summary          string
	// InjectionFlagged is the model's own report, a known injection pattern in an answer or a follow-up,
	// or model text that echoed an instruction (and was dropped).
	InjectionFlagged bool
}
