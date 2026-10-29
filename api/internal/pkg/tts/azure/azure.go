// Package azure implements tts.Synthesizer on the Azure AI Speech REST endpoint.
//
// Visemes are not available here. Microsoft documents viseme events only for the Speech SDK
// (VisemeReceived), and the REST synthesis endpoint returns audio only, so Capabilities reports no
// visemes and the client lip-syncs from the audio signal. There is no official Go SDK; the SDK's
// WebSocket protocol is undocumented and is deliberately not reimplemented.
package azure

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/internal/pkg/tts"
)

const (
	// outputFormat is a 24 kHz 48 kbps mono MP3: small, and playable by every browser.
	outputFormat = "audio-24khz-48kbitrate-mono-mp3"
	mimeType     = "audio/mpeg"
	// language is the SSML document language. The interview speaks English only.
	language = "en-US"

	defaultMaxRetries = 2
	defaultBackoff    = 250 * time.Millisecond
	// maxRetryAfter caps how long a Retry-After header can make a call wait.
	maxRetryAfter = 2 * time.Second
	// maxAudioBytes bounds the response body. Ten minutes of this format is about 3.6 MB.
	maxAudioBytes = 16 << 20
)

var regionPattern = regexp.MustCompile(`^[a-z0-9]{1,40}$`)

// Config selects the Speech resource. Timeout is the budget for one Synthesize call including retries.
type Config struct {
	Key     string
	Region  string
	Timeout time.Duration
}

// Client calls the Azure AI Speech REST API. It is safe for concurrent use.
type Client struct {
	key        string
	endpoint   string
	timeout    time.Duration
	maxRetries int
	backoff    time.Duration
	http       *http.Client
	log        *logger.Logger
}

// Option configures a Client.
type Option func(*Client)

// WithEndpoint replaces the regional synthesis URL, for tests.
func WithEndpoint(url string) Option { return func(c *Client) { c.endpoint = url } }

// WithRetries sets how many times a retryable failure is retried after the first attempt.
func WithRetries(n int) Option { return func(c *Client) { c.maxRetries = max(n, 0) } }

// WithBackoff sets the base delay before the first retry; it doubles on each further retry.
func WithBackoff(d time.Duration) Option { return func(c *Client) { c.backoff = max(d, 0) } }

// WithLogger enables per-request logging of character counts and failures. Text and key are never logged.
func WithLogger(l *logger.Logger) Option { return func(c *Client) { c.log = l } }

// New validates cfg and returns a Client.
func New(cfg Config, opts ...Option) (*Client, error) {
	if strings.TrimSpace(cfg.Key) == "" || strings.ContainsAny(cfg.Key, "\r\n") {
		return nil, errors.New("azure speech key is required and must not contain newlines")
	}
	if !regionPattern.MatchString(cfg.Region) {
		return nil, errors.New("azure speech region must be a lowercase region name such as koreacentral")
	}
	if cfg.Timeout <= 0 {
		return nil, errors.New("azure speech timeout must be positive")
	}
	c := &Client{
		key:        cfg.Key,
		endpoint:   "https://" + cfg.Region + ".tts.speech.microsoft.com/cognitiveservices/v1",
		timeout:    cfg.Timeout,
		maxRetries: defaultMaxRetries,
		backoff:    defaultBackoff,
		// The call's context carries the deadline. Redirects are refused so the key never follows one.
		http: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// Capabilities reports audio only: the REST endpoint has no viseme output.
func (c *Client) Capabilities() tts.Capabilities { return tts.Capabilities{} }

// attemptError is one failed attempt, already mapped to a tts error.
type attemptError struct {
	err        error
	retryable  bool
	retryAfter time.Duration
}

func (e *attemptError) Error() string { return e.err.Error() }
func (e *attemptError) Unwrap() error { return e.err }

func (c *Client) Synthesize(ctx context.Context, req tts.Request) (tts.Result, error) {
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

	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	for attempt := 1; ; attempt++ {
		audio, err := c.attempt(ctx, ssml)
		if err == nil {
			chars := req.CountChars()
			c.logInfo("tts synthesized",
				logger.Int("characters", chars),
				logger.Int("audio_bytes", len(audio)),
				logger.Int("attempts", attempt),
				logger.String("voice", req.ProviderVoiceID),
				logger.Duration("elapsed", time.Since(start)),
			)
			return tts.Result{Audio: audio, MimeType: mimeType, Characters: chars}, nil
		}

		var ae *attemptError
		retry := errors.As(err, &ae) && ae.retryable && attempt <= c.maxRetries
		c.logWarn("tts attempt failed",
			logger.Int("characters", req.CountChars()),
			logger.Int("attempt", attempt),
			logger.String("voice", req.ProviderVoiceID),
			logger.Any("retrying", retry),
			logger.Error(err),
		)
		if !retry {
			return tts.Result{}, err
		}
		if err := sleep(ctx, c.delay(attempt, ae.retryAfter)); err != nil {
			return tts.Result{}, ctxError(err)
		}
	}
}

// attempt makes one HTTP call and maps every failure to an *attemptError, a timeout or a cancellation.
func (c *Client) attempt(ctx context.Context, ssml []byte) ([]byte, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(ssml))
	if err != nil {
		return nil, &attemptError{err: fmt.Errorf("%w: build request: %w", tts.ErrUnavailable, err)}
	}
	httpReq.Header.Set("Ocp-Apim-Subscription-Key", c.key)
	httpReq.Header.Set("Content-Type", "application/ssml+xml")
	httpReq.Header.Set("X-Microsoft-OutputFormat", outputFormat)
	httpReq.Header.Set("User-Agent", "rolecue-api")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctxError(ctx.Err())
		}
		return nil, &attemptError{err: fmt.Errorf("%w: %w", tts.ErrUnavailable, err), retryable: true}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		return nil, statusError(resp)
	}

	audio, err := io.ReadAll(io.LimitReader(resp.Body, maxAudioBytes+1))
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctxError(ctx.Err())
		}
		return nil, &attemptError{err: fmt.Errorf("%w: read audio: %w", tts.ErrUnavailable, err), retryable: true}
	}
	if len(audio) == 0 || len(audio) > maxAudioBytes {
		return nil, &attemptError{err: fmt.Errorf("%w: provider returned %d audio bytes", tts.ErrUnavailable, len(audio))}
	}
	return audio, nil
}

// statusError maps a non-200 response. The body is never copied into the error: it can echo the request.
func statusError(resp *http.Response) error {
	code := resp.StatusCode
	switch {
	case code == http.StatusUnauthorized || code == http.StatusForbidden:
		return &attemptError{err: fmt.Errorf("%w: credentials rejected (HTTP %d)", tts.ErrUnavailable, code)}
	case code == http.StatusTooManyRequests:
		return &attemptError{
			err:        fmt.Errorf("%w (HTTP 429)", tts.ErrQuota),
			retryable:  true,
			retryAfter: parseRetryAfter(resp.Header.Get("Retry-After")),
		}
	case code == http.StatusRequestTimeout || code >= 500:
		return &attemptError{err: fmt.Errorf("%w: HTTP %d", tts.ErrUnavailable, code), retryable: true}
	case code >= 400:
		return &attemptError{err: fmt.Errorf("%w (HTTP %d)", tts.ErrRejected, code)}
	default:
		return &attemptError{err: fmt.Errorf("%w: unexpected HTTP %d", tts.ErrUnavailable, code)}
	}
}

// delay is the wait before retry number attempt: exponential from the base backoff, or the provider's
// Retry-After when it asks for longer, capped.
func (c *Client) delay(attempt int, retryAfter time.Duration) time.Duration {
	d := c.backoff << (attempt - 1)
	return min(max(d, retryAfter), maxRetryAfter)
}

func parseRetryAfter(v string) time.Duration {
	secs, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || secs <= 0 {
		return 0
	}
	return time.Duration(secs) * time.Second
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

// ctxError turns a context error into a tts error: a deadline is a timeout, a cancellation stays itself.
func ctxError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %w", tts.ErrTimeout, err)
	}
	return err
}

// buildSSML wraps the text in a one-voice SSML document, escaping both values.
func buildSSML(req tts.Request) ([]byte, error) {
	var b bytes.Buffer
	b.WriteString(`<speak version="1.0" xml:lang="` + language + `"><voice name="`)
	if err := xml.EscapeText(&b, []byte(req.ProviderVoiceID)); err != nil {
		return nil, err
	}
	b.WriteString(`">`)
	if err := xml.EscapeText(&b, []byte(req.Text)); err != nil {
		return nil, err
	}
	b.WriteString(`</voice></speak>`)
	return b.Bytes(), nil
}

func (c *Client) logInfo(msg string, fields ...zap.Field) {
	if c.log != nil {
		c.log.Info(msg, fields...)
	}
}

func (c *Client) logWarn(msg string, fields ...zap.Field) {
	if c.log != nil {
		c.log.Warn(msg, fields...)
	}
}
