package config

import (
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

const fakeAccount = "0123456789abcdef0123456789abcdef"

func TestFollowUpDecisionIsOffWithoutAProvider(t *testing.T) {
	c := Config{LLM: LLMConfig{Provider: "gemini", APIKey: "fake", Model: "legacy", Timeout: 30 * time.Second}}
	if c.AI.FollowUpDecisionRoute().Provider != "" {
		t.Fatal("decision provider enabled by default")
	}
	// No Cloudflare credentials are needed while no route names Clef.
	if err := c.validateAI(); err != nil {
		t.Fatal(err)
	}
}

func TestFollowUpDecisionDefaults(t *testing.T) {
	ai := AIConfig{FollowUpDecision: DecisionRoute{Provider: "clef"}}
	r := ai.FollowUpDecisionRoute()
	if r.Model != "clef-flash" || r.Threshold != 0.5 || r.Timeout != 2*time.Second {
		t.Fatalf("defaults: %+v", r)
	}
	ai.FollowUpDecision = DecisionRoute{Provider: "clef", Model: "clef", Threshold: 0.7, Timeout: time.Second}
	if r := ai.FollowUpDecisionRoute(); r.Model != "clef" || r.Threshold != 0.7 || r.Timeout != time.Second {
		t.Fatalf("overrides: %+v", r)
	}
}

func TestFollowUpDecisionValidation(t *testing.T) {
	valid := AIConfig{CloudflareAccountID: fakeAccount, CloudflareAPIToken: "fake-token", FollowUpDecision: DecisionRoute{Provider: "clef"}}
	if err := (&Config{AI: valid}).validateAI(); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*AIConfig){
		"unknown provider": func(a *AIConfig) { a.FollowUpDecision.Provider = "jev" },
		"unknown model":    func(a *AIConfig) { a.FollowUpDecision.Model = "clef-pro" },
		"threshold high":   func(a *AIConfig) { a.FollowUpDecision.Threshold = 1 },
		"threshold low":    func(a *AIConfig) { a.FollowUpDecision.Threshold = -0.1 },
		"timeout long":     func(a *AIConfig) { a.FollowUpDecision.Timeout = 6 * time.Second },
		"timeout negative": func(a *AIConfig) { a.FollowUpDecision.Timeout = -time.Second },
		"no account":       func(a *AIConfig) { a.CloudflareAccountID = "" },
		"bad account":      func(a *AIConfig) { a.CloudflareAccountID = "../other" },
		"no token":         func(a *AIConfig) { a.CloudflareAPIToken = " " },
		"token newline":    func(a *AIConfig) { a.CloudflareAPIToken = "a\nb" },
	}
	for name, mutate := range cases {
		ai := valid
		mutate(&ai)
		err := (&Config{AI: ai}).validateAI()
		if err == nil {
			t.Fatalf("%s accepted", name)
		}
		if strings.Contains(err.Error(), "fake-token") {
			t.Fatalf("%s: error leaks the token", name)
		}
	}
}

func TestFollowUpDecisionEnvironmentBinding(t *testing.T) {
	t.Setenv("CLOUDFLARE_ACCOUNT_ID", fakeAccount)
	t.Setenv("CLOUDFLARE_AI_API_TOKEN", "fake-token")
	t.Setenv("AI_FOLLOW_UP_DECISION_PROVIDER", "clef")
	t.Setenv("AI_FOLLOW_UP_DECISION_MODEL", "clef")
	t.Setenv("AI_FOLLOW_UP_DECISION_THRESHOLD", "0.6")
	t.Setenv("AI_FOLLOW_UP_DECISION_TIMEOUT", "1500ms")
	// A developer .env may route the chat task; empty values count as unset for this check.
	for _, field := range []string{"PROVIDER", "MODEL", "TIMEOUT", "MAX_RETRIES", "FALLBACK"} {
		t.Setenv("AI_FOLLOW_UP_"+field, "")
	}
	v := viper.New()
	if err := bindAI(v); err != nil {
		t.Fatal(err)
	}
	var c Config
	if err := v.Unmarshal(&c); err != nil {
		t.Fatal(err)
	}
	r := c.AI.FollowUpDecisionRoute()
	if c.AI.CloudflareAccountID != fakeAccount || c.AI.CloudflareAPIToken != "fake-token" || r.Provider != "clef" || r.Model != "clef" || r.Threshold != 0.6 || r.Timeout != 1500*time.Millisecond {
		t.Fatalf("env not loaded: %+v", r)
	}
	// Setting a decision provider does not turn the chat route of the follow-up task on.
	if c.AI.Route(TaskFollowUp) != (TaskRoute{}) {
		t.Fatal("decision route changed the chat route")
	}
}
