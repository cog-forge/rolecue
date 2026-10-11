package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const minimalYAML = "app:\n  name: rolecue\n"

func validConfig() Config {
	return Config{
		App:  AppConfig{Environment: "development"},
		Auth: AuthConfig{URL: "http://localhost:3000", Timeout: 5 * time.Second, CookiePrefix: "rolecue"},
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
