package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// JobsConfig configures the System Handler's background jobs (KAN-174). Enabled switches every job off
// at once; each job also has its own switch and interval. The env names are JOBS_<KEY> with dots as
// underscores, for example JOBS_ABANDONED_SESSIONS_TIMEOUT.
type JobsConfig struct {
	Enabled bool `mapstructure:"enabled"`
	// AbandonedSessions ends unfinished interviews with no activity for Timeout (KAN-180). The default of
	// 15 minutes settles Brain Decision 16.5 (team decision 2026-11-03).
	AbandonedSessions AbandonedSessionsJobConfig `mapstructure:"abandoned_sessions"`
	// StaleRecordings fails a completed recruitment interview's recording still none Grace after
	// completion, or still uploading past the upload grant's lifetime (KAN-272).
	StaleRecordings StaleRecordingsJobConfig `mapstructure:"stale_recordings"`
	// RecordingRetention deletes recordings 90 days after the Application's final decision (KAN-272).
	RecordingRetention JobConfig `mapstructure:"recording_retention"`
	// RoadmapCatchup links evaluated practice results that the evaluation hook did not link to their Roadmap
	// topics (KAN-351). Each run examines at most BatchSize sessions, oldest first.
	RoadmapCatchup RoadmapCatchupJobConfig `mapstructure:"roadmap_catchup"`
}

// JobConfig is the switch and the interval of one job.
type JobConfig struct {
	Enabled  bool          `mapstructure:"enabled"`
	Interval time.Duration `mapstructure:"interval"`
}

type AbandonedSessionsJobConfig struct {
	JobConfig `mapstructure:",squash"`
	Timeout   time.Duration `mapstructure:"timeout"`
}

type RoadmapCatchupJobConfig struct {
	JobConfig `mapstructure:",squash"`
	BatchSize int `mapstructure:"batch_size"`
}

type StaleRecordingsJobConfig struct {
	JobConfig `mapstructure:",squash"`
	Grace     time.Duration `mapstructure:"grace"`
}

// Bounds of the job settings. The short minimums let a development run see a job act within a minute.
const (
	MinJobInterval    = time.Second
	MaxJobInterval    = 24 * time.Hour
	MinAbandonTimeout = 30 * time.Second
	MaxAbandonTimeout = 24 * time.Hour
	MinRecordingGrace = 30 * time.Second
	MaxRecordingGrace = 24 * time.Hour
	// MinCatchupBatch and MaxCatchupBatch bound the sessions one Roadmap catch-up run examines.
	MinCatchupBatch = 1
	MaxCatchupBatch = 1000
)

// jobEnv lists the job settings and their environment variables.
var jobEnv = map[string]string{
	"jobs.enabled":                      "JOBS_ENABLED",
	"jobs.abandoned_sessions.enabled":   "JOBS_ABANDONED_SESSIONS_ENABLED",
	"jobs.abandoned_sessions.interval":  "JOBS_ABANDONED_SESSIONS_INTERVAL",
	"jobs.abandoned_sessions.timeout":   "JOBS_ABANDONED_SESSIONS_TIMEOUT",
	"jobs.stale_recordings.enabled":     "JOBS_STALE_RECORDINGS_ENABLED",
	"jobs.stale_recordings.interval":    "JOBS_STALE_RECORDINGS_INTERVAL",
	"jobs.stale_recordings.grace":       "JOBS_STALE_RECORDINGS_GRACE",
	"jobs.recording_retention.enabled":  "JOBS_RECORDING_RETENTION_ENABLED",
	"jobs.recording_retention.interval": "JOBS_RECORDING_RETENTION_INTERVAL",
	"jobs.roadmap_catchup.enabled":      "JOBS_ROADMAP_CATCHUP_ENABLED",
	"jobs.roadmap_catchup.interval":     "JOBS_ROADMAP_CATCHUP_INTERVAL",
	"jobs.roadmap_catchup.batch_size":   "JOBS_ROADMAP_CATCHUP_BATCH_SIZE",
}

func setJobDefaults(v *viper.Viper) {
	v.SetDefault("jobs.enabled", true)
	v.SetDefault("jobs.abandoned_sessions.enabled", true)
	v.SetDefault("jobs.abandoned_sessions.interval", "1m")
	v.SetDefault("jobs.abandoned_sessions.timeout", "15m")
	v.SetDefault("jobs.stale_recordings.enabled", true)
	v.SetDefault("jobs.stale_recordings.interval", "1m")
	v.SetDefault("jobs.stale_recordings.grace", "5m")
	v.SetDefault("jobs.recording_retention.enabled", true)
	v.SetDefault("jobs.recording_retention.interval", "1h")
	v.SetDefault("jobs.roadmap_catchup.enabled", true)
	v.SetDefault("jobs.roadmap_catchup.interval", "1m")
	v.SetDefault("jobs.roadmap_catchup.batch_size", 100)
}

func bindJobEnv(v *viper.Viper) error {
	for key, env := range jobEnv {
		if err := v.BindEnv(key, env); err != nil {
			return fmt.Errorf("bind %s: %w", key, err)
		}
	}
	return nil
}

// Validate checks every job's settings, even a disabled job's, so switching it on never meets a bad
// value. With the jobs switched off as a whole nothing is read, so nothing is checked.
func (c JobsConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	for name, interval := range map[string]time.Duration{
		"abandoned_sessions":  c.AbandonedSessions.Interval,
		"stale_recordings":    c.StaleRecordings.Interval,
		"recording_retention": c.RecordingRetention.Interval,
		"roadmap_catchup":     c.RoadmapCatchup.Interval,
	} {
		if interval < MinJobInterval || interval > MaxJobInterval {
			return fmt.Errorf("jobs.%s.interval must be between %s and %s", name, MinJobInterval, MaxJobInterval)
		}
	}
	if t := c.AbandonedSessions.Timeout; t < MinAbandonTimeout || t > MaxAbandonTimeout {
		return fmt.Errorf("jobs.abandoned_sessions.timeout must be between %s and %s", MinAbandonTimeout, MaxAbandonTimeout)
	}
	if b := c.RoadmapCatchup.BatchSize; b < MinCatchupBatch || b > MaxCatchupBatch {
		return fmt.Errorf("jobs.roadmap_catchup.batch_size must be between %d and %d", MinCatchupBatch, MaxCatchupBatch)
	}
	if g := c.StaleRecordings.Grace; g < MinRecordingGrace || g > MaxRecordingGrace {
		return fmt.Errorf("jobs.stale_recordings.grace must be between %s and %s", MinRecordingGrace, MaxRecordingGrace)
	}
	return nil
}
