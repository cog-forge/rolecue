package config

import (
	"os"
	"testing"
	"time"
)

func TestJobsConfigDefaultsAndEnvironment(t *testing.T) {
	t.Setenv("PAYOS_CLIENT_ID", "test-client-id")
	t.Setenv("PAYOS_API_KEY", "test-api-key")
	t.Setenv("PAYOS_CHECKSUM_KEY", "test-checksum-key")
	for _, env := range jobEnv {
		t.Setenv(env, "")
		if err := os.Unsetenv(env); err != nil {
			t.Fatal(err)
		}
	}

	cfg, err := Load(writeConfig(t, minimalYAML))
	if err != nil {
		t.Fatal(err)
	}
	jobs := cfg.Jobs
	if !jobs.Enabled || !jobs.AbandonedSessions.Enabled || !jobs.StaleRecordings.Enabled || !jobs.RecordingRetention.Enabled {
		t.Fatalf("every job is on by default: %+v", jobs)
	}
	if jobs.AbandonedSessions.Timeout != 15*time.Minute || jobs.StaleRecordings.Grace != 5*time.Minute {
		t.Fatalf("defaults timeout=%s grace=%s; want 15m and 5m", jobs.AbandonedSessions.Timeout, jobs.StaleRecordings.Grace)
	}
	if jobs.AbandonedSessions.Interval != time.Minute || jobs.RecordingRetention.Interval != time.Hour {
		t.Fatalf("default intervals %s, %s", jobs.AbandonedSessions.Interval, jobs.RecordingRetention.Interval)
	}
	if !jobs.RoadmapCatchup.Enabled || jobs.RoadmapCatchup.Interval != time.Minute || jobs.RoadmapCatchup.BatchSize != 100 {
		t.Fatalf("roadmap catch-up defaults: %+v", jobs.RoadmapCatchup)
	}

	t.Setenv("JOBS_ABANDONED_SESSIONS_TIMEOUT", "1m")
	t.Setenv("JOBS_ABANDONED_SESSIONS_INTERVAL", "10s")
	t.Setenv("JOBS_RECORDING_RETENTION_ENABLED", "false")
	t.Setenv("JOBS_ROADMAP_CATCHUP_BATCH_SIZE", "25")
	t.Setenv("JOBS_ROADMAP_CATCHUP_ENABLED", "false")
	t.Setenv("JOBS_ENABLED", "false")
	cfg, err = Load(writeConfig(t, minimalYAML))
	if err != nil {
		t.Fatal(err)
	}
	jobs = cfg.Jobs
	if jobs.Enabled || jobs.RecordingRetention.Enabled || jobs.AbandonedSessions.Timeout != time.Minute ||
		jobs.AbandonedSessions.Interval != 10*time.Second || jobs.RoadmapCatchup.Enabled || jobs.RoadmapCatchup.BatchSize != 25 {
		t.Fatalf("from the environment: %+v", jobs)
	}
}

func TestJobsConfigValidate(t *testing.T) {
	valid := JobsConfig{
		Enabled:            true,
		AbandonedSessions:  AbandonedSessionsJobConfig{JobConfig: JobConfig{Interval: time.Minute}, Timeout: 15 * time.Minute},
		StaleRecordings:    StaleRecordingsJobConfig{JobConfig: JobConfig{Interval: time.Minute}, Grace: 5 * time.Minute},
		RecordingRetention: JobConfig{Interval: time.Hour},
		RoadmapCatchup:     RoadmapCatchupJobConfig{JobConfig: JobConfig{Interval: time.Minute}, BatchSize: 100},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid config refused: %v", err)
	}
	if err := (JobsConfig{}).Validate(); err != nil {
		t.Fatalf("switched-off jobs are not checked: %v", err)
	}
	for name, mutate := range map[string]func(*JobsConfig){
		"zero interval":       func(c *JobsConfig) { c.RecordingRetention.Interval = 0 },
		"interval too long":   func(c *JobsConfig) { c.StaleRecordings.Interval = 25 * time.Hour },
		"timeout too short":   func(c *JobsConfig) { c.AbandonedSessions.Timeout = time.Second },
		"timeout missing":     func(c *JobsConfig) { c.AbandonedSessions.Timeout = 0 },
		"grace missing":       func(c *JobsConfig) { c.StaleRecordings.Grace = 0 },
		"catch-up interval":   func(c *JobsConfig) { c.RoadmapCatchup.Interval = 0 },
		"catch-up batch zero": func(c *JobsConfig) { c.RoadmapCatchup.BatchSize = 0 },
		"catch-up batch huge": func(c *JobsConfig) { c.RoadmapCatchup.BatchSize = MaxCatchupBatch + 1 },
		"disabled bad values": func(c *JobsConfig) { c.AbandonedSessions.Enabled, c.AbandonedSessions.Interval = false, 0 },
	} {
		c := valid
		mutate(&c)
		if err := c.Validate(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
