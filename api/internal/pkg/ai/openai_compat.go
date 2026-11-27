package ai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/cog-forge/rolecue/api/internal/config"
)

// maxCompatBody bounds one provider response; a longer one is cut and fails to decode as invalid output.
const maxCompatBody = 2 << 20

// HTTPError deliberately excludes the response body, which may echo private input.
type HTTPError struct {
	Status     int
	RetryAfter time.Duration
}

func (e *HTTPError) Error() string { return fmt.Sprintf("LLM HTTP status %d", e.Status) }

// compatModel serves every OpenAI-compatible provider (Groq, Cloudflare Workers AI, OpenRouter) through the
// eino-ext openai component. The component does not expose Retry-After and does not convert error bodies that
// are not JSON, so the HTTP status and Retry-After are read at the transport instead (see compatTransport)
// and a failed call is reported as a bare *HTTPError, never as the library's error (its text embeds the body).
type compatModel struct{ inner *einoopenai.ChatModel }

// newOpenAICompat builds a chat model for an OpenAI-compatible API rooted at cfg.BaseURL. It sends one HTTP
// request per Generate (no SDK retry), never follows a redirect, and asks for a JSON object with low reasoning
// effort, as the Groq adapter it replaces did.
func newOpenAICompat(cfg config.LLMConfig) (model.BaseChatModel, error) {
	client := &http.Client{
		Timeout:       cfg.Timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		Transport:     compatTransport{title: titleFor(cfg.Provider)},
	}
	conf := &einoopenai.ChatModelConfig{
		APIKey:          cfg.APIKey,
		HTTPClient:      client,
		BaseURL:         cfg.BaseURL,
		Model:           cfg.Model,
		ReasoningEffort: einoopenai.ReasoningEffortLevelLow,
		ResponseFormat:  &einoopenai.ChatCompletionResponseFormat{Type: einoopenai.ChatCompletionResponseFormatTypeJSONObject},
	}
	if cfg.MaxOutputTokens > 0 {
		limit := cfg.MaxOutputTokens
		conf.MaxCompletionTokens = &limit
	}
	inner, err := einoopenai.NewChatModel(context.Background(), conf)
	if err != nil {
		return nil, fmt.Errorf("create %s chat model: %w", cfg.Provider, err)
	}
	return &compatModel{inner: inner}, nil
}

func titleFor(provider string) string {
	if provider == "openrouter" {
		return "RoleCue"
	}
	return ""
}

func (m *compatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	note := &responseNote{}
	msg, err := m.inner.Generate(context.WithValue(ctx, noteKey{}, note), input, opts...)
	if note.status != 0 && note.status != http.StatusOK {
		return nil, &HTTPError{Status: note.status, RetryAfter: note.retryAfter}
	}
	if err != nil {
		return nil, compatFailure(err)
	}
	return msg, nil
}

func (m *compatModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("OpenAI-compatible routing supports Generate only")
}

// compatFailure keeps transport failures classifiable (timeout, cancellation, network) and reports everything
// else the library raised, which is a reply it could not decode, as invalid output without its text.
func compatFailure(err error) error {
	var netErr net.Error
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.As(err, &netErr) {
		return err
	}
	return ErrInvalidOutput
}

type noteKey struct{}

// responseNote carries what the transport saw of the response to the Generate call that made the request.
type responseNote struct {
	status     int
	retryAfter time.Duration
}

type compatTransport struct {
	base  http.RoundTripper
	title string
}

func (t compatTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	req = req.Clone(req.Context())
	req.Header.Set("User-Agent", "rolecue/1.0")
	if t.title != "" {
		req.Header.Set("X-Title", t.title)
	}
	resp, err := base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if note, ok := req.Context().Value(noteKey{}).(*responseNote); ok {
		note.status = resp.StatusCode
		note.retryAfter = retryAfterFrom(resp.Header, time.Now())
	}
	resp.Body = struct {
		io.Reader
		io.Closer
	}{io.LimitReader(resp.Body, maxCompatBody), resp.Body}
	return resp, nil
}

// retryAfterFrom reads a Retry-After header, in seconds or as an HTTP date. An absent, unreadable, negative,
// past or non-finite value is zero, so it can never lock a provider out.
func retryAfterFrom(h http.Header, now time.Time) time.Duration {
	value := strings.TrimSpace(h.Get("Retry-After"))
	if value == "" {
		return 0
	}
	if seconds, err := strconv.ParseFloat(value, 64); err == nil {
		if math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds <= 0 {
			return 0
		}
		return time.Duration(math.Min(seconds, 1e6) * float64(time.Second))
	}
	if date, err := http.ParseTime(value); err == nil {
		if wait := date.Sub(now); wait > 0 {
			return wait
		}
	}
	return 0
}
