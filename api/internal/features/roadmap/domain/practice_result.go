package domain

import "time"

// Practice-result adapter rules (KAN-349, RM-08). The Roadmap reads the per-core results of a finished practice
// session from data the interview and evaluation features already store; it adds no column. This file holds
// the pure decisions. The facts come from a query on interview_questions, conversation_turns and interviews.
//
// What the stored data says about one assigned core question:
//
//   - Delivered: interview_questions.asked_at is set, which the live engine does when it delivers the question
//     to the Candidate. A core the session never reached has no asked_at.
//   - Answered: the session has a stored turn for that position (a turn is stored only with an answer, and the
//     retention pass clears the answer text but keeps the turn). The engine asks the
//     core questions in position order and stores one core turn per answer, so the n-th core turn answers
//     position n (the same rule the grader uses). Follow-ups belong to their core and add nothing.
//   - QuestionScore: interview_questions.score, the Barem total of the core, set only for a graded core.
//
// Barem keeps ignoring unanswered questions for the interview report and the session overall; the rule below
// applies to the Roadmap's own per-question score only.

// CoreOutcome is what a core question contributes to the Roadmap.
type CoreOutcome string

const (
	// CoreScored: answered and graded; the Barem question score counts.
	CoreScored CoreOutcome = "scored"
	// CoreSkipped: delivered to the Candidate and not answered when the session ended. Counts as 0.
	CoreSkipped CoreOutcome = "skipped"
	// CoreNotAsked: never delivered. Not counted, not 0.
	CoreNotAsked CoreOutcome = "not_asked"
	// CoreUngraded: answered but no question score exists (a question without rubric, or a grading outcome
	// without a result for it). Not counted, not 0: an unreliable or missing evaluation is never evidence.
	CoreUngraded CoreOutcome = "ungraded"
)

// CoreFacts are the stored facts about one assigned core question of a finished session.
type CoreFacts struct {
	Position      int32
	Delivered     bool
	Answered      bool
	QuestionScore *float64
}

// CoreResult is the Roadmap's reading of CoreFacts. Score is set exactly when the core counts.
type CoreResult struct {
	Outcome CoreOutcome
	Score   *float64
}

// Counted reports whether the core is evidence for its topic.
func (r CoreResult) Counted() bool { return r.Score != nil }

// ClassifyCore applies the Roadmap rule of the 13/11 decision (spec 2, rule 11): a shown-but-unanswered core is
// 0, an unasked core and a core without a trustworthy grade are not counted.
//
// It cannot tell a deliberate skip from a technical problem that kept the Candidate from answering the last
// delivered question (speech recognition that never succeeded, audio that did not play) before the Candidate
// left; neither is stored today. At most one core per session can be in that state, because the cores are
// delivered and answered in order.
func ClassifyCore(f CoreFacts) CoreResult {
	switch {
	case f.QuestionScore != nil:
		score := *f.QuestionScore
		return CoreResult{Outcome: CoreScored, Score: &score}
	case f.Answered:
		return CoreResult{Outcome: CoreUngraded}
	case f.Delivered:
		zero := 0.0
		return CoreResult{Outcome: CoreSkipped, Score: &zero}
	default:
		return CoreResult{Outcome: CoreNotAsked}
	}
}

// SessionFacts are the stored facts about the interview that decide whether its results may feed a Roadmap.
type SessionFacts struct {
	// Type is interviews.type: "practice" or "recruitment".
	Type             string
	CompletedAt      *time.Time
	EvaluationStatus string
	// InjectionFlagged is interviews.injection_flagged: the evaluation could not fully trust the Candidate's
	// answers (the answers tried to steer the grader).
	InjectionFlagged bool
}

// Eligible reports whether the session's results may feed a Roadmap: a completed practice session whose
// evaluation succeeded and could be trusted. A withheld overall is still a successful evaluation. Recruitment
// results never feed a Roadmap; an unfinished session and a pending or failed evaluation update nothing, and an
// injection-flagged evaluation is not evidence (spec 8: unreliable evaluation data is never evidence).
//
// This is the one predicate for "may this session feed a Roadmap": the read model, the assessment writer and
// the catch-up job all use it (the catch-up SQL pre-filter mirrors it and is tested against it).
func (s SessionFacts) Eligible() bool {
	return s.Type == "practice" && s.CompletedAt != nil && s.EvaluationStatus == "completed" && !s.InjectionFlagged
}
