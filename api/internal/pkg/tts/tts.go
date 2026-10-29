// Package tts is the vendor-neutral text-to-speech boundary. Interview and preview features depend on
// Synthesizer only; a provider (internal/pkg/tts/azure) hides behind it, so swapping vendors never
// touches them.
package tts

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// MaxTextChars bounds one request. An interviewer utterance is a few hundred characters; the cap keeps a
// runaway prompt from burning the monthly provider quota in a single call.
const MaxTextChars = 5000

// Errors a Synthesizer returns, wrapped with detail. Match them with errors.Is. A caller maps all of them to
// the same behaviour for the interviewer: show the question as text instead of aborting the session.
var (
	// ErrUnavailable means the provider cannot be used right now: not configured, bad credentials, or an
	// outage that survived the retries.
	ErrUnavailable = errors.New("tts: provider unavailable")
	// ErrTimeout means the request did not finish within the synthesizer's time budget.
	ErrTimeout = errors.New("tts: provider timed out")
	// ErrInvalidVoice means the provider voice id is empty or malformed, so no request was sent.
	ErrInvalidVoice = errors.New("tts: invalid provider voice id")
	// ErrInvalidRequest means the text is empty or longer than MaxTextChars, so no request was sent.
	ErrInvalidRequest = errors.New("tts: invalid request")
	// ErrRejected means the provider refused the request, for example an unknown voice or unsupported input.
	// Retrying the same request will not help.
	ErrRejected = errors.New("tts: request rejected by provider")
	// ErrQuota means the provider's rate limit or quota is exhausted.
	ErrQuota = errors.New("tts: provider quota or rate limit exceeded")
)

// Request is one utterance to speak. ProviderVoiceID is the provider's own voice name (for Azure, for example
// "en-US-AvaMultilingualNeural"); mapping a RoleCue Voice Profile to it belongs to the voice catalog.
type Request struct {
	Text            string
	ProviderVoiceID string
}

// Viseme is one mouth shape: ID is the provider viseme number and OffsetMs the time from the start of the
// audio at which it begins.
type Viseme struct {
	OffsetMs int
	ID       int
}

// Result is the synthesized speech. Audio is a complete, playable file of MimeType. Visemes is empty unless
// the synthesizer's Capabilities report Visemes; the client then lip-syncs from the audio signal.
// Characters is the number of characters billed to the provider quota for this call.
type Result struct {
	Audio      []byte
	MimeType   string
	Visemes    []Viseme
	Characters int
}

// Capabilities tell a caller what a Synthesizer's Result can contain.
type Capabilities struct {
	// Visemes reports whether Result.Visemes carries timing. When false the client lip-syncs from amplitude.
	Visemes bool
}

// Synthesizer turns text into speech.
type Synthesizer interface {
	Synthesize(ctx context.Context, req Request) (Result, error)
	Capabilities() Capabilities
}

// voiceIDPattern covers provider voice names such as "en-US-AvaNeural" and "en-US-Ava:DragonHDLatestNeural".
var voiceIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$`)

// Validate rejects a request no provider should be called with. Every Synthesizer applies it first, so the
// fake, the unavailable stub and real providers agree on what is invalid.
func (r Request) Validate() error {
	if !voiceIDPattern.MatchString(r.ProviderVoiceID) {
		return ErrInvalidVoice
	}
	if strings.TrimSpace(r.Text) == "" {
		return fmt.Errorf("%w: text is empty", ErrInvalidRequest)
	}
	if n := utf8.RuneCountInString(r.Text); n > MaxTextChars {
		return fmt.Errorf("%w: text has %d characters, the limit is %d", ErrInvalidRequest, n, MaxTextChars)
	}
	return nil
}

// CountChars is the number of characters a request is billed for.
func (r Request) CountChars() int {
	return utf8.RuneCountInString(r.Text)
}
