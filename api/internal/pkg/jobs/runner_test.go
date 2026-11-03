package jobs

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// memoryLocker is an in-process Locker shared by several runners, standing in for the advisory lock.
type memoryLocker struct {
	mu   sync.Mutex
	held map[string]bool
}

func newMemoryLocker() *memoryLocker { return &memoryLocker{held: map[string]bool{}} }

func (l *memoryLocker) TryLock(_ context.Context, name string) (func(), bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.held[name] {
		return nil, false, nil
	}
	l.held[name] = true
	return func() {
		l.mu.Lock()
		delete(l.held, name)
		l.mu.Unlock()
	}, true, nil
}

func (l *memoryLocker) isHeld(name string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.held[name]
}

type failingLocker struct{}

func (failingLocker) TryLock(context.Context, string) (func(), bool, error) {
	return nil, false, errors.New("database unavailable")
}

var start = time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)

func receive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a job run")
	}
	var zero T
	return zero
}

func expectNone[T any](t *testing.T, ch <-chan T) {
	t.Helper()
	select {
	case value := <-ch:
		t.Fatalf("unexpected run: %v", value)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestNewRejectsInvalidJobs(t *testing.T) {
	run := func(context.Context, time.Time) error { return nil }
	cases := map[string][]Job{
		"bad name":      {{Name: "Bad Name", Interval: time.Minute, Run: run}},
		"duplicate":     {{Name: "a", Interval: time.Minute, Run: run}, {Name: "a", Interval: time.Minute, Run: run}},
		"zero interval": {{Name: "a", Run: run}},
		"no run":        {{Name: "a", Interval: time.Minute}},
	}
	for name, jobs := range cases {
		if _, err := New(newMemoryLocker(), nil, jobs...); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if _, err := New(nil, nil); err == nil {
		t.Error("a nil locker must be refused")
	}
	if _, err := New(newMemoryLocker(), nil); err != nil {
		t.Errorf("a runner without jobs (all disabled) is valid: %v", err)
	}
}

func TestRunnerRunsAtStartAndOnEveryTickWithTheClockTime(t *testing.T) {
	clock := NewFakeClock(start)
	runs := make(chan time.Time, 8)
	runner, err := New(newMemoryLocker(), clock, Job{Name: "tick", Interval: time.Minute, Run: func(_ context.Context, now time.Time) error {
		runs <- now
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	runner.Start(context.Background())
	defer func() { _ = runner.Stop(context.Background()) }()
	clock.WaitTickers(1)

	if got := receive(t, runs); !got.Equal(start) {
		t.Fatalf("first run at %v, want %v", got, start)
	}
	clock.Advance(30 * time.Second)
	expectNone(t, runs)
	clock.Advance(30 * time.Second)
	if got := receive(t, runs); !got.Equal(start.Add(time.Minute)) {
		t.Fatalf("second run at %v, want %v", got, start.Add(time.Minute))
	}
	clock.Advance(time.Minute)
	if got := receive(t, runs); !got.Equal(start.Add(2 * time.Minute)) {
		t.Fatalf("third run at %v", got)
	}
}

func TestEachJobHasItsOwnInterval(t *testing.T) {
	clock := NewFakeClock(start)
	fast, slow := make(chan time.Time, 8), make(chan time.Time, 8)
	runner, err := New(newMemoryLocker(), clock,
		Job{Name: "fast", Interval: time.Minute, Run: func(_ context.Context, now time.Time) error { fast <- now; return nil }},
		Job{Name: "slow", Interval: time.Hour, Run: func(_ context.Context, now time.Time) error { slow <- now; return nil }},
	)
	if err != nil {
		t.Fatal(err)
	}
	runner.Start(context.Background())
	defer func() { _ = runner.Stop(context.Background()) }()
	clock.WaitTickers(2)
	receive(t, fast)
	receive(t, slow)

	clock.Advance(time.Minute)
	receive(t, fast)
	expectNone(t, slow)
}

// One job's failures and panics are isolated: the job runs again on its next tick and the other job
// never misses a run.
func TestFailureAndPanicDoNotStopTheRunner(t *testing.T) {
	clock := NewFakeClock(start)
	var attempts atomic.Int32
	flaky := make(chan int32, 8)
	healthy := make(chan time.Time, 8)
	runner, err := New(newMemoryLocker(), clock,
		Job{Name: "flaky", Interval: time.Minute, Run: func(context.Context, time.Time) error {
			n := attempts.Add(1)
			flaky <- n
			switch n {
			case 1:
				panic("secret answer text must not reach the log")
			case 2:
				return errors.New("storage unavailable")
			}
			return nil
		}},
		Job{Name: "healthy", Interval: time.Minute, Run: func(_ context.Context, now time.Time) error { healthy <- now; return nil }},
	)
	if err != nil {
		t.Fatal(err)
	}
	runner.Start(context.Background())
	defer func() { _ = runner.Stop(context.Background()) }()
	clock.WaitTickers(2)

	for want := int32(1); want <= 3; want++ {
		if got := receive(t, flaky); got != want {
			t.Fatalf("attempt %d, want %d", got, want)
		}
		receive(t, healthy)
		clock.Advance(time.Minute)
	}
}

func TestRunOnceRecoversAPanicAndReleasesTheLock(t *testing.T) {
	locker := newMemoryLocker()
	runner, err := New(locker, NewFakeClock(start), Job{Name: "boom", Interval: time.Minute, Run: func(context.Context, time.Time) error {
		panic(errors.New("boom"))
	}})
	if err != nil {
		t.Fatal(err)
	}
	ran, err := runner.RunOnce(context.Background(), "boom")
	if !ran || err == nil {
		t.Fatalf("ran=%v err=%v, want a recovered panic", ran, err)
	}
	if locker.isHeld("boom") {
		t.Fatal("the lock must be released after a panic")
	}
	if _, err := runner.RunOnce(context.Background(), "missing"); err == nil {
		t.Fatal("an unknown job must be refused")
	}
}

// Two runners (two API instances) share one lock: while one runs the job, the other skips it, and the
// job never runs twice at once.
func TestTwoRunnersNeverOverlapOneJob(t *testing.T) {
	locker := newMemoryLocker()
	var running, maxRunning, total atomic.Int32
	release := make(chan struct{})
	entered := make(chan struct{}, 4)
	job := Job{Name: "exclusive", Interval: time.Minute, Run: func(context.Context, time.Time) error {
		n := running.Add(1)
		defer running.Add(-1)
		for {
			m := maxRunning.Load()
			if n <= m || maxRunning.CompareAndSwap(m, n) {
				break
			}
		}
		total.Add(1)
		entered <- struct{}{}
		<-release
		return nil
	}}
	first, err := New(locker, NewFakeClock(start), job)
	if err != nil {
		t.Fatal(err)
	}
	second, err := New(locker, NewFakeClock(start), job)
	if err != nil {
		t.Fatal(err)
	}

	result := make(chan bool, 1)
	go func() {
		ran, _ := first.RunOnce(context.Background(), "exclusive")
		result <- ran
	}()
	receive(t, entered)
	ran, err := second.RunOnce(context.Background(), "exclusive")
	if ran || err != nil {
		t.Fatalf("second instance ran=%v err=%v while the first held the lock", ran, err)
	}
	close(release)
	if !receive(t, result) {
		t.Fatal("the first instance must have run")
	}
	if ran, _ := second.RunOnce(context.Background(), "exclusive"); !ran {
		t.Fatal("the second instance runs once the lock is free")
	}
	if maxRunning.Load() != 1 || total.Load() != 2 {
		t.Fatalf("max concurrent %d, total %d", maxRunning.Load(), total.Load())
	}
}

func TestALockErrorIsReportedAndTheJobDoesNotRun(t *testing.T) {
	called := false
	runner, err := New(failingLocker{}, NewFakeClock(start), Job{Name: "a", Interval: time.Minute, Run: func(context.Context, time.Time) error {
		called = true
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	if ran, err := runner.RunOnce(context.Background(), "a"); ran || err == nil || called {
		t.Fatalf("ran=%v err=%v called=%v", ran, err, called)
	}
}

// Stop cancels the run in flight and waits for it; after Stop nothing runs again.
func TestStopCancelsAndWaitsForTheRunInFlight(t *testing.T) {
	clock := NewFakeClock(start)
	entered := make(chan struct{}, 4)
	var finished atomic.Bool
	runner, err := New(newMemoryLocker(), clock, Job{Name: "long", Interval: time.Minute, Run: func(ctx context.Context, _ time.Time) error {
		entered <- struct{}{}
		<-ctx.Done()
		time.Sleep(20 * time.Millisecond)
		finished.Store(true)
		return ctx.Err()
	}})
	if err != nil {
		t.Fatal(err)
	}
	runner.Start(context.Background())
	clock.WaitTickers(1)
	receive(t, entered)
	if err := runner.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !finished.Load() {
		t.Fatal("Stop returned before the run finished")
	}
	clock.Advance(time.Hour)
	expectNone(t, entered)
}

func TestStopGivesUpAtItsDeadline(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	entered := make(chan struct{}, 1)
	runner, err := New(newMemoryLocker(), NewFakeClock(start), Job{Name: "stuck", Interval: time.Minute, Run: func(context.Context, time.Time) error {
		entered <- struct{}{}
		<-block
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	runner.Start(context.Background())
	receive(t, entered)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := runner.Stop(ctx); err == nil {
		t.Fatal("Stop must report a run that ignores cancellation")
	}
}

func TestStopBeforeStartAndStartTwice(t *testing.T) {
	runs := make(chan struct{}, 4)
	runner, err := New(newMemoryLocker(), NewFakeClock(start), Job{Name: "once", Interval: time.Minute, Run: func(context.Context, time.Time) error {
		runs <- struct{}{}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	runner.Start(context.Background())
	runner.Start(context.Background())
	receive(t, runs)
	expectNone(t, runs)
	if err := runner.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}
