package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// ExpiryInterval is how often the expiry pass runs. The pass is one indexed query when nothing is due,
// so a minute keeps an expired Application rejected within a minute of its deadline.
const ExpiryInterval = time.Minute

// ExpiryJob is one expiry pass. The application Service satisfies it.
type ExpiryJob interface {
	AutoRejectExpired(ctx context.Context, now time.Time) (int, error)
}

// ExpiryRunner runs the expiry pass on a ticker in this process. Several instances may run it: the
// pass takes an advisory lock, and an instance that finds it busy does nothing.
type ExpiryRunner struct {
	job ExpiryJob
}

// NewExpiryRunner builds a runner for one expiry job.
func NewExpiryRunner(job ExpiryJob) (*ExpiryRunner, error) {
	if job == nil {
		return nil, fmt.Errorf("application expiry job is required")
	}
	return &ExpiryRunner{job: job}, nil
}

// Run runs the pass once at start, then on every interval tick, until ctx is done. now supplies the
// clock the pass compares deadlines with, so tests can drive it. A failed pass is logged, and the next
// tick tries again.
func (r *ExpiryRunner) Run(ctx context.Context, interval time.Duration, now func() time.Time) {
	if interval <= 0 {
		slog.ErrorContext(ctx, "application expiry interval must be positive", "interval", interval)
		return
	}
	if now == nil {
		now = time.Now
	}
	r.runOnce(ctx, now)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.runOnce(ctx, now)
		}
	}
}

func (r *ExpiryRunner) runOnce(ctx context.Context, now func() time.Time) {
	if ctx.Err() != nil {
		return
	}
	rejected, err := r.job.AutoRejectExpired(ctx, now())
	if err != nil {
		slog.ErrorContext(ctx, "application expiry pass failed", "error", err)
		return
	}
	if rejected > 0 {
		slog.InfoContext(ctx, "expired applications rejected", "count", rejected)
	}
}
