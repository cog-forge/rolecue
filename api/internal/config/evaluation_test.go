package config

import (
	"math"
	"os"
	"testing"
)

func TestEvaluationWeightTotalValidation(t *testing.T) {
	for _, tc := range []struct {
		name  string
		total float64
		ok    bool
	}{
		{"sum check off", 0, true},
		{"percentages", 100, true},
		{"fractions", 1, true},
		{"maximum", MaxEvaluationWeightTotal, true},
		{"negative", -1, false},
		{"above maximum", MaxEvaluationWeightTotal + 1, false},
		{"not a number", math.NaN(), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Evaluation.WeightTotal = tc.total
			if err := cfg.Validate(); (err == nil) != tc.ok {
				t.Fatalf("Validate() = %v, want ok=%t", err, tc.ok)
			}
		})
	}
}

func TestEvaluationWeightTotalLoading(t *testing.T) {
	t.Setenv("PAYOS_CLIENT_ID", "test-client-id")
	t.Setenv("PAYOS_API_KEY", "test-api-key")
	t.Setenv("PAYOS_CHECKSUM_KEY", "test-checksum-key")

	t.Run("defaults to 100", func(t *testing.T) {
		t.Setenv("EVALUATION_WEIGHT_TOTAL", "") // registers cleanup; the project treats an empty variable as set
		if err := os.Unsetenv("EVALUATION_WEIGHT_TOTAL"); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(writeConfig(t, minimalYAML))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Evaluation.WeightTotal != 100 {
			t.Fatalf("default total = %v, want 100", cfg.Evaluation.WeightTotal)
		}
	})
	t.Run("environment overrides the file", func(t *testing.T) {
		t.Setenv("EVALUATION_WEIGHT_TOTAL", "0")
		cfg, err := Load(writeConfig(t, "evaluation:\n  weight_total: 10\n"))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Evaluation.WeightTotal != 0 {
			t.Fatalf("total = %v, want 0 from EVALUATION_WEIGHT_TOTAL", cfg.Evaluation.WeightTotal)
		}
	})
}
