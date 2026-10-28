//go:build integration

package ledger

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The ledger leaves created_at to the column default; both directions must get a timestamp.
func TestLedgerRowsGetCreatedAtFromTheDefault(t *testing.T) {
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
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	user := uuid.New()
	if _, err := tx.Exec(ctx, `INSERT INTO public.users (id, name, email, role, email_verified)
		VALUES ($1, 'Ledger Clock', $2, 'candidate', true)`, user, user.String()+"@example.com"); err != nil {
		t.Fatal(err)
	}
	var before time.Time
	if err := tx.QueryRow(ctx, `SELECT now()`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if err := Credit(ctx, tx, user, 40, "Ledger clock credit"); err != nil {
		t.Fatal(err)
	}
	if err := Debit(ctx, tx, user, 10, "Ledger clock debit"); err != nil {
		t.Fatal(err)
	}

	rows, err := tx.Query(ctx, `SELECT created_at FROM public.transactions
		WHERE description IN ('Ledger clock credit', 'Ledger clock debit')`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var createdAt time.Time
		if err := rows.Scan(&createdAt); err != nil {
			t.Fatal(err)
		}
		if !createdAt.Equal(before) {
			t.Fatalf("created_at = %v, want the transaction time %v", createdAt, before)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("got %d ledger rows, want 2", count)
	}
}

// A paid PayOS top-up already has its VND order row, so crediting it moves the wallet and writes no row.
func TestCreditForOrderMovesBalanceWithoutARow(t *testing.T) {
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
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	user := uuid.New()
	if _, err := tx.Exec(ctx, `INSERT INTO public.users (id, name, email, role, email_verified)
		VALUES ($1, 'Order Credit', $2, 'candidate', true)`, user, user.String()+"@example.com"); err != nil {
		t.Fatal(err)
	}
	// No wallet yet: the first credit creates it, the second adds to it.
	if err := CreditForOrder(ctx, tx, user, 100); err != nil {
		t.Fatal(err)
	}
	if err := CreditForOrder(ctx, tx, user, 50); err != nil {
		t.Fatal(err)
	}

	var balance int32
	if err := tx.QueryRow(ctx, `SELECT balance FROM public.wallets WHERE user_id = $1`, user).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if balance != 150 {
		t.Fatalf("balance = %d, want 150", balance)
	}
	var rows int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM public.transactions WHERE "to" = $1 OR "from" = $1`,
		user.String()).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 0 {
		t.Fatalf("got %d transactions rows, want none", rows)
	}
}
