//go:build integration

// Package tests: inventory slot purchase integration tests against a real Postgres. The repository and
// the service run unmodified. Every test seeds its own users and removes exactly their rows afterwards.
package tests

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/features/inventory/domain"
	"github.com/cog-forge/rolecue/api/internal/features/inventory/repository"
	"github.com/cog-forge/rolecue/api/internal/features/inventory/service"
	"github.com/cog-forge/rolecue/api/internal/pkg/ledger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// openPool connects to the disposable database named by DATABASE_URL. The test is skipped when the
// variable is unset or the database is unreachable.
func openPool(t *testing.T) (context.Context, *pgxpool.Pool) {
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

// seed tracks the users a test creates; the cleanup removes their rows children first.
type seed struct {
	t     *testing.T
	ctx   context.Context
	pool  *pgxpool.Pool
	users []string
}

func newSeed(t *testing.T, ctx context.Context, pool *pgxpool.Pool) *seed {
	t.Helper()
	s := &seed{t: t, ctx: ctx, pool: pool}
	t.Cleanup(s.cleanup)
	return s
}

func (s *seed) cleanup() {
	for _, step := range []struct{ name, sql string }{
		{"transactions", `DELETE FROM public.transactions WHERE "from" = ANY($1::text[]) OR "to" = ANY($1::text[])`},
		{"inventories", `DELETE FROM public.inventories WHERE user_id = ANY($1::text[]::uuid[])`},
		{"wallets", `DELETE FROM public.wallets WHERE user_id = ANY($1::text[]::uuid[])`},
		{"users", `DELETE FROM public.users WHERE id = ANY($1::text[]::uuid[])`},
	} {
		if _, err := s.pool.Exec(context.Background(), step.sql, s.users); err != nil {
			s.t.Errorf("cleanup %s: %v", step.name, err)
		}
	}
}

func (s *seed) exec(sql string, args ...any) {
	s.t.Helper()
	if _, err := s.pool.Exec(s.ctx, sql, args...); err != nil {
		s.t.Fatalf("seed %q: %v", sql, err)
	}
}

// candidate adds a verified Candidate, optionally with a wallet of the balance (nil means no wallet).
func (s *seed) candidate(balance *int32) authdomain.User {
	s.t.Helper()
	id := uuid.New()
	s.exec(`INSERT INTO public.users (id, name, email, role, email_verified) VALUES ($1, 'inventory candidate', $2, 'candidate', true)`,
		id, fmt.Sprintf("inventory-%s@example.test", id))
	s.users = append(s.users, id.String())
	if balance != nil {
		s.exec(`INSERT INTO public.wallets (user_id, balance) VALUES ($1, $2)`, id, *balance)
	}
	return authdomain.User{ID: id, Role: "candidate", EmailVerified: true}
}

func coins(n int32) *int32 { return &n }

// wallet reads the balance and whether the user has a wallet at all.
func (s *seed) wallet(userID uuid.UUID) (int32, bool) {
	s.t.Helper()
	var balance int32
	err := s.pool.QueryRow(s.ctx, `SELECT balance FROM public.wallets WHERE user_id = $1`, userID).Scan(&balance)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false
	}
	if err != nil {
		s.t.Fatal(err)
	}
	return balance, true
}

// slots reads the inventory capacity, or -1 when the user has no inventory row.
func (s *seed) slots(userID uuid.UUID) int32 {
	s.t.Helper()
	var slot int32
	err := s.pool.QueryRow(s.ctx, `SELECT slot FROM public.inventories WHERE user_id = $1`, userID).Scan(&slot)
	if errors.Is(err, pgx.ErrNoRows) {
		return -1
	}
	if err != nil {
		s.t.Fatal(err)
	}
	return slot
}

// ledgerRow is one transactions row the user takes part in.
type ledgerRow struct {
	from, to, currency, status, description string
	amount                                  int32
}

func (s *seed) ledgerRows(userID uuid.UUID) []ledgerRow {
	s.t.Helper()
	rows, err := s.pool.Query(s.ctx, `SELECT "from", "to", currency, status::text, COALESCE(description, ''), amount
		FROM public.transactions WHERE "from" = $1 OR "to" = $1 ORDER BY id`, userID.String())
	if err != nil {
		s.t.Fatal(err)
	}
	defer rows.Close()
	var out []ledgerRow
	for rows.Next() {
		var r ledgerRow
		if err := rows.Scan(&r.from, &r.to, &r.currency, &r.status, &r.description, &r.amount); err != nil {
			s.t.Fatal(err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		s.t.Fatal(err)
	}
	return out
}

func newService(pool *pgxpool.Pool) *service.Service {
	return service.NewService(repository.NewRepository(pool), nil)
}

// TestPurchaseDebitsTheLedgerAndReturnsTheSummary buys two slots for a Candidate with five coins and no
// inventory yet. It proves the inventory starts at the initial capacity and grows by two, the wallet
// loses two coins, one ledger row is written like every other coin debit, and the result is the GET
// /inventory summary plus the balance after the debit.
func TestPurchaseDebitsTheLedgerAndReturnsTheSummary(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(pool)
	user := s.candidate(coins(5))

	got, err := svc.PurchaseSlot(ctx, user, 2)
	if err != nil {
		t.Fatalf("PurchaseSlot() error = %v", err)
	}
	want := domain.SlotPurchase{
		Capacity: domain.Capacity{Capacity: domain.InitialCapacity + 2, Used: 0, Available: int64(domain.InitialCapacity + 2), IsFull: false},
		Balance:  3,
	}
	if got != want {
		t.Fatalf("PurchaseSlot() = %+v, want %+v", got, want)
	}
	summary, err := svc.Summary(ctx, user)
	if err != nil || summary != got.Capacity {
		t.Fatalf("Summary() = %+v, %v, want the purchase summary %+v", summary, err, got.Capacity)
	}
	if balance, _ := s.wallet(user.ID); balance != 3 {
		t.Fatalf("balance = %d, want 3", balance)
	}
	rows := s.ledgerRows(user.ID)
	if len(rows) != 1 {
		t.Fatalf("ledger rows = %+v, want one", rows)
	}
	r := rows[0]
	if r.from != user.ID.String() || r.to != ledger.Counterparty || r.currency != ledger.Currency || r.status != "success" ||
		r.amount != 2*domain.PricePerSlot || r.description != "Avatar inventory, 2 slots" {
		t.Fatalf("ledger row = %+v, want a successful COIN debit of 2 to system", r)
	}
}

// TestConcurrentPurchasesNeverLoseAnUpdate runs two purchases of three slots at once on a wallet of ten
// coins. Both must land: the wallet ends at four, the capacity grows by six and there are exactly two
// ledger rows.
func TestConcurrentPurchasesNeverLoseAnUpdate(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(pool)
	user := s.candidate(coins(10))

	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, errs[i] = svc.PurchaseSlot(ctx, user, 3)
		}()
	}
	close(start)
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("purchase #%d error = %v", i+1, err)
		}
	}
	if balance, _ := s.wallet(user.ID); balance != 4 {
		t.Fatalf("balance = %d, want 4 after two purchases of 3", balance)
	}
	if slot := s.slots(user.ID); slot != domain.InitialCapacity+6 {
		t.Fatalf("capacity = %d, want %d", slot, domain.InitialCapacity+6)
	}
	if rows := s.ledgerRows(user.ID); len(rows) != 2 || rows[0].amount != 3 || rows[1].amount != 3 {
		t.Fatalf("ledger rows = %+v, want exactly two debits of 3", rows)
	}
}

// TestPurchaseKeepsAConcurrentTopUp runs a purchase and a ledger credit at once. The purchase never
// writes an absolute balance it read earlier, so the credit is kept.
func TestPurchaseKeepsAConcurrentTopUp(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(pool)
	user := s.candidate(coins(10))

	var wg sync.WaitGroup
	start := make(chan struct{})
	var purchaseErr, creditErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, purchaseErr = svc.PurchaseSlot(ctx, user, 4)
	}()
	go func() {
		defer wg.Done()
		<-start
		creditErr = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
			return ledger.Credit(ctx, tx, user.ID, 5, "test top-up")
		})
	}()
	close(start)
	wg.Wait()
	if purchaseErr != nil || creditErr != nil {
		t.Fatalf("purchase error = %v, credit error = %v", purchaseErr, creditErr)
	}
	if balance, _ := s.wallet(user.ID); balance != 11 {
		t.Fatalf("balance = %d, want 11 (10 - 4 + 5)", balance)
	}
}

// TestPurchaseRefusalsWriteNothing proves a balance too low, a missing wallet and an amount over the
// per-request bound are refused with stable codes (never INTERNAL_ERROR) and leave no inventory, wallet
// or ledger row behind.
func TestPurchaseRefusalsWriteNothing(t *testing.T) {
	ctx, pool := openPool(t)
	svc := newService(pool)

	cases := []struct {
		name    string
		balance *int32
		amount  int32
		code    apperror.Code
	}{
		{"insufficient balance", coins(1), 2, apperror.CodeInsufficientBalance},
		{"missing wallet", nil, 1, apperror.CodeInsufficientBalance},
		{"amount over the bound", coins(100), domain.MaxSlotsPerPurchase + 1, apperror.CodeValidation},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newSeed(t, ctx, pool)
			user := s.candidate(tc.balance)
			got, err := svc.PurchaseSlot(ctx, user, tc.amount)
			if !apperror.IsCode(err, tc.code) {
				t.Fatalf("PurchaseSlot() error = %v, want %s", err, tc.code)
			}
			if got != (domain.SlotPurchase{}) {
				t.Fatalf("refused purchase returned %+v", got)
			}
			balance, found := s.wallet(user.ID)
			if found != (tc.balance != nil) || (found && balance != *tc.balance) {
				t.Fatalf("wallet = %d (found %v), want it unchanged", balance, found)
			}
			if slot := s.slots(user.ID); slot != -1 {
				t.Fatalf("inventory capacity = %d, want no inventory row written", slot)
			}
			if rows := s.ledgerRows(user.ID); len(rows) != 0 {
				t.Fatalf("ledger rows = %+v, want none", rows)
			}
		})
	}
}
