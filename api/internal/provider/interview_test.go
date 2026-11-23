package provider

import (
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/pkg/ai"
)

func TestFollowUpDeciderSelectsClefOnlyWhenConfigured(t *testing.T) {
	cfg := &config.Config{LLM: config.LLMConfig{Provider: "gemini", APIKey: "fake", Model: "gemini-test", Timeout: 5 * time.Second}}
	decider, err := provideFollowUpDecider(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := decider.(*ai.TaskAdapter); !ok {
		t.Fatalf("without a decision route the chat adapter decides, got %T", decider)
	}
	cfg.AI.CloudflareAccountID = "0123456789abcdef0123456789abcdef"
	cfg.AI.CloudflareAPIToken = "fake-token"
	cfg.AI.FollowUpDecision = config.DecisionRoute{Provider: config.DecisionProviderClef}
	decider, err = provideFollowUpDecider(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := decider.(*ai.ClefFollowUpDecider); !ok {
		t.Fatalf("with a decision route Clef decides, got %T", decider)
	}
}
