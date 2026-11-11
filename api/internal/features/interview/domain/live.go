package domain

import "time"

// TurnKind tells a core question from a bounded follow-up (KAN-56). The kind is stored on every turn so
// the report and the Recruiter's evidence can show a follow-up under its core question.
type TurnKind string

const (
	TurnCore     TurnKind = "core"
	TurnFollowUp TurnKind = "follow_up"
)

// AskedQuestion is the question the live interviewer asks now. Position is its place in the
// conversation (core questions and follow-ups alike), the number an answer names. QuestionNumber is the
// core question it belongs to, 1..QuestionCount: the core question itself, or the one a follow-up
// probes. Text is what the interviewer speaks. A core question's text is frozen on its first delivery,
// so a reconnect repeats exactly the same words and the stored turn keeps them. A follow-up's text lives
// only in the socket that asked it and is stored with its answer. It never carries the core question id
// or any other bank data. Cues are the performance cues a follow-up decider attached to the line (KAN-250),
// already validated; nil for a core question, whose cues come from rules in the live engine.
type AskedQuestion struct {
	Position       int32
	QuestionNumber int32
	Kind           TurnKind
	Text           string
	AskedAt        time.Time
	Cues           []PerformanceCue
}

// Delivery is the live engine's view of a session at a turn boundary. QuestionCount is the number of core
// questions; AnsweredCount the number of stored answers, follow-ups included, which is also the last
// answered position. Question is nil when every question is answered: the session is then ready to
// complete. ProviderVoiceID is the provider voice the session speaks with, frozen on the first delivery;
// it is empty when the session has no provider voice and no default is configured, and the interviewer
// is then shown as text only.
type Delivery struct {
	Type            Type
	QuestionCount   int32
	AnsweredCount   int32
	Question        *AskedQuestion
	ProviderVoiceID string
}

// FollowUpContext is what a follow-up decider sees for one slot: the core question, what was said about
// it so far and the requirements of the Job Description. The answers are the Candidate's words and are
// untrusted data. Position is where the follow-up would be asked.
type FollowUpContext struct {
	Position       int32
	QuestionNumber int32
	Ordinal        int32
	CoreQuestion   string
	// Exchanges is the core question's answer, then each earlier follow-up and its answer, in order.
	Exchanges    []Exchange
	Requirements []Requirement
	// Guideline is the Interview Behavior Guideline the session started under (KAN-276); the defaults when
	// the session has none. The decider words and decides the follow-up within it.
	Guideline BehaviorGuideline
}

// Exchange is one question the interviewer asked and the Candidate's answer.
type Exchange struct {
	Question string
	Answer   string
}

// Requirement is one requirement of the Job Description the session was started from.
type Requirement struct {
	Name   string
	Target string
	// Type is required or preferred; empty when the JD does not say.
	Type string
}

// FollowUpProposal is a decider's untrusted answer: ask a follow-up (with its wording) or not, and why.
// The service validates it before anything is asked. Cues are optional performance cues for the question,
// word-anchored and limited to the bounded vocabulary (KAN-250); the service drops the whole list when any
// cue is invalid, and the engine then uses its rules.
type FollowUpProposal struct {
	FollowUp bool
	Question string
	Reason   string
	Cues     []PerformanceCue
}

// Slot is the place of the next turn: its position in the conversation, the core question it belongs to
// and, for a follow-up, which follow-up of that core question it is (1-based; 0 for a core question).
type Slot struct {
	Position       int32
	QuestionNumber int32
	Ordinal        int32
}

// NextCore is the core question a session asks next: the one after the core questions already answered,
// at the next position. It reports false when every core question is answered. A follow-up that was asked
// but not answered is never stored, so it never blocks the next core question: when the socket drops
// during a follow-up, the reconnect goes on with the next core question (team decision 2026-11-03).
func NextCore(turns []Turn, questionCount int32) (Slot, bool) {
	answered := int32(0)
	for _, turn := range turns {
		if turn.Kind != TurnFollowUp {
			answered++
		}
	}
	if answered >= questionCount {
		return Slot{}, false
	}
	return Slot{Position: int32(len(turns)) + 1, QuestionNumber: answered + 1}, true
}

// FollowUpSlot reports whether the interviewer may ask a follow-up after the last stored answer, and
// where. Only the core question answered last can get one, and at most followUpLimit of them, counted
// from the stored follow-up answers. Whatever a decider returns, a core question therefore never gets
// more than followUpLimit follow-ups; with followUpLimit 0 there are none.
func FollowUpSlot(turns []Turn, followUpLimit int32) (Slot, bool) {
	ordinal := int32(1)
	core := -1
	for i := len(turns) - 1; i >= 0; i-- {
		if turns[i].Kind != TurnFollowUp {
			core = i
			break
		}
		ordinal++
	}
	if core < 0 || ordinal > followUpLimit {
		return Slot{}, false
	}
	return Slot{Position: int32(len(turns)) + 1, QuestionNumber: turns[core].QuestionNumber, Ordinal: ordinal}, true
}
