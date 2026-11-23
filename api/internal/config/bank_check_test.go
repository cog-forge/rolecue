package config

import (
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestBankCheckDefaultsAndValidation(t *testing.T) {
	c := BankCheckConfig{Topic: DecisionRoute{Provider: "clef"}}
	r := c.TopicRoute()
	if r.Model != "clef" || r.Threshold != 0.5 || r.Timeout != 3*time.Second {
		t.Fatalf("defaults: %+v", r)
	}
	// On by default with clef, batched, threshold 0.5 (KAN-384, product decision 2026-11-18); "off" turns it off.
	if r := (BankCheckConfig{}).TopicRoute(); r.Provider != "clef" || r.Model != "clef" || r.Threshold != 0.5 {
		t.Fatalf("topic check is not on by default: %+v", r)
	}
	if (BankCheckConfig{Topic: DecisionRoute{Provider: TopicCheckOff}}).TopicRoute().Provider != "" {
		t.Fatal("off did not turn the topic check off")
	}
	// The default needs no Cloudflare credentials to start.
	if err := (&Config{}).validateAI(); err != nil {
		t.Fatal(err)
	}
	// Missing Cloudflare credentials never fail startup: the check is fail-open.
	if err := (&Config{AI: AIConfig{BankCheck: c}}).validateAI(); err != nil {
		t.Fatal(err)
	}
	for name, bad := range map[string]BankCheckConfig{
		"duplicate above 1": {DuplicateThreshold: 1.2},
		"duplicate below 0": {DuplicateThreshold: -0.1},
		"provider":          {Topic: DecisionRoute{Provider: "jev"}},
		"model":             {Topic: DecisionRoute{Provider: "clef", Model: "x"}},
		"threshold":         {Topic: DecisionRoute{Provider: "clef", Threshold: 1}},
		"timeout":           {Topic: DecisionRoute{Provider: "clef", Timeout: 11 * time.Second}},
	} {
		if (&Config{AI: AIConfig{BankCheck: bad}}).validateAI() == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}

func TestBankCheckEnvironmentBinding(t *testing.T) {
	t.Setenv("AI_QUESTION_BANK_CHECK_DUPLICATE_THRESHOLD", "0.8")
	t.Setenv("AI_QUESTION_BANK_CHECK_TOPIC_PROVIDER", "clef")
	t.Setenv("AI_QUESTION_BANK_CHECK_TOPIC_THRESHOLD", "0.4")
	t.Setenv("AI_QUESTION_BANK_CHECK_TOPIC_TIMEOUT", "4s")
	v := viper.New()
	if err := bindAI(v); err != nil {
		t.Fatal(err)
	}
	var c Config
	if err := v.Unmarshal(&c); err != nil {
		t.Fatal(err)
	}
	r := c.AI.BankCheck.TopicRoute()
	if c.AI.BankCheck.DuplicateThreshold != 0.8 || r.Provider != "clef" || r.Threshold != 0.4 || r.Timeout != 4*time.Second {
		t.Fatalf("env not loaded: %+v %+v", c.AI.BankCheck, r)
	}
}
