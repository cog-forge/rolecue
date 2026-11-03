package ai

import (
	"context"
	"errors"
	"net"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
	"google.golang.org/genai"

	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
)

// loggedModel logs each model call. It records message counts, durations and a classified failure
// reason, never prompt or response content. Provider error text is not logged because it can echo
// request content, so the failure is reported as a reason plus the HTTP status when one exists.
type loggedModel struct {
	inner    model.BaseChatModel
	provider string
	model    string
	task     string
}

func (m *loggedModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	done := logger.Start(ctx, "llm.generate", m.fields(input)...)
	msg, err := m.inner.Generate(ctx, input, opts...)
	fields := failureFields(err)
	if msg != nil && msg.ResponseMeta != nil && msg.ResponseMeta.Usage != nil {
		u := msg.ResponseMeta.Usage
		fields = append(fields, logger.Int("prompt_tokens", u.PromptTokens), logger.Int("completion_tokens", u.CompletionTokens), logger.Int("total_tokens", u.TotalTokens))
	}
	done(sanitizedFailure(err), fields...)
	if m.task != "" && err != nil {
		err = safeProviderError(err)
	}
	return msg, err
}

func (m *loggedModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	done := logger.Start(ctx, "llm.stream_open", m.fields(input)...)
	reader, err := m.inner.Stream(ctx, input, opts...)
	done(sanitizedFailure(err), failureFields(err)...)
	return reader, err
}

func (m *loggedModel) fields(input []*schema.Message) []zap.Field {
	return []zap.Field{
		logger.String("provider", m.provider),
		logger.String("task", m.task),
		logger.String("model", m.model),
		logger.Int("messages", len(input)),
	}
}

// sanitizedFailure returns an error that carries only the classified reason, so Start logs no
// provider text.
func sanitizedFailure(err error) error {
	if err == nil {
		return nil
	}
	return errors.New("llm call failed: " + llmFailureReason(err))
}

func failureFields(err error) []zap.Field {
	if err == nil {
		return nil
	}
	fields := []zap.Field{logger.String("reason", llmFailureReason(err))}
	var safe *providerFailure
	if errors.As(err, &safe) && safe.status != 0 {
		fields = append(fields, logger.Int("status_code", safe.status))
	}
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		fields = append(fields, logger.Int("status_code", httpErr.Status), zap.Duration("retry_after", httpErr.RetryAfter))
	}
	var apiErr genai.APIError
	if errors.As(err, &apiErr) && apiErr.Code != 0 {
		fields = append(fields, logger.Int("status_code", apiErr.Code))
	}
	return fields
}

func llmFailureReason(err error) string {
	var safe *providerFailure
	if errors.As(err, &safe) {
		return safe.reason
	}
	var apiErr genai.APIError
	var httpErr *HTTPError
	if errors.Is(err, ErrInvalidOutput) {
		return "invalid_output"
	}
	if errors.As(err, &httpErr) {
		return statusReason(httpErr.Status)
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.As(err, &apiErr):
		switch {
		case apiErr.Code == 429:
			return "rate_limited"
		case apiErr.Code == 401 || apiErr.Code == 403:
			return "auth_rejected"
		case apiErr.Code >= 500:
			return "http_5xx"
		case apiErr.Code >= 400:
			return "http_4xx"
		}
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout"
	}
	if errors.As(err, &netErr) {
		return "network"
	}
	return "other"
}

func statusReason(status int) string {
	switch {
	case status == 429:
		return "rate_limited"
	case status == 401 || status == 403:
		return "auth_rejected"
	case status >= 500:
		return "http_5xx"
	default:
		return "http_4xx"
	}
}
func (m *loggedModel) startAttempt(ctx context.Context) func(error, ...zap.Field) {
	return logger.Start(ctx, "llm.task_attempt", m.fields(nil)...)
}

// providerFailure retains classification without retaining an SDK cause/body. Feature adapters can
// log returned errors before the route sees them, so sanitization must happen at this boundary.
type providerFailure struct {
	reason string
	status int
}

func (e *providerFailure) Error() string { return "llm call failed: " + e.reason }
func (e *providerFailure) Is(target error) bool {
	return (target == context.DeadlineExceeded && e.reason == "timeout") || (target == context.Canceled && e.reason == "canceled")
}
func safeProviderError(err error) error {
	status := 0
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		status = httpErr.Status
	}
	var geminiErr genai.APIError
	if errors.As(err, &geminiErr) {
		status = geminiErr.Code
	}
	return &providerFailure{reason: llmFailureReason(err), status: status}
}
