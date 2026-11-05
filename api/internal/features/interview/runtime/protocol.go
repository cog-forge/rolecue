// Package runtime is the live interview engine (KAN-52). It drives one interview session over a
// WebSocket: it delivers the interviewer's turns with synthesized speech, takes the Candidate's answers,
// transcribes them and stores them through the interview service, and completes the session. The wire
// contract is docs/interview-ws-protocol.md; this file mirrors it.
package runtime

import (
	"encoding/json"
	"time"
)

// Subprotocol is the WebSocket subprotocol of protocol version 1. A client must offer it.
const Subprotocol = "rolecue.interview.v1"

// ProtocolVersion is the v field of every message.
const ProtocolVersion = 1

// Limits of the protocol. docs/interview-ws-protocol.md §6 explains each value.
const (
	MaxMessageBytes     = 512 << 10
	MaxAudioChunkBytes  = 256 << 10
	MaxAnswerAudioBytes = 4 << 20
	MaxAnswerChunks     = 1024
	MaxInterviewerAudio = 3 << 20
	// MaxAnswerDuration is the longest spoken answer (product rule, 2026-10-30). The client stops
	// recording at this length; the server rejects a transcript whose Duration exceeds it plus
	// answerDurationGrace, which absorbs recorder start and stop jitter.
	MaxAnswerDuration    = 3 * time.Minute
	answerDurationGrace  = 5 * time.Second
	maxProtocolErrors    = 8
	rememberedCommandIDs = 256
	pingInterval         = 25 * time.Second
	pongWait             = 60 * time.Second
	writeTimeout         = 10 * time.Second
	verifyTimeout        = 10 * time.Second
	streamRetention      = 30 * time.Minute
	persistTimeout       = 30 * time.Second
	synthesisTimeout     = 20 * time.Second
	defaultSTTTimeout    = 30 * time.Second
	maxCommandIDLength   = 64
	closeReasonMaxBytes  = 120
)

// State is the server state of a live session. The server announces every change.
type State string

const (
	StateInterviewerSpeaking State = "interviewer_speaking"
	StateListening           State = "listening"
	StateProcessing          State = "processing"
	StateTransitioning       State = "transitioning"
	// StateCompleted is the end, however the session ended (docs/interview-ws-protocol.md §3.1). There
	// is no pause state: leaving ends the interview.
	StateCompleted State = "completed"
)

// Server message types.
const (
	TypeSessionSync      = "session.sync"
	TypeState            = "state"
	TypeInterviewerTurn  = "interviewer.turn"
	TypeAnswerAccepted   = "answer.accepted"
	TypeSessionCompleted = "session.completed"
	TypeAck              = "ack"
	TypeError            = "error"
	TypePong             = "pong"
)

// Client message types.
const (
	TypeTurnPlayed  = "turn.played"
	TypeAnswerAudio = "answer.audio"
	TypeAnswerText  = "answer.text"
	TypePing        = "ping"
)

// Close codes besides the standard ones (1000, 1001, 1009, 1011).
const (
	CloseProtocolViolation = 4000
	CloseSuperseded        = 4001
	CloseSessionInvalid    = 4002
	CloseNotEligible       = 4003
)

// Error codes of the error message that are not service codes. Service errors keep their apperror code
// (APPLICATION_NOT_ELIGIBLE, INTERVIEW_TURN_CONFLICT, VALIDATION_ERROR, INTERNAL_ERROR).
const (
	CodeProtocolError          = "PROTOCOL_ERROR"
	CodeInvalidState           = "INVALID_STATE"
	CodeAudioFormatUnsupported = "AUDIO_FORMAT_UNSUPPORTED"
	CodeAudioTooLarge          = "AUDIO_TOO_LARGE"
	CodeAudioChunkOrder        = "AUDIO_CHUNK_ORDER"
	CodeSTTNoSpeech            = "STT_NO_SPEECH"
	CodeSTTFailed              = "STT_FAILED"
	CodeAnswerTooLong          = "ANSWER_TOO_LONG"
	CodeRateLimited            = "RATE_LIMITED"
)

// Audio status of an interviewer turn.
const (
	AudioReady       = "ready"
	AudioUnavailable = "unavailable"
	AudioNoVoice     = "no_voice"
)

// ServerMessage is the envelope of every server message.
type ServerMessage struct {
	V       int    `json:"v"`
	Stream  string `json:"stream"`
	Seq     int64  `json:"seq"`
	Type    string `json:"type"`
	ReplyTo string `json:"reply_to,omitempty"`
	Data    any    `json:"data"`
}

// ClientMessage is the envelope of every client message. Data is decoded per type.
type ClientMessage struct {
	V    int             `json:"v"`
	ID   string          `json:"id,omitempty"`
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// SessionSync opens every connection with the full state. QuestionCount counts core questions;
// AnsweredCount every stored answer (follow-ups included), the last answered position. QuestionNumber is
// the core question the open turn belongs to, for progress; nil like Position when no turn is open.
type SessionSync struct {
	SessionID        string  `json:"session_id"`
	SessionType      string  `json:"session_type"`
	State            State   `json:"state"`
	QuestionCount    int32   `json:"question_count"`
	AnsweredCount    int32   `json:"answered_count"`
	Position         *int32  `json:"position"`
	QuestionNumber   *int32  `json:"question_number"`
	Resumed          bool    `json:"resumed"`
	EvaluationStatus *string `json:"evaluation_status"`
	ServerTime       string  `json:"server_time"`
}

// StateChange announces a server state.
type StateChange struct {
	State    State  `json:"state"`
	Position *int32 `json:"position"`
}

// TurnAudio is the synthesized speech of a turn, inline.
type TurnAudio struct {
	MimeType string `json:"mime_type"`
	Data     []byte `json:"data"`
}

// Viseme is one mouth shape at an offset from the audio start.
type Viseme struct {
	OffsetMs int `json:"offset_ms"`
	ID       int `json:"id"`
}

// BlendShapeFrame is one frame of facial blend-shape values (KAN-248).
type BlendShapeFrame struct {
	OffsetMs int       `json:"offset_ms"`
	Values   []float64 `json:"values"`
}

// WordTiming is one spoken word with its timing (KAN-248).
type WordTiming struct {
	OffsetMs   int    `json:"offset_ms"`
	DurationMs int    `json:"duration_ms"`
	Text       string `json:"text"`
}

// Cue is a bounded emotion or gesture cue (KAN-250).
type Cue struct {
	OffsetMs int    `json:"offset_ms"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
}

// InterviewerTurn is one question as the interviewer performs it. Position is its place in the
// conversation (the number its answer names); QuestionNumber the core question it belongs to, 1..
// QuestionCount, which a follow-up shares with its core question. The arrays are always present, empty
// when there is nothing to perform, so the client schema never changes shape.
type InterviewerTurn struct {
	Position       int32             `json:"position"`
	QuestionNumber int32             `json:"question_number"`
	Kind           string            `json:"kind"`
	QuestionCount  int32             `json:"question_count"`
	Text           string            `json:"text"`
	Audio          *TurnAudio        `json:"audio"`
	AudioStatus    string            `json:"audio_status"`
	Visemes        []Viseme          `json:"visemes"`
	BlendShapes    []BlendShapeFrame `json:"blend_shapes"`
	Words          []WordTiming      `json:"words"`
	Cues           []Cue             `json:"cues"`
}

// AnswerAccepted confirms a stored answer. Transcript is set for practice sessions only: a recruitment
// transcript is never sent to the Candidate.
type AnswerAccepted struct {
	Position      int32   `json:"position"`
	AnsweredCount int32   `json:"answered_count"`
	Duplicate     bool    `json:"duplicate"`
	Transcript    *string `json:"transcript,omitempty"`
}

// SessionCompleted ends the interview; the server closes with 1000 after it.
type SessionCompleted struct {
	EvaluationStatus string `json:"evaluation_status"`
}

// Ack answers a repeated command id.
type Ack struct {
	Duplicate bool `json:"duplicate"`
}

// ErrorData is the error message. Fatal errors are followed by a close.
type ErrorData struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
	Fatal     bool   `json:"fatal"`
}

// TurnPlayed reports that the interviewer turn finished playing.
type TurnPlayed struct {
	Position int32 `json:"position"`
}

// AnswerAudio is one chunk of a recorded answer. Every chunk of one answer shares the message id.
type AnswerAudio struct {
	Position int32  `json:"position"`
	Chunk    int    `json:"chunk"`
	Final    bool   `json:"final"`
	MimeType string `json:"mime_type,omitempty"`
	Audio    []byte `json:"audio"`
}

// AnswerText is a typed answer.
type AnswerText struct {
	Position int32  `json:"position"`
	Text     string `json:"text"`
}

// empty is the data of a message that carries none.
type empty struct{}
