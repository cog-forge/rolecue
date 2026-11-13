package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const minimalYAML = "app:\n  name: rolecue\n"

func validConfig() Config {
	return Config{
		App:  AppConfig{Environment: "development"},
		Auth: AuthConfig{URL: "http://localhost:3000", PublicURL: "http://localhost:3000", Timeout: 5 * time.Second, CookiePrefix: "rolecue"},
		PayOS: PayOSConfig{
			ClientID:    "test-client-id",
			APIKey:      "test-api-key",
			ChecksumKey: "test-checksum-key",
		},
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestQuestionBankSizeValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		size int
		ok   bool
	}{
		{"disabled", 0, true},
		{"team decision", 50, true},
		{"typical", 15, true},
		{"maximum", MaxQuestionBankSize, true},
		{"negative", -1, false},
		{"above maximum", MaxQuestionBankSize + 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.QuestionBank.Size = tc.size
			if err := cfg.Validate(); (err == nil) != tc.ok {
				t.Fatalf("Validate() = %v, want ok=%t", err, tc.ok)
			}
		})
	}
}

func TestQuestionBankSizeLoading(t *testing.T) {
	t.Setenv("PAYOS_CLIENT_ID", "test-client-id")
	t.Setenv("PAYOS_API_KEY", "test-api-key")
	t.Setenv("PAYOS_CHECKSUM_KEY", "test-checksum-key")

	t.Run("defaults to 20 questions", func(t *testing.T) {
		t.Setenv("QUESTION_BANK_SIZE", "")
		if err := os.Unsetenv("QUESTION_BANK_SIZE"); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(writeConfig(t, minimalYAML))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.QuestionBank.Size != 20 {
			t.Fatalf("default size = %d, want 20", cfg.QuestionBank.Size)
		}
	})
	t.Run("environment overrides the file", func(t *testing.T) {
		t.Setenv("QUESTION_BANK_SIZE", "12")
		cfg, err := Load(writeConfig(t, "question_bank:\n  size: 5\n"))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.QuestionBank.Size != 12 {
			t.Fatalf("size = %d, want 12 from QUESTION_BANK_SIZE", cfg.QuestionBank.Size)
		}
	})
	t.Run("file value is used without the environment", func(t *testing.T) {
		t.Setenv("QUESTION_BANK_SIZE", "") // registers cleanup; the project treats an empty variable as set
		if err := os.Unsetenv("QUESTION_BANK_SIZE"); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(writeConfig(t, "question_bank:\n  size: 5\n"))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.QuestionBank.Size != 5 {
			t.Fatalf("size = %d, want 5 from the file", cfg.QuestionBank.Size)
		}
	})
	t.Run("invalid value fails loading", func(t *testing.T) {
		t.Setenv("QUESTION_BANK_SIZE", "500")
		if _, err := Load(writeConfig(t, minimalYAML)); err == nil {
			t.Fatal("expected the out-of-range size to fail config validation")
		}
	})
}

func TestPayOSConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		modify      func(c *Config)
		wantErr     bool
		errContains string
	}{
		{
			name:   "valid config",
			modify: func(c *Config) {},
		},
		{
			name: "missing client_id",
			modify: func(c *Config) {
				c.PayOS.ClientID = ""
			},
			wantErr:     true,
			errContains: "payos.client_id is required",
		},
		{
			name: "missing api_key",
			modify: func(c *Config) {
				c.PayOS.APIKey = ""
			},
			wantErr:     true,
			errContains: "payos.api_key is required",
		},
		{
			name: "missing checksum_key",
			modify: func(c *Config) {
				c.PayOS.ChecksumKey = "   "
			},
			wantErr:     true,
			errContains: "payos.checksum_key is required",
		},
		{
			name: "negative timeout",
			modify: func(c *Config) {
				c.PayOS.Timeout = -1 * time.Second
			},
			wantErr:     true,
			errContains: "payos.timeout must not be negative",
		},
		{
			name: "negative max_retries",
			modify: func(c *Config) {
				c.PayOS.MaxRetries = -1
			},
			wantErr:     true,
			errContains: "payos.max_retries must not be negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.modify(&cfg)
			err := cfg.Validate()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errContains)
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("expected error containing %q, got %v", tt.errContains, err)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestDefaultWriteTimeout(t *testing.T) {
	t.Setenv("PAYOS_CLIENT_ID", "test-client-id")
	t.Setenv("PAYOS_API_KEY", "test-api-key")
	t.Setenv("PAYOS_CHECKSUM_KEY", "test-checksum-key")
	cfg, err := Load(writeConfig(t, minimalYAML))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.WriteTimeout != 30*time.Second {
		t.Fatalf("write timeout = %v, want 30s", cfg.Server.WriteTimeout)
	}
}

func TestTTSConfig(t *testing.T) {
	t.Setenv("PAYOS_CLIENT_ID", "test-client-id")
	t.Setenv("PAYOS_API_KEY", "test-api-key")
	t.Setenv("PAYOS_CHECKSUM_KEY", "test-checksum-key")
	unset := func(names ...string) {
		for _, name := range names {
			t.Setenv(name, "")
			if err := os.Unsetenv(name); err != nil {
				t.Fatal(err)
			}
		}
	}

	t.Run("defaults keep en-US voices and start without credentials", func(t *testing.T) {
		unset("AZURE_SPEECH_KEY", "AZURE_SPEECH_REGION", "TTS_TIMEOUT", "TTS_SYNC_LOCALES")
		cfg, err := Load(writeConfig(t, minimalYAML))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.TTS.Key != "" || cfg.TTS.Region != "" || cfg.TTS.Timeout != 10*time.Second ||
			len(cfg.TTS.SyncLocales) != 1 || cfg.TTS.SyncLocales[0] != "en-US" {
			t.Fatalf("TTS defaults = %+v", cfg.TTS)
		}
	})
	t.Run("environment overrides", func(t *testing.T) {
		t.Setenv("AZURE_SPEECH_KEY", "k")
		t.Setenv("AZURE_SPEECH_REGION", "eastus")
		t.Setenv("TTS_TIMEOUT", "5s")
		t.Setenv("TTS_SYNC_LOCALES", "en,fr-FR")
		cfg, err := Load(writeConfig(t, minimalYAML))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.TTS.Key != "k" || cfg.TTS.Region != "eastus" || cfg.TTS.Timeout != 5*time.Second ||
			len(cfg.TTS.SyncLocales) != 2 || cfg.TTS.SyncLocales[1] != "fr-FR" {
			t.Fatalf("TTS from environment = %+v", cfg.TTS)
		}
	})
	t.Run("negative timeout is rejected", func(t *testing.T) {
		cfg := validConfig()
		cfg.TTS.Timeout = -time.Second
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected a negative TTS timeout to fail validation")
		}
	})
}

func TestEmailConfig(t *testing.T) {
	t.Setenv("PAYOS_CLIENT_ID", "test-client-id")
	t.Setenv("PAYOS_API_KEY", "test-api-key")
	t.Setenv("PAYOS_CHECKSUM_KEY", "test-checksum-key")
	names := []string{"EMAIL_PROVIDER", "EMAIL_FROM", "RESEND_API_KEY", "EMAIL_TIMEOUT", "EMAIL_WORKERS", "EMAIL_QUEUE_SIZE", "EMAIL_MAX_ATTEMPTS", "EMAIL_BACKOFF", "EMAIL_MODERATION_APPROVED_TEMPLATE_ID", "EMAIL_MODERATION_REJECTED_TEMPLATE_ID"}
	reset := func() {
		for _, name := range names {
			t.Setenv(name, "")
			if err := os.Unsetenv(name); err != nil {
				t.Fatal(err)
			}
		}
	}

	t.Run("defaults only log and need no credentials", func(t *testing.T) {
		reset()
		cfg, err := Load(writeConfig(t, minimalYAML))
		if err != nil {
			t.Fatal(err)
		}
		want := EmailConfig{Provider: "log", Timeout: 10 * time.Second, Workers: 2, QueueSize: 256, MaxAttempts: 3, Backoff: time.Second}
		if cfg.Email != want {
			t.Fatalf("email defaults = %+v, want %+v", cfg.Email, want)
		}
	})
	t.Run("resend needs a key and a sender", func(t *testing.T) {
		reset()
		t.Setenv("EMAIL_PROVIDER", "resend")
		if _, err := Load(writeConfig(t, minimalYAML)); err == nil {
			t.Fatal("resend without credentials loaded")
		}
		t.Setenv("RESEND_API_KEY", "re_test")
		t.Setenv("EMAIL_FROM", "RoleCue <no-reply@example.test>")
		cfg, err := Load(writeConfig(t, minimalYAML))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Email.Provider != "resend" || cfg.Email.APIKey != "re_test" || cfg.Email.From == "" {
			t.Fatalf("email = %+v", cfg.Email)
		}
	})
	t.Run("loads hosted moderation template IDs", func(t *testing.T) {
		reset()
		t.Setenv("EMAIL_MODERATION_APPROVED_TEMPLATE_ID", "approved-template")
		t.Setenv("EMAIL_MODERATION_REJECTED_TEMPLATE_ID", "rejected-template")
		cfg, err := Load(writeConfig(t, minimalYAML))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Email.ModerationApprovedTemplateID != "approved-template" || cfg.Email.ModerationRejectedTemplateID != "rejected-template" {
			t.Fatal("template env bindings were not loaded")
		}
	})
	t.Run("rejects an unknown provider and out-of-range pool settings", func(t *testing.T) {
		for name, env := range map[string][2]string{
			"unknown provider": {"EMAIL_PROVIDER", "smtp"},
			"zero workers":     {"EMAIL_WORKERS", "0"},
			"too many workers": {"EMAIL_WORKERS", "33"},
			"empty queue":      {"EMAIL_QUEUE_SIZE", "0"},
			"zero attempts":    {"EMAIL_MAX_ATTEMPTS", "0"},
			"negative timeout": {"EMAIL_TIMEOUT", "-1s"},
		} {
			reset()
			t.Setenv(env[0], env[1])
			if _, err := Load(writeConfig(t, minimalYAML)); err == nil {
				t.Fatalf("%s: config loaded", name)
			}
		}
	})
}

func TestInterviewFollowUpConfig(t *testing.T) {
	t.Setenv("PAYOS_CLIENT_ID", "test-client-id")
	t.Setenv("PAYOS_API_KEY", "test-api-key")
	t.Setenv("PAYOS_CHECKSUM_KEY", "test-checksum-key")
	for _, key := range []string{"INTERVIEW_FOLLOW_UP_LIMIT", "INTERVIEW_FOLLOW_UP_TIMEOUT"} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}

	cfg, err := Load(writeConfig(t, minimalYAML))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Interview.FollowUpLimit != 1 || cfg.Interview.FollowUpTimeout != 5*time.Second {
		t.Fatalf("defaults = %d, %s; want 1 follow-up within 5s", cfg.Interview.FollowUpLimit, cfg.Interview.FollowUpTimeout)
	}

	t.Setenv("INTERVIEW_FOLLOW_UP_LIMIT", "0")
	t.Setenv("INTERVIEW_FOLLOW_UP_TIMEOUT", "3s")
	cfg, err = Load(writeConfig(t, minimalYAML))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Interview.FollowUpLimit != 0 || cfg.Interview.FollowUpTimeout != 3*time.Second {
		t.Fatalf("from the environment = %d, %s; want 0 and 3s", cfg.Interview.FollowUpLimit, cfg.Interview.FollowUpTimeout)
	}

	for _, tc := range []struct {
		name    string
		limit   int
		timeout time.Duration
		ok      bool
	}{
		{"off", 0, 0, true},
		{"maximum", MaxInterviewFollowUpLimit, MaxInterviewFollowUpTimeout, true},
		{"negative limit", -1, 0, false},
		{"limit above maximum", MaxInterviewFollowUpLimit + 1, 0, false},
		{"timeout below minimum", 1, 500 * time.Millisecond, false},
		{"timeout above maximum", 1, MaxInterviewFollowUpTimeout + time.Second, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := InterviewConfig{FollowUpLimit: tc.limit, FollowUpTimeout: tc.timeout}.Validate()
			if (err == nil) != tc.ok {
				t.Fatalf("Validate() = %v, want ok=%t", err, tc.ok)
			}
		})
	}
}

func TestAzureSTTProvider(t *testing.T) {
	if err := (InterviewConfig{STTProvider: "azure"}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (InterviewConfig{STTProvider: "unknown"}).Validate(); err == nil {
		t.Fatal("unknown STT provider accepted")
	}
}
