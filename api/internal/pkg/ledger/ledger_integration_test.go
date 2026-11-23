//go:build integration

package ledger

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

// Credit, Debit and DebitKeyed against the real schema: balances, the COIN rows, the idempotency key on a
// keyed debit, the unique index behind it, and the refusals that change nothing.
func TestSingleWalletOperationsOnTheRealSchema(t *testing.T) {
	e := newItEnv(t)
	user := e.user(coinsOf(50))
	missing := e.user(nil)

	if err := e.run(func(tx pgx.Tx) error { return Credit(e.ctx, tx, user, 25, "Ledger credit") }); err != nil {
		t.Fatal(err)
	}
	if err := e.run(func(tx pgx.Tx) error { return Debit(e.ctx, tx, user, 10, "Ledger debit") }); err != nil {
		t.Fatal(err)
	}
	key := "slot-" + uuid.NewString()
	if err := e.run(func(tx pgx.Tx) error { return DebitKeyed(e.ctx, tx, user, 5, "Ledger keyed debit", key) }); err != nil {
		t.Fatal(err)
	}
	if got := e.balance(user); got != 60 {
		t.Fatalf("balance = %d, want 60", got)
	}
	var keyed, plain int
	if err := e.pool.QueryRow(e.ctx, `SELECT count(*) FILTER (WHERE idempotency_key = $2), count(*) FILTER (WHERE idempotency_key IS NULL)
		FROM public.transactions WHERE "from" = $1 OR "to" = $1`, user.String(), key).Scan(&keyed, &plain); err != nil {
		t.Fatal(err)
	}
	if keyed != 1 || plain != 2 {
		t.Fatalf("rows with the key = %d, without = %d, want 1 and 2", keyed, plain)
	}

	// The unique index is the backstop for a second row under one key: it fails and the balance is untouched.
	err := e.run(func(tx pgx.Tx) error { return DebitKeyed(e.ctx, tx, user, 5, "Ledger keyed debit again", key) })
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperror.CodeInternal {
		t.Fatalf("duplicate key error = %v, want an internal persistence error", err)
	}
	if got := e.balance(user); got != 60 {
		t.Fatalf("balance after the refused duplicate = %d, want 60", got)
	}

	for name, fn := range map[string]func(tx pgx.Tx) error{
		"over balance":   func(tx pgx.Tx) error { return Debit(e.ctx, tx, user, 61, "too much") },
		"missing wallet": func(tx pgx.Tx) error { return Debit(e.ctx, tx, missing, 1, "no wallet") },
	} {
		err := e.run(fn)
		if !errors.As(err, &appErr) || appErr.Code != apperror.CodeInsufficientBalance {
			t.Fatalf("%s error = %v, want INSUFFICIENT_BALANCE", name, err)
		}
	}
	if e.balance(user) != 60 || e.balance(missing) != -1 || e.rowCount(missing) != 0 {
		t.Fatalf("a refused debit changed state: user=%d missing=%d rows=%d", e.balance(user), e.balance(missing), e.rowCount(missing))
	}

	// A credit creates the missing wallet.
	if err := e.run(func(tx pgx.Tx) error { return Credit(e.ctx, tx, missing, 7, "First credit") }); err != nil {
		t.Fatal(err)
	}
	if e.balance(missing) != 7 {
		t.Fatalf("created wallet balance = %d, want 7", e.balance(missing))
	}
}
