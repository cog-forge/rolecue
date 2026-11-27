package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/cog-forge/rolecue/api/internal/config"
)

// bedrockModel calls the Amazon Bedrock Converse API over HTTPS with a Bedrock API key (a bearer token). It does
// not use the AWS SDK: the key needs no signing, and a hand-written call keeps the same guarantees as the other
// adapters: one request per Generate, no redirect, a bare *HTTPError on failure and no provider text in errors.
// Text goes in and text comes out; the prompts already ask for JSON, as Converse has no JSON mode.
type bedrockModel struct {
	key, model, baseURL string
	maxTokens           int
	client              *http.Client
}

func newBedrock(cfg config.LLMConfig) (model.BaseChatModel, error) {
	return &bedrockModel{
		key:       cfg.APIKey,
		model:     cfg.Model,
		baseURL:   strings.TrimRight(cfg.BaseURL, "/"),
		maxTokens: cfg.MaxOutputTokens,
		client: &http.Client{
			Timeout:       cfg.Timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}, nil
}

type bedrockBlock struct {
	Text string `json:"text"`
}

type bedrockMessage struct {
	Role    string         `json:"role"`
	Content []bedrockBlock `json:"content"`
}

func (m *bedrockModel) Generate(ctx context.Context, input []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	var system []bedrockBlock
	var messages []bedrockMessage
	for _, msg := range input {
		if msg == nil {
			return nil, errors.New("nil chat message")
		}
		switch msg.Role {
		case schema.System:
			system = append(system, bedrockBlock{Text: msg.Content})
		case schema.User, schema.Assistant:
			// Converse wants alternating roles, so consecutive messages of one role share a message.
			role := string(msg.Role)
			if n := len(messages); n > 0 && messages[n-1].Role == role {
				messages[n-1].Content = append(messages[n-1].Content, bedrockBlock{Text: msg.Content})
				continue
			}
			messages = append(messages, bedrockMessage{Role: role, Content: []bedrockBlock{{Text: msg.Content}}})
		default:
			return nil, fmt.Errorf("unsupported chat message role %q", msg.Role)
		}
	}
	payload := struct {
		System          []bedrockBlock   `json:"system,omitempty"`
		Messages        []bedrockMessage `json:"messages"`
		InferenceConfig *struct {
			MaxTokens int `json:"maxTokens"`
		} `json:"inferenceConfig,omitempty"`
	}{System: system, Messages: messages}
	if m.maxTokens > 0 {
		payload.InferenceConfig = &struct {
			MaxTokens int `json:"maxTokens"`
		}{m.maxTokens}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	endpoint, err := m.endpoint()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+m.key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "rolecue/1.0")
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, &HTTPError{Status: resp.StatusCode, RetryAfter: retryAfterFrom(resp.Header, time.Now())}
	}
	var out struct {
		Output struct {
			Message struct {
				Content []struct {
					Text *string `json:"text"`
				} `json:"content"`
			} `json:"message"`
		} `json:"output"`
		Usage struct {
			Input  int `json:"inputTokens"`
			Output int `json:"outputTokens"`
			Total  int `json:"totalTokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxCompatBody)).Decode(&out); err != nil {
		// A body that stalls past the timeout is the provider being slow; anything else is an unusable reply.
		return nil, compatFailure(err)
	}
	// Reasoning models put their thinking in reasoningContent blocks; only the text blocks are the answer.
	var text strings.Builder
	for _, block := range out.Output.Message.Content {
		if block.Text != nil {
			text.WriteString(*block.Text)
		}
	}
	if text.Len() == 0 {
		return nil, ErrInvalidOutput
	}
	return &schema.Message{Role: schema.Assistant, Content: text.String(), ResponseMeta: &schema.ResponseMeta{
		Usage: &schema.TokenUsage{PromptTokens: out.Usage.Input, CompletionTokens: out.Usage.Output, TotalTokens: out.Usage.Total},
	}}, nil
}

// endpoint builds .../model/<id>/converse with the model ID escaped, colon included (openai.gpt-oss-120b-1%3A0).
func (m *bedrockModel) endpoint() (*url.URL, error) {
	u, err := url.Parse(m.baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Bedrock base URL")
	}
	u.Path = "/model/" + m.model + "/converse"
	u.RawPath = "/model/" + strings.ReplaceAll(url.PathEscape(m.model), ":", "%3A") + "/converse"
	return u, nil
}

func (m *bedrockModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("bedrock routing supports Generate only")
}
