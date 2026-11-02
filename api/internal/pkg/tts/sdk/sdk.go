// Package sdk implements tts.Synthesizer on the Azure Speech SDK for Go, which returns viseme ids,
// blend-shape frames and word boundaries along with the audio. The cgo binding builds only with the
// azure_sdk tag; install steps are in api/docs/azure-speech-sdk-spike.md.
package sdk

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/internal/pkg/tts"
)

const (
	mimeType = "audio/mpeg" // audio-48khz-192kbitrate-mono-mp3
	language = "en-US"

	defaultMaxRetries = 2
	// The SDK reports no Retry-After, and F0 allows 20 requests per minute.
	defaultBackoff  = 500 * time.Millisecond
	maxBackoff      = 4 * time.Second
	defaultEndGrace = time.Second
	maxAudioBytes   = 16 << 20
)

var regionPattern = regexp.MustCompile(`^[a-z0-9]{1,40}$`)

// Config selects the Speech resource. Timeout is the budget for one Synthesize call including retries.
type Config struct {
	Key     string
	Region  string
	Timeout time.Duration
}

// Synthesizer speaks through the Azure Speech SDK. It is safe for concurrent use.
type Synthesizer struct {
	engine     engine
	timeout    time.Duration
	maxRetries int
	backoff    time.Duration
	endGrace   time.Duration
	log        *logger.Logger
}

// Option configures a Synthesizer.
type Option func(*Synthesizer)

// WithRetries sets how many times a retryable failure is retried after the first attempt.
func WithRetries(n int) Option { return func(s *Synthesizer) { s.maxRetries = max(n, 0) } }

// WithBackoff sets the base delay before the first retry; it doubles on each further retry.
func WithBackoff(d time.Duration) Option { return func(s *Synthesizer) { s.backoff = max(d, 0) } }

// WithLogger enables per-request logging of character counts and failures. Text and key are never logged.
func WithLogger(l *logger.Logger) Option { return func(s *Synthesizer) { s.log = l } }

func withEngine(e engine) Option { return func(s *Synthesizer) { s.engine = e } }

func withEndGrace(d time.Duration) Option { return func(s *Synthesizer) { s.endGrace = d } }

// New validates cfg and initializes the native SDK without calling the service. It returns ErrNotBuilt when
// the binary lacks the azure_sdk tag.
func New(cfg Config, opts ...Option) (*Synthesizer, error) {
	s := &Synthesizer{
		timeout:    cfg.Timeout,
		maxRetries: defaultMaxRetries,
		backoff:    defaultBackoff,
		endGrace:   defaultEndGrace,
	}
	for _, opt := range opts {
		opt(s)
	}
	if s.engine == nil && !built {
		return nil, ErrNotBuilt
	}
	if strings.TrimSpace(cfg.Key) == "" || strings.ContainsAny(cfg.Key, "\r\n") {
		return nil, errors.New("azure speech key is required and must not contain newlines")
	}
	if !regionPattern.MatchString(cfg.Region) {
		return nil, errors.New("azure speech region must be a lowercase region name such as koreacentral")
	}
	if cfg.Timeout <= 0 {
		return nil, errors.New("azure speech timeout must be positive")
	}
	if s.engine == nil {
		e, err := newEngine(cfg, s.log)
		if err != nil {
			return nil, err
		}
		s.engine = e
	}
	return s, nil
}

func (s *Synthesizer) Capabilities() tts.Capabilities {
	return tts.Capabilities{Visemes: true, BlendShapes: true, WordBoundaries: true}
}

type attemptError struct {
	err       error
	reason    string // classified for logs, as in the REST adapter
	retryable bool
}

func failureReason(err error) string {
	var ae *attemptError
	switch {
	case errors.Is(err, tts.ErrTimeout):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.As(err, &ae) && ae.reason != "":
		return ae.reason
	}
	return "other"
}

func (e *attemptError) Error() string { return e.err.Error() }
func (e *attemptError) Unwrap() error { return e.err }

func (s *Synthesizer) Synthesize(ctx context.Context, req tts.Request) (tts.Result, error) {
	if err := ctx.Err(); err != nil {
		return tts.Result{}, ctxError(err)
	}
	if err := req.Validate(); err != nil {
		return tts.Result{}, err
	}
	ssml, err := buildSSML(req)
	if err != nil {
		return tts.Result{}, fmt.Errorf("%w: %w", tts.ErrInvalidRequest, err)
	}

	chars := req.CountChars()
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	for attempt := 1; ; attempt++ {
		res, dropped, err := s.attempt(ctx, ssml)
		if err == nil {
			res.Characters = chars
			s.logInfo("tts synthesized",
				logger.String("engine", "sdk"),
				logger.Int("characters", chars),
				logger.Int("audio_bytes", len(res.Audio)),
				logger.Int("visemes", len(res.Visemes)),
				logger.Int("blend_shape_frames", len(res.BlendShapes)),
				logger.Int("dropped_frames", dropped),
				logger.Int("words", len(res.Words)),
				logger.Int("attempts", attempt),
				logger.String("voice", req.ProviderVoiceID),
				logger.Duration("elapsed", time.Since(start)),
			)
			return res, nil
		}

		var ae *attemptError
		retry := errors.As(err, &ae) && ae.retryable && attempt <= s.maxRetries
		s.logWarn("tts attempt failed",
			logger.String("engine", "sdk"),
			logger.Int("characters", chars),
			logger.Int("attempt", attempt),
			logger.String("voice", req.ProviderVoiceID),
			logger.Any("retrying", retry),
			logger.String("reason", failureReason(err)),
			logger.Cause(err),
		)
		if !retry {
			return tts.Result{}, err
		}
		if err := sleep(ctx, s.delay(attempt)); err != nil {
			return tts.Result{}, ctxError(err)
		}
	}
}

func (s *Synthesizer) attempt(ctx context.Context, ssml string) (tts.Result, int, error) {
	events := newCollector()
	audio, release, err := s.engine.speak(ctx, ssml, events)
	if err != nil {
		if ctx.Err() != nil {
			return tts.Result{}, 0, ctxError(ctx.Err())
		}
		return tts.Result{}, 0, speakError(err)
	}
	// Every timeline event precedes the end event, which can lag the result. The audio is complete either way.
	grace := time.NewTimer(s.endGrace)
	select {
	case <-events.done:
	case <-grace.C:
		s.logWarn("tts end event not received; timelines may be incomplete", logger.String("engine", "sdk"))
	case <-ctx.Done():
	}
	grace.Stop()
	release()

	if len(audio) == 0 {
		// Nothing in the text was speakable, such as "...".
		return tts.Result{}, 0, &attemptError{err: fmt.Errorf("%w: provider returned no audio", tts.ErrRejected), reason: "empty_response"}
	}
	if len(audio) > maxAudioBytes {
		return tts.Result{}, 0, &attemptError{err: fmt.Errorf("%w: provider returned %d audio bytes", tts.ErrUnavailable, len(audio)), reason: "response_too_large"}
	}
	t := events.result()
	return tts.Result{
		Audio:       audio,
		MimeType:    mimeType,
		Visemes:     t.visemes,
		BlendShapes: t.blendShapes,
		Words:       t.words,
	}, t.droppedFrames, nil
}

// speakError maps an engine failure; anything but a service cancellation is a local SDK failure.
func speakError(err error) error {
	var ce *canceledError
	if !errors.As(err, &ce) {
		return &attemptError{err: fmt.Errorf("%w: speech sdk: %w", tts.ErrUnavailable, err), reason: "sdk_error"}
	}
	switch ce.code {
	case codeTooManyRequests:
		return &attemptError{err: fmt.Errorf("%w: %w", tts.ErrQuota, ce), reason: "rate_limited", retryable: true}
	case codeConnectionFailure:
		return &attemptError{err: fmt.Errorf("%w: %w", tts.ErrUnavailable, ce), reason: "unreachable", retryable: true}
	case codeServiceTimeout:
		return &attemptError{err: fmt.Errorf("%w: %w", tts.ErrUnavailable, ce), reason: "timeout", retryable: true}
	case codeServiceError, codeServiceUnavailable:
		return &attemptError{err: fmt.Errorf("%w: %w", tts.ErrUnavailable, ce), reason: "service_error", retryable: true}
	case codeAuthenticationFailure, codeForbidden:
		return &attemptError{err: fmt.Errorf("%w: credentials rejected: %w", tts.ErrUnavailable, ce), reason: "auth_rejected"}
	case codeBadRequest:
		return &attemptError{err: fmt.Errorf("%w: %w", tts.ErrRejected, ce), reason: "rejected"}
	default:
		return &attemptError{err: fmt.Errorf("%w: %w", tts.ErrUnavailable, ce), reason: "sdk_error"}
	}
}

func (s *Synthesizer) delay(attempt int) time.Duration {
	return min(s.backoff<<(attempt-1), maxBackoff)
}

func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func ctxError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %w", tts.ErrTimeout, err)
	}
	return err
}

func buildSSML(req tts.Request) (string, error) {
	var b bytes.Buffer
	b.WriteString(`<speak version="1.0" xmlns="http://www.w3.org/2001/10/synthesis" ` +
		`xmlns:mstts="http://www.w3.org/2001/mstts" xml:lang="` + language + `"><voice name="`)
	if err := xml.EscapeText(&b, []byte(req.ProviderVoiceID)); err != nil {
		return "", err
	}
	b.WriteString(`"><mstts:viseme type="FacialExpression"/>`)
	if err := xml.EscapeText(&b, []byte(req.Text)); err != nil {
		return "", err
	}
	b.WriteString(`</voice></speak>`)
	return b.String(), nil
}

func (s *Synthesizer) logInfo(msg string, fields ...zap.Field) {
	if s.log != nil {
		s.log.Info(msg, fields...)
	}
}

func (s *Synthesizer) logWarn(msg string, fields ...zap.Field) {
	if s.log != nil {
		s.log.Warn(msg, fields...)
	}
}
