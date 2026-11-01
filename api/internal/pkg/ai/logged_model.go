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
}

func (m *loggedModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	done := logger.Start(ctx, "llm.generate", m.fields(input)...)
	msg, err := m.inner.Generate(ctx, input, opts...)
	done(sanitizedFailure(err), failureFields(err)...)
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
	var apiErr genai.APIError
	if errors.As(err, &apiErr) && apiErr.Code != 0 {
		fields = append(fields, logger.Int("status_code", apiErr.Code))
	}
	return fields
}

func llmFailureReason(err error) string {
	var apiErr genai.APIError
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
	return "other"
}
