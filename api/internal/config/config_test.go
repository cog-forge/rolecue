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
		Auth: AuthConfig{URL: "http://localhost:3000", Timeout: 5 * time.Second, CookiePrefix: "rolecue"},
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

	t.Run("defaults to the team's 50 questions", func(t *testing.T) {
		t.Setenv("QUESTION_BANK_SIZE", "")
		if err := os.Unsetenv("QUESTION_BANK_SIZE"); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(writeConfig(t, minimalYAML))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.QuestionBank.Size != 50 {
			t.Fatalf("default size = %d, want 50", cfg.QuestionBank.Size)
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
