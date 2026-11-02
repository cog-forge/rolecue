package runtime

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Transcriber turns one recorded answer into text. It is the engine's speech-to-text boundary: KAN-55
// (local Whisper) implements it, and FakeTranscriber stands in until then. An implementation honours
// ctx (the engine bounds each call), never logs the audio or the transcript (recruitment answers are
// Recruiter-only evidence), and returns ErrNoSpeech for silence and ErrUnsupportedAudio for audio it
// cannot decode. Any other error is reported to the client as STT_FAILED and the Candidate may answer
// again.
type Transcriber interface {
	Transcribe(ctx context.Context, audio Audio) (Transcript, error)
}

// Audio is one complete recording exactly as the browser recorded it. MimeType is normalized to the
// container type without parameters: "audio/webm", "audio/ogg", "audio/mp4" or "audio/wav".
type Audio struct {
	Data     []byte
	MimeType string
}

// Transcript is the recognized text of one answer. An empty Text means no speech. Duration is the length
// of the recording as decoded by the transcriber; zero means unknown. The engine uses it to enforce
// MaxAnswerDuration, because a browser WebM recording usually carries no duration of its own.
type Transcript struct {
	Text     string
	Duration time.Duration
}

var (
	// ErrNoSpeech means the recording holds no recognizable speech.
	ErrNoSpeech = errors.New("stt: no speech")
	// ErrUnsupportedAudio means the recording cannot be decoded.
	ErrUnsupportedAudio = errors.New("stt: unsupported audio")
	// ErrAnswerTooLong means the recording is longer than MaxAnswerDuration. The engine returns it;
	// a Transcriber only reports Duration.
	ErrAnswerTooLong = errors.New("stt: answer too long")
)

// FakeTranscript is the text FakeTranscriber returns for every recording.
const FakeTranscript = "This is a placeholder answer from the development speech-to-text fake."

// FakeTranscriber is a deterministic Transcriber for tests and for runs before a real STT adapter
// exists. It returns FakeTranscript for any non-empty recording and ErrNoSpeech for an empty one.
type FakeTranscriber struct {
	mu       sync.Mutex
	err      error
	text     string
	duration time.Duration
	calls    []Audio
}

// NewFakeTranscriber returns a fake that answers FakeTranscript.
func NewFakeTranscriber() *FakeTranscriber {
	return &FakeTranscriber{text: FakeTranscript}
}

// SetText changes the transcript the fake returns; an empty text makes it report no speech.
func (f *FakeTranscriber) SetText(text string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.text = text
}

// SetDuration changes the recording length the fake reports; zero reports an unknown length.
func (f *FakeTranscriber) SetDuration(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.duration = d
}

// SetError makes every later call fail with err; nil restores normal answers.
func (f *FakeTranscriber) SetError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

// Calls returns the recordings received so far, oldest first.
func (f *FakeTranscriber) Calls() []Audio {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Audio(nil), f.calls...)
}

func (f *FakeTranscriber) Transcribe(ctx context.Context, audio Audio) (Transcript, error) {
	if err := ctx.Err(); err != nil {
		return Transcript{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, audio)
	if f.err != nil {
		return Transcript{}, f.err
	}
	if len(audio.Data) == 0 || f.text == "" {
		return Transcript{}, ErrNoSpeech
	}
	return Transcript{Text: f.text, Duration: f.duration}, nil
}
