package ai

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/gemini"
	"github.com/cloudwego/eino/components/model"
	"google.golang.org/genai"

	"github.com/cog-forge/rolecue/api/internal/config"
)

func NewChatModel(ctx context.Context, cfg config.LLMConfig) (model.BaseChatModel, error) {
	if cfg.Provider != "gemini" {
		return nil, fmt.Errorf("unsupported LLM provider %q: expected gemini", cfg.Provider)
	}
	if strings.TrimSpace(cfg.APIKey) == "" || strings.ContainsAny(cfg.APIKey, "\r\n") {
		return nil, fmt.Errorf("LLM API key is required and must not contain newlines")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, fmt.Errorf("LLM model is required")
	}
	if cfg.Timeout <= 0 {
		return nil, fmt.Errorf("LLM timeout must be positive")
	}
	attempts := int32(1)
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend:    genai.BackendGeminiAPI,
		APIKey:     cfg.APIKey,
		HTTPClient: &http.Client{Timeout: cfg.Timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		HTTPOptions: genai.HTTPOptions{
			Timeout: &cfg.Timeout,
			// The application owns retries; one Generate must make one API attempt.
			RetryOptions: &genai.HTTPRetryOptions{Attempts: &attempts},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create Gemini client: %w", err)
	}
	return gemini.NewChatModel(ctx, &gemini.Config{Client: client, Model: cfg.Model})
}
