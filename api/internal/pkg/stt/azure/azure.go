// Package azure implements synchronous Azure Speech fast transcription without transcoding.
package azure

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"regexp"
	"strings"
	"time"

	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/internal/pkg/stt"
)

const maxResponseBytes = 2 << 20

var regionPattern = regexp.MustCompile(`^[a-z0-9]{1,40}$`)

type Config struct {
	Key     string
	Region  string
	Timeout time.Duration
}

type Client struct {
	key      string
	endpoint string
	timeout  time.Duration
	http     *http.Client
}

type Option func(*Client)

// WithEndpoint replaces the regional endpoint for HTTP tests.
func WithEndpoint(url string) Option { return func(c *Client) { c.endpoint = url } }

func New(cfg Config, opts ...Option) (*Client, error) {
	if strings.TrimSpace(cfg.Key) == "" || strings.ContainsAny(cfg.Key, "\r\n") {
		return nil, errors.New("azure STT requires AZURE_SPEECH_KEY without newlines")
	}
	if !regionPattern.MatchString(cfg.Region) {
		return nil, errors.New("azure STT requires AZURE_SPEECH_REGION as a lowercase region name")
	}
	if cfg.Timeout <= 0 {
		return nil, errors.New("azure STT timeout must be positive")
	}
	c := &Client{key: cfg.Key, endpoint: "https://" + cfg.Region + ".api.cognitive.microsoft.com/speechtotext/transcriptions:transcribe?api-version=2024-11-15", timeout: cfg.Timeout, http: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

type classifiedError struct {
	err    error
	reason string
}

func (e *classifiedError) Error() string     { return e.err.Error() + ": " + e.reason }
func (e *classifiedError) Unwrap() error     { return e.err }
func failure(err error, reason string) error { return &classifiedError{err: err, reason: reason} }
func failureReason(err error) string {
	var classified *classifiedError
	switch {
	case err == nil:
		return ""
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.As(err, &classified):
		return classified.reason
	default:
		return "other"
	}
}

// Transcribe makes one attempt inside the caller's deadline and the configured budget. No retry:
// the candidate can answer again on STT_FAILED without extending live-turn latency.
func (c *Client) Transcribe(ctx context.Context, audio stt.Audio) (result stt.Transcript, err error) {
	done := logger.Start(ctx, "stt.transcribe", logger.String("provider", "azure"), logger.Int("audio_bytes", len(audio.Data)))
	defer func() {
		done(err, logger.Duration("audio_duration", result.Duration), logger.String("reason", failureReason(err)))
	}()
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	if err = ctx.Err(); err != nil {
		return result, err
	}
	if len(audio.Data) == 0 {
		return result, failure(stt.ErrNoSpeech, "no_speech")
	}
	switch audio.MimeType {
	case "audio/webm", "audio/ogg", "audio/wav", "audio/mp4", "audio/mpeg":
	default:
		return result, failure(stt.ErrUnsupportedAudio, "unsupported_audio")
	}
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="audio"; filename="answer"`)
	header.Set("Content-Type", audio.MimeType)
	part, err := form.CreatePart(header)
	if err != nil {
		return result, failure(stt.ErrUnavailable, "build_request")
	}
	if _, err = part.Write(audio.Data); err != nil {
		return result, failure(stt.ErrUnavailable, "build_request")
	}
	if err = form.WriteField("definition", `{"locales":["en-US"]}`); err != nil {
		return result, failure(stt.ErrUnavailable, "build_request")
	}
	if err = form.Close(); err != nil {
		return result, failure(stt.ErrUnavailable, "build_request")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, &body)
	if err != nil {
		return result, failure(stt.ErrUnavailable, "build_request")
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	req.Header.Set("Ocp-Apim-Subscription-Key", c.key)
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		return result, failure(stt.ErrUnavailable, "unreachable")
	}
	defer func() { _ = resp.Body.Close() }()
	// Never expose provider error bodies: they can echo sensitive evidence or credentials.
	switch {
	case resp.StatusCode == 400 || resp.StatusCode == 415:
		return result, failure(stt.ErrUnsupportedAudio, "unsupported_audio")
	case resp.StatusCode == 401 || resp.StatusCode == 403:
		return result, failure(stt.ErrUnavailable, "auth_rejected")
	case resp.StatusCode == 429:
		return result, failure(stt.ErrUnavailable, "rate_limited")
	case resp.StatusCode == 408 || resp.StatusCode == 504:
		return result, failure(stt.ErrUnavailable, "timeout")
	case resp.StatusCode >= 500:
		return result, failure(stt.ErrUnavailable, "http_5xx")
	case resp.StatusCode != 200:
		return result, failure(stt.ErrUnavailable, "http_error")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		return result, failure(stt.ErrUnavailable, "read_error")
	}
	var decoded struct {
		DurationMilliseconds *int64 `json:"durationMilliseconds"`
		CombinedPhrases      []struct {
			Text string `json:"text"`
		} `json:"combinedPhrases"`
	}
	if len(data) > maxResponseBytes || json.Unmarshal(data, &decoded) != nil || decoded.DurationMilliseconds == nil || *decoded.DurationMilliseconds < 0 || *decoded.DurationMilliseconds > int64((1<<63-1)/time.Millisecond) {
		return result, failure(stt.ErrUnavailable, "invalid_response")
	}
	result.Duration = time.Duration(*decoded.DurationMilliseconds) * time.Millisecond
	var phrases []string
	for _, phrase := range decoded.CombinedPhrases {
		if text := strings.TrimSpace(phrase.Text); text != "" {
			phrases = append(phrases, text)
		}
	}
	result.Text = strings.Join(phrases, " ")
	if result.Text == "" {
		return result, failure(stt.ErrNoSpeech, "no_speech")
	}
	return result, nil
}
