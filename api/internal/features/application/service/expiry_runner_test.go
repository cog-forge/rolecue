package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// recordingJob records the clock each pass receives and returns the scripted errors, one per pass, then
// nil.
type recordingJob struct {
	nows   chan time.Time
	errs   []error
	passes atomic.Int32
}

func (j *recordingJob) AutoRejectExpired(_ context.Context, now time.Time) (int, error) {
	index := int(j.passes.Add(1)) - 1
	select {
	case j.nows <- now:
	default:
	}
	if index < len(j.errs) {
		return 0, j.errs[index]
	}
	return 0, nil
}

func newRecordingJob(errs ...error) *recordingJob {
	return &recordingJob{nows: make(chan time.Time, 64), errs: errs}
}

// runAsync runs the runner in a goroutine and reports when Run returns.
func runAsync(ctx context.Context, runner *ExpiryRunner, interval time.Duration, now func() time.Time) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		runner.Run(ctx, interval, now)
		close(done)
	}()
	return done
}

func TestExpiryRunnerRunsOnTheClockAndStopsOnCancel(t *testing.T) {
	base := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	var ticks atomic.Int64
	clock := func() time.Time { return base.Add(time.Duration(ticks.Add(1)) * time.Hour) }
	job := newRecordingJob()
	runner, err := NewExpiryRunner(job)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := runAsync(ctx, runner, time.Millisecond, clock)

	var previous time.Time
	for i := 0; i < 3; i++ {
		select {
		case now := <-job.nows:
			if !now.After(previous) {
				t.Fatalf("pass %d got clock %s after %s, want strictly increasing", i, now, previous)
			}
			previous = now
		case <-time.After(5 * time.Second):
			t.Fatalf("only %d passes ran before the timeout", i)
		}
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after the context was cancelled")
	}
}

func TestExpiryRunnerKeepsRunningAfterAFailedPass(t *testing.T) {
	job := newRecordingJob(errors.New("database unavailable"))
	runner, err := NewExpiryRunner(job)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := runAsync(ctx, runner, time.Millisecond, time.Now)

	deadline := time.After(5 * time.Second)
	for job.passes.Load() < 3 {
		select {
		case <-deadline:
			t.Fatalf("passes after a failure = %d, want the runner to keep ticking", job.passes.Load())
		case <-time.After(time.Millisecond):
		}
	}
	cancel()
	<-done
}

func TestExpiryRunnerDoesNothingWithANonPositiveInterval(t *testing.T) {
	job := newRecordingJob()
	runner, err := NewExpiryRunner(job)
	if err != nil {
		t.Fatal(err)
	}
	done := runAsync(context.Background(), runner, 0, time.Now)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("a zero interval must return at once")
	}
	if job.passes.Load() != 0 {
		t.Fatalf("passes = %d, want none", job.passes.Load())
	}
}

func TestNewExpiryRunnerRequiresAJob(t *testing.T) {
	if _, err := NewExpiryRunner(nil); err == nil {
		t.Fatal("a runner without a job must be refused")
	}
}
