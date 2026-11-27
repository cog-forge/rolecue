package config

import (
	"testing"
	"time"

	"github.com/spf13/viper"
)

const testAccountID = "0123456789abcdef0123456789abcdef"

func providersConfig() Config {
	c := Config{LLM: LLMConfig{Provider: "gemini", APIKey: "gemini-key", Model: "gemini-lite", Timeout: 30 * time.Second}}
	c.AI.GroqAPIKey = "groq-key"
	c.AI.CloudflareAccountID = testAccountID
	c.AI.CloudflareAPIToken = "cf-token"
	c.AI.OpenRouterAPIKey = "or-key"
	c.AI.BedrockAPIKey = "br-key"
	return c
}

func TestTaskLLMCloudflare(t *testing.T) {
	c := providersConfig()
	c.AI.JD = TaskRoute{Provider: "cloudflare"}
	c.AI.FollowUp = TaskRoute{Provider: "cloudflare"}
	jd, _ := c.TaskLLM(TaskJD)
	if jd.Provider != "cloudflare" || jd.Model != "@cf/openai/gpt-oss-120b" || jd.APIKey != "cf-token" {
		t.Fatalf("jd = %+v", jd)
	}
	if want := "https://api.cloudflare.com/client/v4/accounts/" + testAccountID + "/ai/v1"; jd.BaseURL != want {
		t.Fatalf("base url = %q", jd.BaseURL)
	}
	followUp, _ := c.TaskLLM(TaskFollowUp)
	if followUp.Model != "@cf/openai/gpt-oss-20b" {
		t.Fatalf("follow-up model = %q", followUp.Model)
	}
}

// Every entry of a chain carries its own credential and base URL: a fallback copied from the primary must
// never send one provider's key to another provider's host.
func TestTaskLLMLongFallback(t *testing.T) {
	c := providersConfig()
	c.AI.Bank = TaskRoute{Provider: "cloudflare", Fallback: "groq:openai/gpt-oss-120b,gemini,openrouter:openrouter/free,cloudflare:@cf/openai/gpt-oss-20b"}
	primary, fallback := c.TaskLLM(TaskBank)
	cfBase := "https://api.cloudflare.com/client/v4/accounts/" + testAccountID + "/ai/v1"
	want := []struct{ provider, model, key, base string }{
		{"cloudflare", "@cf/openai/gpt-oss-120b", "cf-token", cfBase},
		{"groq", "openai/gpt-oss-120b", "groq-key", "https://api.groq.com/openai/v1"},
		{"gemini", "gemini-lite", "gemini-key", ""},
		{"openrouter", "openrouter/free", "or-key", "https://openrouter.ai/api/v1"},
		{"cloudflare", "@cf/openai/gpt-oss-20b", "cf-token", cfBase},
	}
	got := append([]LLMConfig{primary}, fallback...)
	if len(got) != len(want) {
		t.Fatalf("chain length = %d, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].Provider != w.provider || got[i].Model != w.model || got[i].APIKey != w.key || got[i].BaseURL != w.base {
			t.Errorf("entry %d = %+v, want %+v", i, got[i], w)
		}
	}
}

func TestTaskLLMBareFallbackTakesProviderDefaults(t *testing.T) {
	c := providersConfig()
	c.AI.FollowUp = TaskRoute{Provider: "groq", Fallback: "cloudflare,groq"}
	_, fallback := c.TaskLLM(TaskFollowUp)
	if fallback[0].Model != "@cf/openai/gpt-oss-20b" || fallback[1].Model != "openai/gpt-oss-20b" {
		t.Fatalf("fallback = %+v", fallback)
	}
}

func TestValidateNewProviders(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Config)
		valid  bool
	}{
		{"cloudflare complete", func(c *Config) { c.AI.JD = TaskRoute{Provider: "cloudflare"} }, true},
		{"cloudflare missing token", func(c *Config) {
			c.AI.JD = TaskRoute{Provider: "cloudflare"}
			c.AI.CloudflareAPIToken = ""
		}, false},
		{"cloudflare bad account id", func(c *Config) {
			c.AI.JD = TaskRoute{Provider: "cloudflare"}
			c.AI.CloudflareAccountID = "short/../id"
		}, false},
		{"cloudflare only in the fallback, token missing", func(c *Config) {
			c.AI.JD = TaskRoute{Provider: "groq", Fallback: "cloudflare"}
			c.AI.CloudflareAPIToken = ""
		}, false},
		{"openrouter with a model", func(c *Config) { c.AI.JD = TaskRoute{Provider: "openrouter", Model: "openrouter/free"} }, true},
		{"openrouter without a model", func(c *Config) { c.AI.JD = TaskRoute{Provider: "openrouter"} }, false},
		{"openrouter without a key", func(c *Config) {
			c.AI.JD = TaskRoute{Provider: "openrouter", Model: "openrouter/free"}
			c.AI.OpenRouterAPIKey = ""
		}, false},
		{"unknown provider", func(c *Config) { c.AI.JD = TaskRoute{Provider: "bogus"} }, false},
		{"unknown provider in the fallback", func(c *Config) { c.AI.JD = TaskRoute{Provider: "groq", Fallback: "bogus:x"} }, false},
		{"full chain", func(c *Config) {
			c.AI.Bank = TaskRoute{Provider: "cloudflare", Fallback: "groq,gemini,openrouter:openrouter/free"}
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := providersConfig()
			tc.mutate(&c)
			if err := c.validateAI(); (err == nil) != tc.valid {
				t.Fatalf("validateAI() = %v, want valid=%v", err, tc.valid)
			}
		})
	}
}

func TestOpenRouterKeyBinding(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "or-from-env")
	v := viper.New()
	if err := bindAI(v); err != nil {
		t.Fatal(err)
	}
	var c Config
	if err := v.Unmarshal(&c); err != nil {
		t.Fatal(err)
	}
	if c.AI.OpenRouterAPIKey != "or-from-env" {
		t.Fatal("OPENROUTER_API_KEY not loaded")
	}
}

func TestTaskLLMBedrock(t *testing.T) {
	c := providersConfig()
	c.AI.Bank = TaskRoute{Provider: "bedrock"}
	c.AI.FollowUp = TaskRoute{Provider: "bedrock"}
	bank, _ := c.TaskLLM(TaskBank)
	if bank.Provider != "bedrock" || bank.Model != "openai.gpt-oss-120b-1:0" || bank.APIKey != "br-key" || bank.BaseURL != "https://bedrock-runtime.us-east-1.amazonaws.com" {
		t.Fatalf("bank = %+v", bank)
	}
	if followUp, _ := c.TaskLLM(TaskFollowUp); followUp.Model != "openai.gpt-oss-20b-1:0" {
		t.Fatalf("follow-up model = %q", followUp.Model)
	}
	c.AI.BedrockRegion = "ap-southeast-1"
	if bank, _ = c.TaskLLM(TaskBank); bank.BaseURL != "https://bedrock-runtime.ap-southeast-1.amazonaws.com" {
		t.Fatalf("base url = %q", bank.BaseURL)
	}
}

// Bedrock model IDs contain a colon; only the first colon of a fallback entry separates the provider.
func TestTaskLLMBedrockModelKeepsItsColonInAFallback(t *testing.T) {
	c := providersConfig()
	c.AI.Bank = TaskRoute{Provider: "groq", Fallback: "bedrock:openai.gpt-oss-120b-1:0,bedrock"}
	_, fallback := c.TaskLLM(TaskBank)
	if fallback[0].Model != "openai.gpt-oss-120b-1:0" || fallback[1].Model != "openai.gpt-oss-120b-1:0" || fallback[0].APIKey != "br-key" {
		t.Fatalf("fallback = %+v", fallback)
	}
}

func TestValidateBedrock(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Config)
		valid  bool
	}{
		{"complete", func(c *Config) { c.AI.Bank = TaskRoute{Provider: "bedrock"} }, true},
		{"missing key", func(c *Config) {
			c.AI.Bank = TaskRoute{Provider: "bedrock"}
			c.AI.BedrockAPIKey = ""
		}, false},
		{"only in the fallback, key missing", func(c *Config) {
			c.AI.Bank = TaskRoute{Provider: "groq", Fallback: "bedrock"}
			c.AI.BedrockAPIKey = ""
		}, false},
		{"region that is not a region", func(c *Config) {
			c.AI.Bank = TaskRoute{Provider: "bedrock"}
			c.AI.BedrockRegion = "evil.example.com/x"
		}, false},
		{"another region", func(c *Config) {
			c.AI.Bank = TaskRoute{Provider: "bedrock"}
			c.AI.BedrockRegion = "eu-central-1"
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := providersConfig()
			tc.mutate(&c)
			if err := c.validateAI(); (err == nil) != tc.valid {
				t.Fatalf("validateAI() = %v, want valid=%v", err, tc.valid)
			}
		})
	}
}

func TestBedrockEnvironmentBinding(t *testing.T) {
	t.Setenv("AWS_BEARER_TOKEN_BEDROCK", "br-from-env")
	t.Setenv("BEDROCK_REGION", "us-west-2")
	v := viper.New()
	if err := bindAI(v); err != nil {
		t.Fatal(err)
	}
	var c Config
	if err := v.Unmarshal(&c); err != nil {
		t.Fatal(err)
	}
	if c.AI.BedrockAPIKey != "br-from-env" || c.AI.BedrockRegion != "us-west-2" {
		t.Fatal("Bedrock settings not loaded from the environment")
	}
}
