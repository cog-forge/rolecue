package tts

import (
	"context"
	"encoding/binary"
	"sync"
)

const (
	fakeSampleRate = 16000
	// fakeMsPerChar and the bounds below give the fake audio a length that scales with the text, like speech.
	fakeMsPerChar = 60
	fakeMinMs     = 200
	fakeMaxMs     = 60000
)

// Fake is a deterministic Synthesizer for tests and for runs without credentials. It returns a valid,
// silent 16 kHz mono WAV whose length depends only on the text length, and applies the same request
// validation as a real provider. The same request always yields the same bytes.
type Fake struct {
	mu      sync.Mutex
	err     error
	visemes bool
	calls   []Request
}

// FakeOption configures a Fake.
type FakeOption func(*Fake)

// WithFakeVisemes makes the fake emit deterministic visemes and report the Visemes capability, to exercise
// callers that consume timing. Without it the fake mirrors a provider that returns audio only.
func WithFakeVisemes() FakeOption { return func(f *Fake) { f.visemes = true } }

// WithFakeError makes every valid request fail with err, to exercise a caller's fallback path.
func WithFakeError(err error) FakeOption { return func(f *Fake) { f.err = err } }

func NewFake(opts ...FakeOption) *Fake {
	f := &Fake{}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

func (f *Fake) Capabilities() Capabilities { return Capabilities{Visemes: f.visemes} }

// Calls returns the valid requests received so far, oldest first.
func (f *Fake) Calls() []Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Request(nil), f.calls...)
}

func (f *Fake) Synthesize(ctx context.Context, req Request) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if err := req.Validate(); err != nil {
		return Result{}, err
	}
	f.mu.Lock()
	f.calls = append(f.calls, req)
	f.mu.Unlock()
	if f.err != nil {
		return Result{}, f.err
	}

	chars := req.CountChars()
	durationMs := min(max(chars*fakeMsPerChar, fakeMinMs), fakeMaxMs)
	res := Result{
		Audio:      silentWAV(durationMs),
		MimeType:   "audio/wav",
		Characters: chars,
	}
	if f.visemes {
		res.Visemes = fakeVisemes(durationMs)
	}
	return res, nil
}

// fakeVisemes cycles through viseme ids 1-21 every 100 ms and closes on silence (0) at the end.
func fakeVisemes(durationMs int) []Viseme {
	var out []Viseme
	for at, i := 0, 0; at < durationMs; at, i = at+100, i+1 {
		out = append(out, Viseme{OffsetMs: at, ID: i%21 + 1})
	}
	return append(out, Viseme{OffsetMs: durationMs, ID: 0})
}

// silentWAV builds a 16-bit mono PCM WAV of durationMs of silence.
func silentWAV(durationMs int) []byte {
	dataLen := fakeSampleRate * durationMs / 1000 * 2
	buf := make([]byte, 44+dataLen)
	copy(buf[0:], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:], uint32(36+dataLen))
	copy(buf[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(buf[16:], 16)
	binary.LittleEndian.PutUint16(buf[20:], 1) // PCM
	binary.LittleEndian.PutUint16(buf[22:], 1) // mono
	binary.LittleEndian.PutUint32(buf[24:], fakeSampleRate)
	binary.LittleEndian.PutUint32(buf[28:], fakeSampleRate*2)
	binary.LittleEndian.PutUint16(buf[32:], 2)  // block align
	binary.LittleEndian.PutUint16(buf[34:], 16) // bits per sample
	copy(buf[36:], "data")
	binary.LittleEndian.PutUint32(buf[40:], uint32(dataLen))
	return buf
}

// Unavailable is the Synthesizer wired when the provider is not configured. Every call fails with
// ErrUnavailable, so the interview falls back to showing the question as text.
type Unavailable struct{}

func NewUnavailable() Unavailable { return Unavailable{} }

func (Unavailable) Capabilities() Capabilities { return Capabilities{} }

func (Unavailable) Synthesize(ctx context.Context, req Request) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if err := req.Validate(); err != nil {
		return Result{}, err
	}
	return Result{}, ErrUnavailable
}
