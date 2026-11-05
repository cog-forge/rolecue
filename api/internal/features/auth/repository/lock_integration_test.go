//go:build integration

package repository

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// These tests need a migrated PostgreSQL database. They prove the active-lock predicate in SQL:
// a lock whose lock_expires_at has passed no longer locks the account, a timed lock still does, and
// a lock with no expiry is permanent. Run with:
//
//	cd api && set -a && . ../.env && set +a
//	go test -tags=integration ./internal/features/auth/repository -count=1
func openLockPool(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is not set (load ../.env)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("database not reachable: %v", err)
	}
	return ctx, pool
}

// lockedUser inserts a verified candidate with the given lock state and removes it afterwards.
// expiresIn nil means the lock has no expiry (permanent); otherwise it is an offset from now().
func lockedUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool, isLocked bool, expiresIn *time.Duration) uuid.UUID {
	t.Helper()
	id := uuid.New()
	var expires *time.Time
	if expiresIn != nil {
		at := time.Now().Add(*expiresIn)
		expires = &at
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.users (id, name, email, role, email_verified, is_locked, lock_expires_at)
		VALUES ($1, 'lock test', $2, 'candidate', true, $3, $4)`,
		id, fmt.Sprintf("lock-%s@example.test", id), isLocked, expires); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM public.users WHERE id = $1`, id); err != nil {
			t.Errorf("cleanup user: %v", err)
		}
	})
	return id
}

func TestGetAuthUserLockHonoursExpiry(t *testing.T) {
	ctx, pool := openLockPool(t)
	past, future := -time.Hour, time.Hour
	for _, tc := range []struct {
		name       string
		isLocked   bool
		expiresIn  *time.Duration
		wantLocked bool
	}{
		{"expired lock allows", true, &past, false},
		{"active timed lock denies", true, &future, true},
		{"permanent lock denies", true, nil, true},
		{"unlocked allows", false, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := lockedUser(t, ctx, pool, tc.isLocked, tc.expiresIn)
			got, err := NewRepository(pool).Get(ctx, id)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if got.IsLocked != tc.wantLocked {
				t.Fatalf("IsLocked=%t, want %t", got.IsLocked, tc.wantLocked)
			}
		})
	}
}

func TestSelectRoleLockHonoursExpiry(t *testing.T) {
	ctx, pool := openLockPool(t)
	past, future := -time.Hour, time.Hour

	// An expired lock must not block choosing a role.
	expired := lockedUser(t, ctx, pool, true, &past)
	got, err := NewRepository(pool).SelectRole(ctx, expired, "candidate")
	if err != nil {
		t.Fatalf("SelectRole with expired lock: %v", err)
	}
	if got.IsLocked || got.Role != "candidate" {
		t.Fatalf("user=%+v, want unlocked candidate", got)
	}

	// An active timed lock and a permanent lock both block it.
	for _, tc := range []struct {
		name      string
		expiresIn *time.Duration
	}{
		{"active timed lock", &future},
		{"permanent lock", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := lockedUser(t, ctx, pool, true, tc.expiresIn)
			if _, err := NewRepository(pool).SelectRole(ctx, id, "candidate"); err == nil {
				t.Fatal("SelectRole succeeded for a locked account")
			}
		})
	}
}
