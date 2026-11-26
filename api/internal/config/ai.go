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
	GroqAPIKey string `mapstructure:"groq_api_key"`
	// OpenRouterAPIKey is the OpenRouter credential (KAN-386). Env only, never YAML.
	OpenRouterAPIKey string `mapstructure:"openrouter_api_key"`
	// BedrockAPIKey is an Amazon Bedrock API key (a bearer token) and BedrockRegion the region it is called in
	// (default us-east-1) (KAN-386). Env only, never YAML.
	BedrockAPIKey string    `mapstructure:"bedrock_api_key"`
	BedrockRegion string    `mapstructure:"bedrock_region"`
	JD            TaskRoute `mapstructure:"jd_extraction"`
	Bank          TaskRoute `mapstructure:"question_bank"`
	FollowUp      TaskRoute `mapstructure:"follow_up"`
	Evaluation    TaskRoute `mapstructure:"evaluation"`
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

// awsRegion keeps the region a plain AWS region name, because it becomes part of the API host.
var awsRegion = regexp.MustCompile(`^[a-z]{2}(-[a-z]+)+-[0-9]$`)

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

// Chat providers a task route may name (KAN-386 added cloudflare and openrouter).
const (
	ProviderGemini     = "gemini"
	ProviderGroq       = "groq"
	ProviderCloudflare = "cloudflare"
	ProviderOpenRouter = "openrouter"
	ProviderBedrock    = "bedrock"

	groqBaseURL       = "https://api.groq.com/openai/v1"
	openRouterBaseURL = "https://openrouter.ai/api/v1"
)

// defaultModel is the model a provider uses when a route names none; "" means the provider has no default.
func (c *Config) defaultModel(task Task, provider string) string {
	small := task == TaskFollowUp
	switch provider {
	case ProviderGemini:
		return c.LLM.Model
	case ProviderGroq:
		if small {
			return "openai/gpt-oss-20b"
		}
		return "openai/gpt-oss-120b"
	case ProviderCloudflare:
		if small {
			return "@cf/openai/gpt-oss-20b"
		}
		return "@cf/openai/gpt-oss-120b"
	case ProviderBedrock:
		if small {
			return "openai.gpt-oss-20b-1:0"
		}
		return "openai.gpt-oss-120b-1:0"
	}
	return ""
}

// fillProvider returns one chain entry: base (which carries the task's timeout and retries) with the provider's
// credential, API root and, when model is empty, default model. Every field that belongs to a provider is set
// here for every provider, so an entry never inherits a key or a host from the entry it was copied from.
func (c *Config) fillProvider(task Task, base LLMConfig, provider, model string) LLMConfig {
	f := base
	f.Provider, f.Model, f.BaseURL, f.APIKey = provider, model, "", ""
	switch provider {
	case ProviderGemini:
		f.APIKey = c.LLM.APIKey
	case ProviderGroq:
		f.APIKey, f.BaseURL = c.AI.GroqAPIKey, groqBaseURL
	case ProviderCloudflare:
		f.APIKey = c.AI.CloudflareAPIToken
		f.BaseURL = "https://api.cloudflare.com/client/v4/accounts/" + c.AI.CloudflareAccountID + "/ai/v1"
	case ProviderOpenRouter:
		f.APIKey, f.BaseURL = c.AI.OpenRouterAPIKey, openRouterBaseURL
	case ProviderBedrock:
		f.APIKey, f.BaseURL = c.AI.BedrockAPIKey, "https://bedrock-runtime."+c.bedrockRegion()+".amazonaws.com"
	}
	if f.Model == "" {
		f.Model = c.defaultModel(task, provider)
	}
	return f
}

func (c *Config) bedrockRegion() string {
	if c.AI.BedrockRegion == "" {
		return "us-east-1"
	}
	return c.AI.BedrockRegion
}

func (c *Config) TaskLLM(task Task) (LLMConfig, []LLMConfig) {
	r := c.AI.Route(task)
	provider := c.LLM.Provider
	if r.Provider != "" {
		provider = r.Provider
	}
	primary := c.LLM
	if r.Timeout != 0 {
		primary.Timeout = r.Timeout
	}
	if r.MaxRetries != nil {
		primary.MaxRetries = *r.MaxRetries
	}
	// A route that names no model keeps the legacy model of the legacy provider and takes the default of
	// any other provider.
	model := r.Model
	if model == "" && provider == c.LLM.Provider && provider != ProviderGroq {
		model = c.LLM.Model
	}
	primary = c.fillProvider(task, primary, provider, model)
	var fallback []LLMConfig
	if r.Fallback != "" {
		for _, entry := range strings.Split(r.Fallback, ",") {
			pair := strings.SplitN(strings.TrimSpace(entry), ":", 2)
			name, entryModel := pair[0], ""
			if len(pair) == 2 {
				entryModel = pair[1]
			}
			fallback = append(fallback, c.fillProvider(task, primary, name, entryModel))
		}
	}
	return primary, fallback
}
func bindAI(v *viper.Viper) error {
	for key, env := range map[string]string{
		"ai.groq_api_key":          "GROQ_API_KEY",
		"ai.openrouter_api_key":    "OPENROUTER_API_KEY",
		"ai.bedrock_api_key":       "AWS_BEARER_TOKEN_BEDROCK",
		"ai.bedrock_region":        "BEDROCK_REGION",
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
			switch m.Provider {
			case ProviderGemini, ProviderGroq, ProviderOpenRouter:
			case ProviderCloudflare:
				if !cloudflareAccountID.MatchString(c.AI.CloudflareAccountID) {
					return fmt.Errorf("ai.%s: CLOUDFLARE_ACCOUNT_ID must be a 32-character hex account ID", task)
				}
				if strings.ContainsAny(m.APIKey, " \t") {
					return fmt.Errorf("ai.%s: CLOUDFLARE_AI_API_TOKEN must be one token", task)
				}
			case ProviderBedrock:
				if !awsRegion.MatchString(c.bedrockRegion()) {
					return fmt.Errorf("ai.%s: BEDROCK_REGION must be an AWS region such as us-east-1", task)
				}
			default:
				return fmt.Errorf("ai.%s: provider must be gemini, groq, cloudflare, openrouter or bedrock", task)
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
