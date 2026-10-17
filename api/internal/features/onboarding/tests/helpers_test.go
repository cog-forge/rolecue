//go:build integration

package tests

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func openTransaction(t *testing.T) (context.Context, pgx.Tx) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is not set; use a disposable migrated test database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	return ctx, tx
}

func seedUser(t *testing.T, ctx context.Context, tx pgx.Tx, role string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := tx.Exec(ctx, `INSERT INTO public.users (id, name, email, role, email_verified)
        VALUES ($1, 'Original', $2, $3, true)`, id, "onboarding-"+id.String()+"@example.test", role)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func wantCode(t *testing.T, err error, code apperror.Code) {
	t.Helper()
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Code != code {
		t.Fatalf("error=%v; want %s", err, code)
	}
}
