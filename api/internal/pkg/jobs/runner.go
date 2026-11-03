// Package jobs runs the System Handler's background jobs inside the API process (KAN-174). Each job has
// a name, an interval and a run function. The runner starts every job with the API, runs it once at start
// and then on every tick, and stops them gracefully on shutdown. A job takes a cluster-wide lock for each
// run, so several API instances never run the same job at the same time; an instance that finds the lock
// busy skips that run. A failed or panicking run is logged with safe fields and never stops another job
// or the API. There is no queue and no Redis: the jobs read their work from the database each run, so a
// skipped or failed run is simply picked up by the next one.
package jobs

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"runtime/debug"
	"sync"
	"time"

	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
)

// Job is one named System Handler use case. Run must be idempotent: it may run again after a failure,
// after a crash in the middle of a run, or on another instance. now is the runner's clock, so a job never
// reads the wall clock itself and tests can drive it.
type Job struct {
	Name     string
	Interval time.Duration
	Run      func(ctx context.Context, now time.Time) error
}

// Locker takes the cluster-wide lock of one job for one run. ok is false when another holder has it;
// unlock is then nil. Unlock must release the lock even if the run's context is cancelled.
type Locker interface {
	TryLock(ctx context.Context, name string) (unlock func(), ok bool, err error)
}

// jobNamePattern keeps job names short and safe to log and to use in a lock key.
var jobNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,63}$`)

// Runner runs a fixed set of jobs. Start it once; Stop waits for the runs in flight.
type Runner struct {
	jobs   []Job
	locker Locker
	clock  Clock

	mu      sync.Mutex
	started bool
	cancel  context.CancelFunc
	done    sync.WaitGroup
}

// New checks the jobs and builds a runner. Names must be unique and match jobNamePattern; intervals must
// be positive. A nil clock is the system clock. No job at all is allowed (every job disabled).
func New(locker Locker, clock Clock, jobs ...Job) (*Runner, error) {
	if locker == nil {
		return nil, errors.New("jobs: a locker is required")
	}
	if clock == nil {
		clock = SystemClock()
	}
	seen := make(map[string]bool, len(jobs))
	for _, job := range jobs {
		switch {
		case !jobNamePattern.MatchString(job.Name):
			return nil, fmt.Errorf("jobs: invalid job name %q", job.Name)
		case seen[job.Name]:
			return nil, fmt.Errorf("jobs: duplicate job name %q", job.Name)
		case job.Interval <= 0:
			return nil, fmt.Errorf("jobs: job %s needs a positive interval", job.Name)
		case job.Run == nil:
			return nil, fmt.Errorf("jobs: job %s has no run function", job.Name)
		}
		seen[job.Name] = true
	}
	return &Runner{jobs: append([]Job(nil), jobs...), locker: locker, clock: clock}, nil
}

// Start runs every job in its own goroutine until ctx is done or Stop is called. Each job runs once
// right away and then on every interval tick; a run that outlasts its interval drops the ticks it missed
// instead of queueing them, so a job never overlaps itself in one process. A second Start does nothing.
func (r *Runner) Start(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started {
		return
	}
	r.started = true
	ctx, r.cancel = context.WithCancel(ctx)
	log := logger.FromContext(ctx)
	for _, job := range r.jobs {
		log.Info("system job started", logger.Op("jobs.start"), logger.String("job", job.Name),
			logger.Duration("interval", job.Interval))
		r.done.Add(1)
		go r.loop(ctx, job)
	}
}

// Stop cancels every job and waits, until ctx ends, for the runs in flight to return. A run sees its
// context cancelled and should stop at its next database call. Stop before Start returns at once.
func (r *Runner) Stop(ctx context.Context) error {
	r.mu.Lock()
	cancel := r.cancel
	r.mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	done := make(chan struct{})
	go func() {
		r.done.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("jobs did not stop in time: %w", ctx.Err())
	}
}

func (r *Runner) loop(ctx context.Context, job Job) {
	defer r.done.Done()
	ticker := r.clock.NewTicker(job.Interval)
	defer ticker.Stop()
	r.runOnce(ctx, job)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C():
			r.runOnce(ctx, job)
		}
	}
}

// RunOnce runs one job by name now, under its lock, with the same isolation as a scheduled run. It
// reports whether the job ran (false when another holder had the lock) and the run's error. Tests and
// operators use it; the scheduler does not.
func (r *Runner) RunOnce(ctx context.Context, name string) (bool, error) {
	for _, job := range r.jobs {
		if job.Name == name {
			return r.execute(ctx, job)
		}
	}
	return false, fmt.Errorf("jobs: unknown job %q", name)
}

// runOnce is one scheduled run. Its outcome is logged here, so the loop never stops for it.
func (r *Runner) runOnce(ctx context.Context, job Job) {
	if ctx.Err() != nil {
		return
	}
	started := r.clock.Now()
	ran, err := r.execute(ctx, job)
	log := logger.FromContext(ctx)
	switch {
	case err != nil && ctx.Err() != nil:
		// Shutdown cancelled the run; the next start picks the work up again.
		log.Info("system job interrupted by shutdown", logger.Op("jobs.run"), logger.String("job", job.Name))
	case err != nil:
		log.Error("system job failed", logger.Op("jobs.run"), logger.String("job", job.Name),
			logger.Duration("duration", r.clock.Now().Sub(started)), logger.Cause(err))
	case !ran:
		log.Debug("system job skipped: another instance holds its lock", logger.Op("jobs.run"),
			logger.String("job", job.Name))
	}
}

// execute takes the job's lock and runs it. A panic in the run is recovered and returned as an error;
// the panic value is not logged, because a job may hold user content.
func (r *Runner) execute(ctx context.Context, job Job) (ran bool, err error) {
	unlock, ok, err := r.locker.TryLock(ctx, job.Name)
	if err != nil {
		return false, fmt.Errorf("take the lock of job %s: %w", job.Name, err)
	}
	if !ok {
		return false, nil
	}
	defer unlock()
	defer func() {
		if recovered := recover(); recovered != nil {
			logger.FromContext(ctx).Error("system job panicked", logger.Op("jobs.run"), logger.String("job", job.Name),
				logger.String("panic_type", fmt.Sprintf("%T", recovered)), logger.String("stack", string(debug.Stack())))
			ran, err = true, fmt.Errorf("job %s panicked", job.Name)
		}
	}()
	return true, job.Run(ctx, r.clock.Now())
}
