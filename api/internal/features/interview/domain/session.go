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
)

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
// and a recruitment session omits the key.
type Session struct {
	ID               uuid.UUID        `json:"id"`
	Type             Type             `json:"type" enums:"practice,recruitment"`
	RoleProfileID    *uuid.UUID       `json:"role_profile_id"`
	ApplicationID    *uuid.UUID       `json:"application_id"`
	AvatarID         *uuid.UUID       `json:"avatar_id"`
	VoiceID          *uuid.UUID       `json:"voice_id"`
	StartedAt        time.Time        `json:"started_at"`
	CompletedAt      *time.Time       `json:"completed_at"`
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
