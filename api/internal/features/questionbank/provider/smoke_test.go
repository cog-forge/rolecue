//go:build llm_smoke

package provider

import (
	"context"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/ai"
)

// TestLLMSmoke is explicitly opt-in and never compiled in the default test suite.
// It makes ONE real Gemini call with synthetic requirements; never print credentials or headers.
func TestLLMSmoke(t *testing.T) {
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal("invalid configuration; check LLM_TIMEOUT and QUESTION_BANK_SIZE")
	}
	size := cfg.QuestionBank.Size
	if size == 0 {
		t.Skip("QUESTION_BANK_SIZE is 0 (generation disabled)")
	}
	chatModel, err := ai.NewChatModel(context.Background(), cfg.LLM)
	if err != nil {
		t.Fatal("invalid Gemini configuration; check provider, API key, model and timeout")
	}
	generator, err := New(chatModel)
	if err != nil {
		t.Fatal("invalid question generator configuration")
	}
	evidence := "payment platform handling high-volume transactions"
	in := domain.GenerationInput{
		Requirements: []domain.ConfirmedRequirement{
			{Name: "Go", Target: "backend services", Type: "required", EvidenceText: &evidence},
			{Name: "PostgreSQL", Target: "data storage", Type: "required"},
			{Name: "Kafka", Target: "event streaming", Type: "required"},
			{Name: "API design", Target: "REST APIs", Type: "required"},
			{Name: "Automated testing", Target: "quality", Type: "required"},
			{Name: "Docker", Target: "deployment", Type: "preferred"},
		},
		RefinementNote: "Do not ask about Spring Boot.",
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.LLM.Timeout+5*time.Second)
	defer cancel()

	start := time.Now()
	got, err := generator.Generate(ctx, in, size)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Gemini question generation failed after %s; provider details omitted to protect credentials", elapsed.Round(time.Millisecond))
	}

	distinct := map[string]struct{}{}
	longest := 0
	for _, q := range got.Questions {
		distinct[strings.ToLower(strings.TrimSpace(q))] = struct{}{}
		if n := utf8.RuneCountInString(q); n > longest {
			longest = n
		}
	}
	t.Logf("asked for %d questions: got %d, %d distinct, longest %d characters, took %s (LLM_TIMEOUT %s)",
		size, len(got.Questions), len(distinct), longest, elapsed.Round(time.Millisecond), cfg.LLM.Timeout)
	for i, q := range got.Questions {
		if i >= 5 {
			break
		}
		t.Logf("  sample %d: %s", i+1, strings.ReplaceAll(q, cfg.LLM.APIKey, "[REDACTED]"))
	}
	if len(distinct) != size {
		t.Errorf("service validation would reject this: need exactly %d distinct questions, got %d distinct of %d", size, len(distinct), len(got.Questions))
	}
	if longest > domain.MaxQuestionRunes {
		t.Errorf("a question exceeds %d characters", domain.MaxQuestionRunes)
	}
}
