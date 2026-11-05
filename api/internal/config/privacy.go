package config

import (
	"fmt"

	"github.com/spf13/viper"
)

// RetentionConfig configures the personal-data retention passes of the System Handler (KAN-281). It is OFF
// by default. Recording retention (90 days after the final decision) is a product rule and not configured here.
type RetentionConfig struct {
	// Enabled is RETENTION_ENABLED. When on, CV files of Applications rejected at the CV stage are deleted
	// CVDays after the decision, and interview transcripts are cleared TranscriptDays after the interview.
	Enabled bool `mapstructure:"enabled"`
	// CVDays is RETENTION_CV_DAYS (default 180).
	CVDays int `mapstructure:"cv_days"`
	// TranscriptDays is RETENTION_TRANSCRIPT_DAYS (default 365).
	TranscriptDays int `mapstructure:"transcript_days"`
}

// RecordingConsentConfig configures the recording consent gate (KAN-297).
type RecordingConsentConfig struct {
	// Required is RECORDING_CONSENT_REQUIRED (default false). When on, no recording upload is granted for an
	// interview whose Candidate has not stored a consent.
	Required bool `mapstructure:"required"`
}

const (
	MinRetentionDays = 1
	MaxRetentionDays = 3650
)

var privacyEnv = map[string]string{
	"retention.enabled":          "RETENTION_ENABLED",
	"retention.cv_days":          "RETENTION_CV_DAYS",
	"retention.transcript_days":  "RETENTION_TRANSCRIPT_DAYS",
	"recording_consent.required": "RECORDING_CONSENT_REQUIRED",
}

func setPrivacyDefaults(v *viper.Viper) {
	v.SetDefault("retention.enabled", false)
	v.SetDefault("retention.cv_days", 180)
	v.SetDefault("retention.transcript_days", 365)
	v.SetDefault("recording_consent.required", false)
}

func bindPrivacyEnv(v *viper.Viper) error {
	for key, env := range privacyEnv {
		if err := v.BindEnv(key, env); err != nil {
			return fmt.Errorf("bind %s: %w", key, err)
		}
	}
	return nil
}

// Validate checks the day counts when retention is on; while it is off nothing is read, so nothing is checked.
func (c RetentionConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.CVDays < MinRetentionDays || c.CVDays > MaxRetentionDays {
		return fmt.Errorf("retention.cv_days must be between %d and %d", MinRetentionDays, MaxRetentionDays)
	}
	if c.TranscriptDays < MinRetentionDays || c.TranscriptDays > MaxRetentionDays {
		return fmt.Errorf("retention.transcript_days must be between %d and %d", MinRetentionDays, MaxRetentionDays)
	}
	return nil
}
