package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// BankCheckConfig holds the quality checks of a generated question bank (KAN-384). Both run inside the
// existing generation and validation path; neither changes the API or a product rule.
type BankCheckConfig struct {
	// DuplicateThreshold is the token similarity from which a question is a near duplicate of an earlier one
	// and is dropped. Zero means the default (0.7); 1 drops only questions with identical informative tokens.
	DuplicateThreshold float64 `mapstructure:"duplicate_threshold"`
	// Topic asks Clef whether each question of a Roadmap bank tests the topic the model assigned it. It is on
	// by default (provider clef) and fail-open: without Cloudflare credentials, or when Clef fails, the bank is
	// accepted as before. Provider "off" turns it off.
	Topic DecisionRoute `mapstructure:"topic"`
}

// TopicCheckOff is the provider value that turns the topic check off.
const TopicCheckOff = "off"

const (
	// defaultTopicModel is clef: in the F4 live check (2026-11-18) packed clef-flash scored a multi-stage
	// Dockerfile question 0.59 under a Go topic, packed clef 0.03 (api/docs/ai-routing.md).
	defaultTopicModel     = "clef"
	defaultTopicThreshold = 0.5
	defaultTopicTimeout   = 3 * time.Second
	maxTopicTimeout       = 10 * time.Second
)

// TopicRoute returns the topic check with defaults filled in. It is on by default: an unset provider means
// clef, and "off" turns it off (an empty route).
func (c BankCheckConfig) TopicRoute() DecisionRoute {
	r := c.Topic
	switch r.Provider {
	case TopicCheckOff:
		return DecisionRoute{}
	case "":
		r.Provider = DecisionProviderClef
	}
	return r.withDefaults(defaultTopicModel, defaultTopicThreshold, defaultTopicTimeout)
}

// validate rejects malformed settings. Missing Cloudflare credentials are not an error here: the topic check
// is fail-open and is switched off at startup with a warning instead.
func (c BankCheckConfig) validate() error {
	if c.DuplicateThreshold < 0 || c.DuplicateThreshold > 1 {
		return fmt.Errorf("ai.question_bank_check: duplicate_threshold must be between 0 and 1")
	}
	return c.TopicRoute().validate("question_bank_check.topic", maxTopicTimeout)
}

func bindBankCheck(v *viper.Viper) error {
	for _, key := range []string{"duplicate_threshold", "topic.provider", "topic.model", "topic.threshold", "topic.timeout"} {
		full := "ai.question_bank_check." + key
		if err := v.BindEnv(full, strings.ToUpper(strings.ReplaceAll(full, ".", "_"))); err != nil {
			return err
		}
	}
	return nil
}
