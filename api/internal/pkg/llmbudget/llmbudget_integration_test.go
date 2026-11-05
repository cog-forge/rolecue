//go:build integration

package llmbudget

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The upsert must count atomically, stop at the limit under concurrency, and keep days apart.
func TestPostgresStoreCapsAtomicallyAndResetsPerDay(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	user := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO public.users (id, name, email, role, email_verified)
		VALUES ($1, 'Budget User', $2, 'candidate', true)`, user, user.String()+"@example.com"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM public.users WHERE id = $1`, user) })

	store := NewPostgresStore(pool)
	day := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)

	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, ok, err := store.Consume(ctx, user, day, 5)
			if err != nil {
				t.Error(err)
				return
			}
			if ok {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if allowed != 5 {
		t.Fatalf("allowed = %d, want exactly 5", allowed)
	}
	if _, ok, err := store.Consume(ctx, user, day.AddDate(0, 0, 1), 5); err != nil || !ok {
		t.Fatalf("next day: ok=%v err=%v, want a fresh budget", ok, err)
	}
	// Limit 0 is unlimited and still counts.
	for i := 0; i < 10; i++ {
		if _, ok, err := store.Consume(ctx, user, day, 0); err != nil || !ok {
			t.Fatalf("unlimited call %d: ok=%v err=%v", i+1, ok, err)
		}
	}
}
