package config

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Task string

const (
	TaskJD         Task = "jd_extraction"
	TaskBank       Task = "question_bank"
	TaskFollowUp   Task = "follow_up"
	TaskEvaluation Task = "evaluation"
)

var AITasks = []Task{TaskJD, TaskBank, TaskFollowUp, TaskEvaluation}

type TaskRoute struct {
	Provider   string        `mapstructure:"provider"`
	Model      string        `mapstructure:"model"`
	Timeout    time.Duration `mapstructure:"timeout"`
	MaxRetries *int          `mapstructure:"max_retries"`
	// Ordered provider:model entries. Models may contain slashes.
	Fallback string `mapstructure:"fallback"`
}
type AIConfig struct {
	GroqAPIKey string    `mapstructure:"groq_api_key"`
	JD         TaskRoute `mapstructure:"jd_extraction"`
	Bank       TaskRoute `mapstructure:"question_bank"`
	FollowUp   TaskRoute `mapstructure:"follow_up"`
	Evaluation TaskRoute `mapstructure:"evaluation"`
	// Cloudflare Workers AI credentials for the Clef decision model (KAN-274). Env only, never YAML.
	CloudflareAccountID string `mapstructure:"cloudflare_account_id"`
	CloudflareAPIToken  string `mapstructure:"cloudflare_api_token"`
	// FollowUpDecision puts a decision model in front of the follow-up chat route: it decides whether a
	// follow-up is asked, and the chat route only words it. Empty provider keeps the chat route deciding.
	FollowUpDecision DecisionRoute `mapstructure:"follow_up_decision"`
	// BankCheck holds the quality checks of a generated question bank (KAN-384).
	BankCheck BankCheckConfig `mapstructure:"question_bank_check"`
}

// DecisionRoute configures a decision model. Zero fields take the defaults of FollowUpDecisionRoute.
type DecisionRoute struct {
	Provider string `mapstructure:"provider"`
	Model    string `mapstructure:"model"`
	// Threshold is the yes probability from which a follow-up is asked.
	Threshold float64 `mapstructure:"threshold"`
	// Timeout bounds the decision call; the rest of the live budget is left for the wording or the fallback.
	Timeout time.Duration `mapstructure:"timeout"`
}

const (
	DecisionProviderClef     = "clef"
	defaultDecisionModel     = "clef-flash"
	defaultDecisionThreshold = 0.5
	defaultDecisionTimeout   = 2 * time.Second
	maxDecisionTimeout       = 5 * time.Second
)

var cloudflareAccountID = regexp.MustCompile(`^[0-9a-f]{32}$`)

// FollowUpDecisionRoute returns the follow-up decision route with defaults filled in. Provider "" means off.
func (c AIConfig) FollowUpDecisionRoute() DecisionRoute {
	return c.FollowUpDecision.withDefaults(defaultDecisionModel, defaultDecisionThreshold, defaultDecisionTimeout)
}

// withDefaults fills the zero fields of an enabled decision route. Provider "" means off.
func (r DecisionRoute) withDefaults(model string, threshold float64, timeout time.Duration) DecisionRoute {
	if r.Provider == "" {
		return DecisionRoute{}
	}
	if r.Model == "" {
		r.Model = model
	}
	if r.Threshold == 0 {
		r.Threshold = threshold
	}
	if r.Timeout == 0 {
		r.Timeout = timeout
	}
	return r
}

// validate checks the fields of an enabled decision route; name is its config key for the message.
func (r DecisionRoute) validate(name string, maxTimeout time.Duration) error {
	if r.Provider == "" {
		return nil
	}
	if r.Provider != DecisionProviderClef {
		return fmt.Errorf("ai.%s: provider must be clef", name)
	}
	if r.Model != "clef" && r.Model != "clef-flash" {
		return fmt.Errorf("ai.%s: model must be clef or clef-flash", name)
	}
	if r.Threshold <= 0 || r.Threshold >= 1 {
		return fmt.Errorf("ai.%s: threshold must be between 0 and 1", name)
	}
	if r.Timeout <= 0 || r.Timeout > maxTimeout {
		return fmt.Errorf("ai.%s: timeout must be positive and at most %s", name, maxTimeout)
	}
	return nil
}

// HasCloudflareCredentials reports whether both Cloudflare values are present and well formed.
func (c AIConfig) HasCloudflareCredentials() bool {
	return cloudflareAccountID.MatchString(c.CloudflareAccountID) &&
		strings.TrimSpace(c.CloudflareAPIToken) != "" && !strings.ContainsAny(c.CloudflareAPIToken, " \r\n\t")
}

func (c AIConfig) validateDecision() error {
	r := c.FollowUpDecisionRoute()
	if err := r.validate("follow_up_decision", maxDecisionTimeout); err != nil || r.Provider == "" {
		return err
	}
	if !cloudflareAccountID.MatchString(c.CloudflareAccountID) {
		return fmt.Errorf("ai.follow_up_decision: CLOUDFLARE_ACCOUNT_ID must be a 32-character hex account ID")
	}
	if !c.HasCloudflareCredentials() {
		return fmt.Errorf("ai.follow_up_decision: CLOUDFLARE_AI_API_TOKEN is required and must be one token")
	}
	return nil
}

func (c AIConfig) Route(task Task) TaskRoute {
	switch task {
	case TaskJD:
		return c.JD
	case TaskBank:
		return c.Bank
	case TaskFollowUp:
		return c.FollowUp
	case TaskEvaluation:
		return c.Evaluation
	}
	return TaskRoute{}
}
func (c *Config) TaskLLM(task Task) (LLMConfig, []LLMConfig) {
	r := c.AI.Route(task)
	primary := c.LLM
	if r.Provider != "" {
		primary.Provider = r.Provider
	}
	if r.Model != "" {
		primary.Model = r.Model
	} else if primary.Provider == "groq" {
		primary.Model = "openai/gpt-oss-120b"
		if task == TaskFollowUp {
			primary.Model = "openai/gpt-oss-20b"
		}
	}
	if r.Timeout != 0 {
		primary.Timeout = r.Timeout
	}
	if r.MaxRetries != nil {
		primary.MaxRetries = *r.MaxRetries
	}
	if primary.Provider == "groq" {
		primary.APIKey = c.AI.GroqAPIKey
	}
	var fallback []LLMConfig
	if r.Fallback != "" {
		for _, entry := range strings.Split(r.Fallback, ",") {
			pair := strings.SplitN(strings.TrimSpace(entry), ":", 2)
			f := primary
			f.Provider = pair[0]
			f.Model = ""
			if len(pair) == 2 {
				f.Model = pair[1]
			}
			if f.Provider == "gemini" {
				f.APIKey = c.LLM.APIKey
				if f.Model == "" {
					f.Model = c.LLM.Model
				}
			} else {
				f.APIKey = c.AI.GroqAPIKey
			}
			fallback = append(fallback, f)
		}
	}
	return primary, fallback
}
func bindAI(v *viper.Viper) error {
	for key, env := range map[string]string{
		"ai.groq_api_key":          "GROQ_API_KEY",
		"ai.cloudflare_account_id": "CLOUDFLARE_ACCOUNT_ID",
		"ai.cloudflare_api_token":  "CLOUDFLARE_AI_API_TOKEN",
	} {
		if err := v.BindEnv(key, env); err != nil {
			return err
		}
	}
	for _, field := range []string{"provider", "model", "threshold", "timeout"} {
		key := "ai.follow_up_decision." + field
		if err := v.BindEnv(key, strings.ToUpper(strings.ReplaceAll(key, ".", "_"))); err != nil {
			return err
		}
	}
	if err := bindBankCheck(v); err != nil {
		return err
	}
	for _, task := range AITasks {
		for _, field := range []string{"provider", "model", "timeout", "max_retries", "fallback"} {
			key := "ai." + string(task) + "." + field
			if err := v.BindEnv(key, strings.ToUpper(strings.ReplaceAll(key, ".", "_"))); err != nil {
				return err
			}
		}
	}
	return nil
}
func (c *Config) validateAI() error {
	if err := c.AI.validateDecision(); err != nil {
		return err
	}
	if err := c.AI.BankCheck.validate(); err != nil {
		return err
	}
	for _, task := range AITasks {
		if c.AI.Route(task) == (TaskRoute{}) {
			continue
		} // Preserve legacy optional LLM setup.
		p, fallback := c.TaskLLM(task)
		for _, m := range append([]LLMConfig{p}, fallback...) {
			if m.Provider != "gemini" && m.Provider != "groq" {
				return fmt.Errorf("ai.%s: provider must be gemini or groq", task)
			}
			if strings.TrimSpace(m.Model) == "" || strings.ContainsAny(m.Model, " \r\n\t") {
				return fmt.Errorf("ai.%s: model is required and must be one ID", task)
			}
			if strings.TrimSpace(m.APIKey) == "" || strings.ContainsAny(m.APIKey, "\r\n") {
				return fmt.Errorf("ai.%s: %s API key is required and must not contain newlines", task, m.Provider)
			}
			if m.Timeout <= 0 || m.Timeout > 10*time.Minute || m.MaxRetries < 0 || m.MaxRetries > 3 {
				return fmt.Errorf("ai.%s: timeout must be positive and at most 10m; max_retries must be 0-3", task)
			}
		}
	}
	return nil
}
