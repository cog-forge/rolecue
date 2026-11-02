package domain

import "time"

// TurnKind tells a core question from a bounded follow-up. Only core questions exist until KAN-56 adds
// follow-ups; the kind is stored on every turn so history can tell them apart later.
type TurnKind string

const (
	TurnCore     TurnKind = "core"
	TurnFollowUp TurnKind = "follow_up"
)

// AskedQuestion is the question the live interviewer asks now. Text is what the interviewer speaks,
// frozen on the first delivery, so a reconnect repeats exactly the same words and the stored turn keeps
// them. It never carries the core question id or any other bank data.
type AskedQuestion struct {
	Position int32
	Kind     TurnKind
	Text     string
	AskedAt  time.Time
}

// Delivery is the live engine's view of a session at a turn boundary. Question is nil when every
// question is answered: the session is then ready to complete. ProviderVoiceID is the provider voice the
// session speaks with, frozen on the first delivery; it is empty when the session has no provider voice
// and no default is configured, and the interviewer is then shown as text only.
type Delivery struct {
	Type            Type
	QuestionCount   int32
	AnsweredCount   int32
	Question        *AskedQuestion
	ProviderVoiceID string
}
