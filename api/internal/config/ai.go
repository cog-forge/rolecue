package config

import (
	"fmt"
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
	if err := v.BindEnv("ai.groq_api_key", "GROQ_API_KEY"); err != nil {
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
