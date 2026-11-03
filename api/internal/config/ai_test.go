package config

import (
	"github.com/spf13/viper"
	"testing"
	"time"
)

func TestTaskDefaultsAndOverrides(t *testing.T) {
	c := Config{LLM: LLMConfig{Provider: "gemini", APIKey: "fake", Model: "legacy", Timeout: 30 * time.Second, MaxRetries: 1}}
	for _, task := range AITasks {
		p, f := c.TaskLLM(task)
		if p != c.LLM || len(f) != 0 {
			t.Fatal("legacy changed", task)
		}
	}
	zero := 0
	c.AI.GroqAPIKey = "fake-groq"
	c.AI.FollowUp = TaskRoute{Provider: "groq", MaxRetries: &zero, Fallback: "gemini"}
	p, f := c.TaskLLM(TaskFollowUp)
	if p.Model != "openai/gpt-oss-20b" || p.APIKey != "fake-groq" || p.MaxRetries != 0 || f[0].Model != "legacy" {
		t.Fatal("route incorrect")
	}
	if err := c.validateAI(); err != nil {
		t.Fatal(err)
	}
	for _, r := range []TaskRoute{{Provider: "bedrock"}, {Provider: "groq", Timeout: -1}, {Provider: "groq", Fallback: "unknown:x"}} {
		c.AI.JD = r
		if c.validateAI() == nil {
			t.Fatal("bad route accepted")
		}
	}
}
func TestTaskEnvironmentBinding(t *testing.T) {
	t.Setenv("AI_JD_EXTRACTION_PROVIDER", "groq")
	t.Setenv("AI_JD_EXTRACTION_MAX_RETRIES", "0")
	t.Setenv("AI_JD_EXTRACTION_TIMEOUT", "12s")
	t.Setenv("GROQ_API_KEY", "fake")
	v := viper.New()
	if err := bindAI(v); err != nil {
		t.Fatal(err)
	}
	var c Config
	if err := v.Unmarshal(&c); err != nil {
		t.Fatal(err)
	}
	if c.AI.JD.Provider != "groq" || c.AI.JD.MaxRetries == nil || *c.AI.JD.MaxRetries != 0 || c.AI.JD.Timeout != 12*time.Second || c.AI.GroqAPIKey != "fake" {
		t.Fatal("env not loaded")
	}
}
