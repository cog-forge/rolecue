package domain

import (
	"time"

	"github.com/google/uuid"
)

// Type is the kind of interview session. A practice session is the Candidate's own rehearsal and is paid
// for; a recruitment session is the Recruiter's interview of an Application and is free for the Candidate.
type Type string

const (
	TypePractice    Type = "practice"
	TypeRecruitment Type = "recruitment"
)

// EvaluationStatus is the state of the evaluation of a completed session. The evaluator writes it.
type EvaluationStatus string

const (
	EvaluationPending   EvaluationStatus = "pending"
	EvaluationCompleted EvaluationStatus = "completed"
	EvaluationFailed    EvaluationStatus = "failed"
)

// RecordStatus is the state of a recruitment recording. Only the recording feature writes it.
type RecordStatus string

const (
	RecordNone       RecordStatus = "none"
	RecordUploading  RecordStatus = "uploading"
	RecordProcessing RecordStatus = "processing"
	RecordReady      RecordStatus = "ready"
	RecordFailed     RecordStatus = "failed"
	// RecordDeleted is a recording purged after the retention period: the object and its key are gone.
	RecordDeleted RecordStatus = "deleted"
)

// EndReason is how a session ended. Every ending goes through the one end path (the repository's End):
// EndCompleted when every core question was answered, EndLeft when the Candidate ended it early, and
// EndAbandoned when the System Handler ended it after the inactivity timeout. A session ended before the
// reason was recorded has none.
type EndReason string

const (
	EndCompleted EndReason = "completed"
	EndLeft      EndReason = "left"
	EndAbandoned EndReason = "abandoned"
)

// EndCause is who ends a session: the Candidate (a normal completion or leaving early, told apart by
// the answers stored) or the System Handler after the inactivity timeout.
type EndCause int

const (
	EndByCandidate EndCause = iota
	EndBySystem
)

// ReasonFor decides the end reason under the session lock. A Candidate ending the session after every
// core question is answered completed it, earlier they left it; the System Handler only abandons.
func ReasonFor(cause EndCause, answeredCore, questionCount int32) EndReason {
	if cause == EndBySystem {
		return EndAbandoned
	}
	if answeredCore >= questionCount {
		return EndCompleted
	}
	return EndLeft
}

// PracticeInterviewCoins is the price of one practice session, in coins. The team fixed it at 5; it is a
// constant on purpose, not configuration.
const PracticeInterviewCoins int32 = 5

// CreateInput is what the Candidate asks for when starting a session. It is checked by the service
// before the repository sees it.
type CreateInput struct {
	Type          Type
	RoleProfileID *uuid.UUID
	ApplicationID *uuid.UUID
	AvatarID      *uuid.UUID
	VoiceID       *uuid.UUID
	// IdempotencyKey is the client's optional Idempotency-Key. A practice start repeated with the same
	// key returns the first session and charges once. Recruitment starts are already idempotent.
	IdempotencyKey string
}

// CreateParams is CreateInput with the settings the repository needs to start a session: the question
// limit and the practice price. PracticeCoins is zero for recruitment sessions.
type CreateParams struct {
	CandidateID   uuid.UUID
	Type          Type
	RoleProfileID *uuid.UUID
	ApplicationID *uuid.UUID
	AvatarID      *uuid.UUID
	VoiceID       *uuid.UUID
	QuestionLimit int32
	PracticeCoins int32
	// StartKey is the Candidate-scoped idempotency key of a practice start, nil when none was sent.
	StartKey *string
}

// CurrentQuestion is the one question the Candidate sees now. The full bank is never returned. Position
// is its place in the conversation, the number an answer names; QuestionNumber is its core question
// number. It is always a core question: a follow-up exists only in the live socket that asks it.
type CurrentQuestion struct {
	Position       int32  `json:"position"`
	QuestionNumber int32  `json:"question_number"`
	Content        string `json:"content"`
}

// Turn is one answered question of a practice session. Position is its place in the conversation;
// QuestionNumber is the core question it belongs to, so a follow-up shares its core question's number.
type Turn struct {
	Position       int32    `json:"position"`
	QuestionNumber int32    `json:"question_number"`
	Kind           TurnKind `json:"kind" enums:"core,follow_up"`
	Question       string   `json:"question"`
	Answer         string   `json:"answer"`
}

// Session is one interview session as the Candidate sees it. Turns is set only for practice sessions:
// recruitment sessions never return turns, the question list, core question ids, the recording or any
// asset link. Turns is a pointer so a practice session with no answers still returns an empty array,
// and a recruitment session omits the key. EndReason is null while the session runs, and for a session
// ended before the reason was recorded.
type Session struct {
	ID               uuid.UUID        `json:"id"`
	Type             Type             `json:"type" enums:"practice,recruitment"`
	RoleProfileID    *uuid.UUID       `json:"role_profile_id"`
	ApplicationID    *uuid.UUID       `json:"application_id"`
	AvatarID         *uuid.UUID       `json:"avatar_id"`
	VoiceID          *uuid.UUID       `json:"voice_id"`
	StartedAt        time.Time        `json:"started_at"`
	CompletedAt      *time.Time       `json:"completed_at"`
	EndReason        *EndReason       `json:"end_reason" enums:"completed,left,abandoned"`
	EvaluationStatus EvaluationStatus `json:"evaluation_status" enums:"pending,completed,failed"`
	// QuestionCount is the number of core questions and AnsweredCount how many of them are answered, so
	// together they are the session's progress. Follow-ups are not counted.
	QuestionCount int32 `json:"question_count"`
	AnsweredCount int32 `json:"answered_count"`
	// AnsweredTurns is every stored answer, follow-ups included: the last answered position. The live
	// engine's position guard uses it; it is not part of the REST response.
	AnsweredTurns int32 `json:"-"`
	// CurrentQuestion is null when every question is answered or the session is completed.
	CurrentQuestion *CurrentQuestion `json:"current_question"`
	Turns           *[]Turn          `json:"turns,omitempty"`
}

// EndParams asks the one end path to end a session. CandidateID is the owner, also when the System Handler
// ends it. IdleBefore is set only for an abandonment: the session is ended only if its last activity is
// before it, checked again under the session lock.
type EndParams struct {
	CandidateID uuid.UUID
	ID          uuid.UUID
	Cause       EndCause
	IdleBefore  *time.Time
}

// EvaluationMaxAttempts caps how many times one interview's evaluation may be claimed (the first run and
// its retries), which bounds the LLM cost of a session the model cannot score.
const EvaluationMaxAttempts int32 = 3

// EndResult is the session after an end. Ended is true only for the call that ended it now;
// StartEvaluation only for the call that claimed its evaluation.
type EndResult struct {
	Session         Session
	Ended           bool
	StartEvaluation bool
	// EvaluationAttempt is the attempt number the claim used (1 for the first run); 0 when nothing was
	// claimed. The first run is paid by the budget reserved when the session started, a retry is not.
	EvaluationAttempt int32
	// EvaluationExhausted is true when the evaluation needs another run but used all
	// EvaluationMaxAttempts. Only a Candidate completing the session reports it; the live socket and the
	// System Handler just do not start another evaluation.
	EvaluationExhausted bool
}

// SessionRef names a session and its owner, for the System Handler.
type SessionRef struct {
	ID          uuid.UUID
	CandidateID uuid.UUID
}

// AnsweredCore counts the answered core questions among the stored turns; follow-ups are not counted.
func AnsweredCore(turns []Turn) int32 {
	answered := int32(0)
	for _, turn := range turns {
		if turn.Kind != TurnFollowUp {
			answered++
		}
	}
	return answered
}

// TurnCoreQuestion ties one stored turn to the core question of its session it belongs to (KAN-333). It is the
// seam grading uses to group a Candidate's answers per core question: a core turn and every follow-up
// that probed it carry the same QuestionNumber and CoreQuestionID.
//
// QuestionNumber is the position of the core question among the session's assigned questions
// (interview_questions.position, 1..QuestionCount), not the position of the turn in the conversation: a
// follow-up shifts the conversation position, never the question number. CoreQuestionID is the bank
// question assigned at that position. It is internal: no Candidate response carries it.
type TurnCoreQuestion struct {
	Position       int32
	QuestionNumber int32
	Kind           TurnKind
	CoreQuestionID uuid.UUID
}
