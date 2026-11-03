package provider

import (
	"context"
	"time"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/features/interview/repository"
	"github.com/cog-forge/rolecue/api/internal/features/interview/service"
	"github.com/cog-forge/rolecue/api/internal/pkg/jobs"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The recording upkeep uses the interview repository; the build fails if a signature drifts.
var _ service.RecordingUpkeepRepository = (*repository.Repository)(nil)

// defaultRecordingGrace is the team decision (2026-11-03) used only when the jobs are off and unvalidated.
const defaultRecordingGrace = 5 * time.Minute

// System job names. They name the advisory lock of each job and appear in the logs.
const (
	jobAbandonedSessions  = "interview.abandoned_sessions"
	jobStaleRecordings    = "interview.stale_recordings"
	jobRecordingRetention = "interview.recording_retention"
)

func ProvideRecordingUpkeep(repo *repository.Repository, store storage.Store, cfg *config.Config) (*service.RecordingUpkeep, error) {
	grace := cfg.Jobs.StaleRecordings.Grace
	if grace <= 0 {
		// Only reachable with the jobs switched off as a whole, when the settings are not validated; the
		// upkeep then never runs, so the 5-minute decision stands in.
		grace = defaultRecordingGrace
	}
	return service.NewRecordingUpkeep(repo, store, grace)
}

// ProvideSystemJobs builds the System Handler's job runner (KAN-174) with every enabled job. main starts
// it with the API and stops it on shutdown. With jobs.enabled off the runner has no job.
func ProvideSystemJobs(cfg *config.Config, pool *pgxpool.Pool, interviews *service.Service, upkeep *service.RecordingUpkeep, log *logger.Logger) (*jobs.Runner, error) {
	locker, err := jobs.NewPostgresLocker(pool)
	if err != nil {
		return nil, err
	}
	settings := cfg.Jobs
	if !settings.Enabled {
		log.Warn("System jobs are switched off (JOBS_ENABLED=false): idle interviews are not ended and recordings are not maintained")
		return jobs.New(locker, nil)
	}
	candidates := []struct {
		enabled bool
		job     jobs.Job
	}{
		{settings.AbandonedSessions.Enabled, jobs.Job{
			Name: jobAbandonedSessions, Interval: settings.AbandonedSessions.Interval,
			Run: countRun(jobAbandonedSessions, "ended", interviews.AbandonIdle),
		}},
		{settings.StaleRecordings.Enabled, jobs.Job{
			Name: jobStaleRecordings, Interval: settings.StaleRecordings.Interval,
			Run: countRun(jobStaleRecordings, "failed", upkeep.FailStale),
		}},
		{settings.RecordingRetention.Enabled, jobs.Job{
			Name: jobRecordingRetention, Interval: settings.RecordingRetention.Interval,
			Run: countRun(jobRecordingRetention, "purged", upkeep.PurgeExpired),
		}},
	}
	enabled := make([]jobs.Job, 0, len(candidates))
	for _, candidate := range candidates {
		if !candidate.enabled {
			log.Info("System job disabled", logger.String("job", candidate.job.Name))
			continue
		}
		enabled = append(enabled, candidate.job)
	}
	return jobs.New(locker, nil, enabled...)
}

// countRun adapts a job that reports how many rows it changed, and logs the count when it is not zero.
func countRun(name, verb string, run func(ctx context.Context, now time.Time) (int, error)) func(ctx context.Context, now time.Time) error {
	return func(ctx context.Context, now time.Time) error {
		count, err := run(ctx, now)
		if count > 0 {
			logger.FromContext(ctx).Info("system job applied", logger.Op("jobs.run"), logger.String("job", name),
				logger.String("action", verb), logger.Int("count", count))
		}
		return err
	}
}
