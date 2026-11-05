package config

import (
	"fmt"

	"github.com/spf13/viper"
)

// RateLimitConfig configures the API's request limits (KAN-277). Every limiter is an in-memory token
// bucket: "burst" requests are allowed at once, then "per_minute" more refill each minute. The buckets
// live in one process, so with several API instances the effective limit is per instance. A per_minute
// of 0 turns that limiter off; Enabled=false turns all of them off. The env names are RATE_LIMIT_<KEY>,
// for example RATE_LIMIT_LLM_PER_MINUTE.
//
// Defaults are conservative for abuse but well above normal use:
//   - LLM: routes that start an LLM call (role-profile analyze/confirm, job-posting extract, question-bank
//     generate, interview completion). Per user, 10 per minute, burst 5.
//   - Expensive: routes that cost money or heavy work without an LLM (wallet checkout/sync, PDF and
//     avatar uploads, recording uploads). Per user, 20 per minute, burst 10.
//   - IP: the same routes again, per client address (c.ClientIP(), which only believes X-Forwarded-For
//     from server.trusted_proxies), so one address cannot spread abuse over many accounts. 120 per
//     minute, burst 40, which leaves room for several users behind one NAT.
type RateLimitConfig struct {
	Enabled            bool `mapstructure:"enabled"`
	LLMPerMinute       int  `mapstructure:"llm_per_minute"`
	LLMBurst           int  `mapstructure:"llm_burst"`
	ExpensivePerMinute int  `mapstructure:"expensive_per_minute"`
	ExpensiveBurst     int  `mapstructure:"expensive_burst"`
	IPPerMinute        int  `mapstructure:"ip_per_minute"`
	IPBurst            int  `mapstructure:"ip_burst"`
}

// MaxRateLimitPerMinute and MaxRateLimitBurst bound the settings to a sane technical range.
const (
	MaxRateLimitPerMinute = 100000
	MaxRateLimitBurst     = 100000
	// MaxLLMUserDailyCallBudget bounds LLM_USER_DAILY_CALL_BUDGET (the column is a 32-bit integer).
	MaxLLMUserDailyCallBudget = 1000000
)

var rateLimitEnv = map[string]string{
	"rate_limit.enabled":              "RATE_LIMIT_ENABLED",
	"rate_limit.llm_per_minute":       "RATE_LIMIT_LLM_PER_MINUTE",
	"rate_limit.llm_burst":            "RATE_LIMIT_LLM_BURST",
	"rate_limit.expensive_per_minute": "RATE_LIMIT_EXPENSIVE_PER_MINUTE",
	"rate_limit.expensive_burst":      "RATE_LIMIT_EXPENSIVE_BURST",
	"rate_limit.ip_per_minute":        "RATE_LIMIT_IP_PER_MINUTE",
	"rate_limit.ip_burst":             "RATE_LIMIT_IP_BURST",
	"llm.user_daily_call_budget":      "LLM_USER_DAILY_CALL_BUDGET",
}

func setRateLimitDefaults(v *viper.Viper) {
	v.SetDefault("rate_limit.enabled", true)
	v.SetDefault("rate_limit.llm_per_minute", 10)
	v.SetDefault("rate_limit.llm_burst", 5)
	v.SetDefault("rate_limit.expensive_per_minute", 20)
	v.SetDefault("rate_limit.expensive_burst", 10)
	v.SetDefault("rate_limit.ip_per_minute", 120)
	v.SetDefault("rate_limit.ip_burst", 40)
	// 200 LLM operations per user and UTC day; 0 means unlimited (KAN-278).
	v.SetDefault("llm.user_daily_call_budget", 200)
}

func bindRateLimitEnv(v *viper.Viper) error {
	for key, env := range rateLimitEnv {
		if err := v.BindEnv(key, env); err != nil {
			return fmt.Errorf("bind %s: %w", key, err)
		}
	}
	return nil
}

// Validate checks the limits. A limiter that is on needs a burst of at least one.
func (c RateLimitConfig) Validate() error {
	for name, pair := range map[string][2]int{
		"llm":       {c.LLMPerMinute, c.LLMBurst},
		"expensive": {c.ExpensivePerMinute, c.ExpensiveBurst},
		"ip":        {c.IPPerMinute, c.IPBurst},
	} {
		perMinute, burst := pair[0], pair[1]
		if perMinute < 0 || perMinute > MaxRateLimitPerMinute {
			return fmt.Errorf("rate_limit.%s_per_minute must be between 0 and %d", name, MaxRateLimitPerMinute)
		}
		if burst < 0 || burst > MaxRateLimitBurst || (perMinute > 0 && burst < 1) {
			return fmt.Errorf("rate_limit.%s_burst must be between 1 and %d when the limit is on", name, MaxRateLimitBurst)
		}
	}
	return nil
}
