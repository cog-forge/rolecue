//go:build integration

package jobs

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is not set (load ../.env)")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// Two lockers on separate pools are two API instances: only one holds a job's lock at a time, and the
// lock is free again after unlock.
func TestPostgresLockerExcludesAnotherInstance(t *testing.T) {
	ctx := context.Background()
	first, err := NewPostgresLocker(testPool(t))
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewPostgresLocker(testPool(t))
	if err != nil {
		t.Fatal(err)
	}
	name := "test-" + uuid.NewString()[:8]

	unlock, ok, err := first.TryLock(ctx, name)
	if err != nil || !ok {
		t.Fatalf("first lock ok=%v err=%v", ok, err)
	}
	if _, ok, err := second.TryLock(ctx, name); err != nil || ok {
		t.Fatalf("second instance took a held lock: ok=%v err=%v", ok, err)
	}
	other, ok, err := second.TryLock(ctx, name+"-other")
	if err != nil || !ok {
		t.Fatalf("another job's lock must be independent: ok=%v err=%v", ok, err)
	}
	other()
	unlock()
	again, ok, err := second.TryLock(ctx, name)
	if err != nil || !ok {
		t.Fatalf("lock not released: ok=%v err=%v", ok, err)
	}
	again()
}

// Two runners on separate pools run one job body; while the first holds it the second skips.
func TestTwoRunnersOnPostgresRunAJobOnce(t *testing.T) {
	ctx := context.Background()
	name := "test-" + uuid.NewString()[:8]
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	job := Job{Name: name, Interval: time.Minute, Run: func(context.Context, time.Time) error {
		entered <- struct{}{}
		<-release
		return nil
	}}
	newRunner := func() *Runner {
		locker, err := NewPostgresLocker(testPool(t))
		if err != nil {
			t.Fatal(err)
		}
		runner, err := New(locker, NewFakeClock(time.Now()), job)
		if err != nil {
			t.Fatal(err)
		}
		return runner
	}
	first, second := newRunner(), newRunner()

	done := make(chan bool, 1)
	go func() {
		ran, _ := first.RunOnce(ctx, name)
		done <- ran
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("first runner did not start")
	}
	if ran, err := second.RunOnce(ctx, name); ran || err != nil {
		t.Fatalf("second runner ran=%v err=%v while the first held the lock", ran, err)
	}
	close(release)
	if !<-done {
		t.Fatal("first runner must have run")
	}
}
