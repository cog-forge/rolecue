package config

import (
	"os"
	"testing"
)

func TestPrivacyConfigDefaultsAndEnvironment(t *testing.T) {
	t.Setenv("PAYOS_CLIENT_ID", "test-client-id")
	t.Setenv("PAYOS_API_KEY", "test-api-key")
	t.Setenv("PAYOS_CHECKSUM_KEY", "test-checksum-key")
	for _, env := range privacyEnv {
		t.Setenv(env, "")
		if err := os.Unsetenv(env); err != nil {
			t.Fatal(err)
		}
	}

	cfg, err := Load(writeConfig(t, minimalYAML))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Retention.Enabled || cfg.Retention.CVDays != 180 || cfg.Retention.TranscriptDays != 365 {
		t.Fatalf("retention defaults: %+v", cfg.Retention)
	}
	if cfg.RecordingConsent.Required {
		t.Fatal("recording consent is not enforced by default")
	}

	t.Setenv("RETENTION_ENABLED", "true")
	t.Setenv("RETENTION_CV_DAYS", "30")
	t.Setenv("RETENTION_TRANSCRIPT_DAYS", "90")
	t.Setenv("RECORDING_CONSENT_REQUIRED", "true")
	cfg, err = Load(writeConfig(t, minimalYAML))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Retention.Enabled || cfg.Retention.CVDays != 30 || cfg.Retention.TranscriptDays != 90 || !cfg.RecordingConsent.Required {
		t.Fatalf("from the environment: %+v %+v", cfg.Retention, cfg.RecordingConsent)
	}
}

func TestRetentionConfigValidate(t *testing.T) {
	if err := (RetentionConfig{}).Validate(); err != nil {
		t.Fatalf("a disabled retention is not checked: %v", err)
	}
	if err := (RetentionConfig{Enabled: true, CVDays: 180, TranscriptDays: 365}).Validate(); err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]RetentionConfig{
		"zero cv":          {Enabled: true, CVDays: 0, TranscriptDays: 365},
		"negative cv":      {Enabled: true, CVDays: -1, TranscriptDays: 365},
		"zero transcript":  {Enabled: true, CVDays: 180, TranscriptDays: 0},
		"too long":         {Enabled: true, CVDays: 180, TranscriptDays: MaxRetentionDays + 1},
		"cv too long":      {Enabled: true, CVDays: MaxRetentionDays + 1, TranscriptDays: 365},
		"both zero, on":    {Enabled: true},
		"negative on only": {Enabled: true, CVDays: 5, TranscriptDays: -3},
	} {
		if err := c.Validate(); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}
