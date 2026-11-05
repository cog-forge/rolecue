package config

import (
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestRateLimitAndBudgetDefaultsAndEnv(t *testing.T) {
	v := viper.New()
	setRateLimitDefaults(v)
	var c Config
	if err := v.Unmarshal(&c); err != nil {
		t.Fatal(err)
	}
	if !c.RateLimit.Enabled || c.RateLimit.LLMPerMinute != 10 || c.RateLimit.LLMBurst != 5 ||
		c.RateLimit.ExpensivePerMinute != 20 || c.RateLimit.IPPerMinute != 120 || c.RateLimit.IPBurst != 40 {
		t.Fatalf("defaults = %+v", c.RateLimit)
	}
	if c.LLM.UserDailyCallBudget != 200 {
		t.Fatalf("daily call budget default = %d, want 200", c.LLM.UserDailyCallBudget)
	}
	if err := c.RateLimit.Validate(); err != nil {
		t.Fatal(err)
	}

	t.Setenv("RATE_LIMIT_LLM_PER_MINUTE", "3")
	t.Setenv("LLM_USER_DAILY_CALL_BUDGET", "0")
	v = viper.New()
	setRateLimitDefaults(v)
	if err := bindRateLimitEnv(v); err != nil {
		t.Fatal(err)
	}
	var fromEnv Config
	if err := v.Unmarshal(&fromEnv); err != nil {
		t.Fatal(err)
	}
	if fromEnv.RateLimit.LLMPerMinute != 3 || fromEnv.LLM.UserDailyCallBudget != 0 {
		t.Fatalf("env not loaded: %+v budget %d", fromEnv.RateLimit, fromEnv.LLM.UserDailyCallBudget)
	}
}

func TestRateLimitValidate(t *testing.T) {
	good := RateLimitConfig{Enabled: true, LLMPerMinute: 10, LLMBurst: 5}
	if err := good.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, bad := range map[string]RateLimitConfig{
		"negative rate":     {LLMPerMinute: -1},
		"rate on, no burst": {LLMPerMinute: 10, LLMBurst: 0},
		"huge rate":         {IPPerMinute: MaxRateLimitPerMinute + 1, IPBurst: 1},
		"negative burst":    {ExpensivePerMinute: 1, ExpensiveBurst: -1},
		"huge burst":        {ExpensivePerMinute: 1, ExpensiveBurst: MaxRateLimitBurst + 1},
	} {
		if bad.Validate() == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

// Every LLM path builds its model from TaskLLM, so the output-token cap must reach the primary route, the
// fallbacks and the legacy (unrouted) route alike (KAN-278).
func TestMaxOutputTokensReachesEveryRoute(t *testing.T) {
	c := Config{LLM: LLMConfig{Provider: "gemini", APIKey: "k", Model: "m", Timeout: 30 * time.Second, MaxOutputTokens: 1234}}
	c.AI.GroqAPIKey = "g"
	c.AI.Evaluation = TaskRoute{Provider: "groq", Fallback: "gemini,groq:other"}
	for _, task := range AITasks {
		primary, fallback := c.TaskLLM(task)
		if primary.MaxOutputTokens != 1234 {
			t.Errorf("%s primary max output tokens = %d", task, primary.MaxOutputTokens)
		}
		for _, f := range fallback {
			if f.MaxOutputTokens != 1234 {
				t.Errorf("%s fallback %s max output tokens = %d", task, f.Provider, f.MaxOutputTokens)
			}
		}
	}
}
