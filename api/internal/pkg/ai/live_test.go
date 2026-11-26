//go:build ai_live

package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/cog-forge/rolecue/api/internal/config"
	ed "github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	id "github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	qd "github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestLiveTasks(t *testing.T) {
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal("config invalid")
	}
	for _, provider := range []string{"groq", "gemini", "cloudflare", "openrouter", "bedrock"} {
		t.Run(provider, func(t *testing.T) {
			if reason := liveSkipReason(cfg, provider); reason != "" {
				t.Skip(reason)
			}
			for _, task := range config.AITasks {
				t.Run(string(task), func(t *testing.T) {
					zero := 0
					timeout := 30 * time.Second
					if task == config.TaskBank {
						timeout = 90 * time.Second
					}
					route := config.TaskRoute{Provider: provider, Model: liveModel(provider), Timeout: timeout, MaxRetries: &zero}
					// A bake-off measures one provider: AI_LIVE_NO_FALLBACK drops Groq's Gemini fallback.
					if provider == "groq" && os.Getenv("AI_LIVE_NO_FALLBACK") == "" {
						route.Fallback = "gemini"
					}
					// A 429 seen by one sample must not put the provider on cooldown for the next one.
					sharedHealth = NewProviderHealth(time.Now)
					cfg.AI.JD = route
					cfg.AI.Bank = route
					cfg.AI.FollowUp = route
					cfg.AI.Evaluation = route
					core, logs := observer.New(zapcore.DebugLevel)
					ctx := logger.ToContext(context.Background(), &logger.Logger{Logger: zap.New(core)})
					adapter, err := NewTaskAdapter(ctx, cfg, task)
					if err != nil {
						t.Fatal("cannot build adapter")
					}
					start := time.Now()
					switch task {
					case config.TaskJD:
						_, err = adapter.Extract(ctx, `We are hiring a Senior Backend Engineer. Required technical skills are Go, PostgreSQL, API design and automated testing. Docker is preferred. Build reliable payment services.`)
					case config.TaskBank:
						_, err = adapter.Generate(ctx, qd.GenerationInput{Seniority: qd.SeniorityLevel{Name: "Senior"}, Requirements: []qd.ConfirmedRequirement{{Name: "Go", Target: "Unspecified", Type: "required"}, {Name: "PostgreSQL", Target: "Unspecified", Type: "required"}, {Name: "API design", Target: "Unspecified", Type: "required"}, {Name: "Automated testing", Target: "Unspecified", Type: "required"}}}, 50)
					case config.TaskFollowUp:
						_, err = adapter.Decide(ctx, id.FollowUpContext{CoreQuestion: "How would you prevent lost updates?", Exchanges: []id.Exchange{{Question: "How would you prevent lost updates?", Answer: "I would use the database."}}})
					case config.TaskEvaluation:
						_, err = adapter.GradeQuestion(ctx, ed.QuestionGradeInput{
							QuestionPosition: 1, RubricName: "Explain a mechanism", ExpectedAnswer: "Transactions group writes so they commit or roll back together.",
							Criteria: []ed.GradeCriterion{{ID: uuid.New(), Key: "C1", MetricID: uuid.New(), MetricName: "Technical accuracy", Name: "Explains the mechanism", MaxPoints: 2, ScoringGuide: "0 = wrong; 1 = partly correct; 2 = correct and complete."}},
							Turns:    []ed.GradeTurn{{Position: 1, Kind: "core", Question: "How do transactions help?", Answer: "They group writes atomically, so either all of them commit or none do."}},
						})
					}
					tokens, attempts, fallbacks := 0, 0, 0
					for _, e := range logs.All() {
						f := e.ContextMap()
						if e.Message == "llm.task_attempt completed" || f["op"] == "llm.task_attempt" {
							if f["event"] == "start" {
								continue
							}
						}
						if f["total_tokens"] != nil {
							tokens += int(f["total_tokens"].(int64))
						}
						if f["op"] == "llm.task_attempt" {
							if _, ok := f["duration"]; ok {
								attempts++
								if f["provider"] != provider {
									fallbacks++
								}
							}
						}
					}
					t.Logf("provider=%s task=%s elapsed=%s tokens=%d attempts=%d fallback=%d reason=%s", provider, task, time.Since(start).Round(time.Millisecond), tokens, attempts, fallbacks, llmFailureReason(err))
					if err != nil {
						t.Errorf("task failed (%s)", llmFailureReason(err))
					}
				})
			}
		})
	}
}

// liveModel is the model a live run names for a provider; "" lets the provider's default apply.
func liveModel(provider string) string {
	if provider == "openrouter" {
		return "openrouter/free"
	}
	return ""
}

// liveSkipReason says why a provider cannot be tried live, or "" when it can. Bedrock is opt-in
// (AI_LIVE_BEDROCK=1) because its account restriction makes every call fail until AWS lifts it.
func liveSkipReason(cfg *config.Config, provider string) string {
	switch provider {
	case "groq":
		if cfg.AI.GroqAPIKey == "" {
			return "GROQ_API_KEY is not set"
		}
	case "gemini":
		if cfg.LLM.APIKey == "" {
			return "LLM_API_KEY is not set"
		}
	case "cloudflare":
		if !cfg.AI.HasCloudflareCredentials() {
			return "CLOUDFLARE_ACCOUNT_ID or CLOUDFLARE_AI_API_TOKEN is not set"
		}
	case "openrouter":
		if cfg.AI.OpenRouterAPIKey == "" {
			return "OPENROUTER_API_KEY is not set"
		}
	case "bedrock":
		if os.Getenv("AI_LIVE_BEDROCK") != "1" {
			return "set AI_LIVE_BEDROCK=1 to try Bedrock live (pending the AWS account approval)"
		}
		if cfg.AI.BedrockAPIKey == "" {
			return "AWS_BEARER_TOKEN_BEDROCK is not set"
		}
	}
	return ""
}

func TestLiveModelIDsAndJSON(t *testing.T) {
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal("invalid config")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.groq.com/openai/v1/models", nil)
	if err != nil {
		t.Fatal("request construction failed")
	}
	req.Header.Set("Authorization", "Bearer "+cfg.AI.GroqAPIKey)
	req.Header.Set("User-Agent", "rolecue/1.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal("model list unavailable")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 {
		t.Fatalf("model list HTTP %d", resp.StatusCode)
	}
	var models struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if json.NewDecoder(resp.Body).Decode(&models) != nil {
		t.Fatal("invalid model list")
	}
	for _, id := range []string{"openai/gpt-oss-120b", "openai/gpt-oss-20b", "qwen/qwen3.8-27b"} {
		found := false
		for _, m := range models.Data {
			if m.ID == id {
				found = true
			}
		}
		if !found {
			t.Fatalf("model missing: %s", id)
		}
		m, err := NewChatModel(ctx, config.LLMConfig{Provider: "groq", APIKey: cfg.AI.GroqAPIKey, Model: id, Timeout: 10 * time.Second})
		if err != nil {
			t.Fatal("model construction failed")
		}
		msg, err := m.Generate(ctx, []*schema.Message{schema.UserMessage(`Return a JSON object with a boolean field ok set to true.`)})
		if err != nil {
			t.Fatalf("%s JSON request failed: %s", id, llmFailureReason(err))
		}
		var value struct {
			OK bool `json:"ok"`
		}
		if json.Unmarshal([]byte(msg.Content), &value) != nil || !value.OK {
			t.Fatalf("%s JSON mode failed", id)
		}
		t.Logf("%s available; JSON mode accepted", id)
	}
}
