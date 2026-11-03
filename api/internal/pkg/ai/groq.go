package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// HTTPError deliberately excludes the response body, which may echo private input.
type HTTPError struct {
	Status     int
	RetryAfter time.Duration
}

func (e *HTTPError) Error() string { return fmt.Sprintf("LLM HTTP status %d", e.Status) }

type groqModel struct {
	key, model, endpoint string
	client               *http.Client
}

func (m *groqModel) Generate(ctx context.Context, input []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	messages := make([]message, 0, len(input))
	for _, msg := range input {
		if msg == nil {
			return nil, errors.New("nil chat message")
		}
		messages = append(messages, message{string(msg.Role), msg.Content})
	}
	body, err := json.Marshal(struct {
		Model    string    `json:"model"`
		Messages []message `json:"messages"`
		Format   struct {
			Type string `json:"type"`
		} `json:"response_format"`
		Reasoning string `json:"reasoning_effort"`
	}{m.model, messages, struct {
		Type string `json:"type"`
	}{"json_object"}, "low"})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+m.key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "rolecue/1.0")
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		delay, _ := strconv.ParseFloat(resp.Header.Get("Retry-After"), 64)
		retry := time.Duration(delay * float64(time.Second))
		if date, e := http.ParseTime(resp.Header.Get("Retry-After")); e == nil {
			retry = time.Until(date)
		}
		return nil, &HTTPError{Status: resp.StatusCode, RetryAfter: retry}
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			Prompt     int `json:"prompt_tokens"`
			Completion int `json:"completion_tokens"`
			Total      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&out); err != nil {
		return nil, ErrInvalidOutput
	}
	if len(out.Choices) != 1 {
		return nil, ErrInvalidOutput
	}
	return &schema.Message{Role: schema.Assistant, Content: out.Choices[0].Message.Content, ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{PromptTokens: out.Usage.Prompt, CompletionTokens: out.Usage.Completion, TotalTokens: out.Usage.Total}}}, nil
}
func (m *groqModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("groq routing supports Generate only")
}
