//go:build integration

package ledger

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// itEnv is a pool on the disposable database plus the users a test created, removed afterwards children
// first. The concurrency tests need committed data on several connections, so nothing here is a
// rollback-only transaction.
type itEnv struct {
	t     *testing.T
	ctx   context.Context
	pool  *pgxpool.Pool
	users []string
}

func newItEnv(t *testing.T) *itEnv {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is not set (load ../.env)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("database not reachable: %v", err)
	}
	e := &itEnv{t: t, ctx: ctx, pool: pool}
	t.Cleanup(e.cleanup)
	return e
}

func (e *itEnv) cleanup() {
	for _, sql := range []string{
		`DELETE FROM public.transactions WHERE "from" = ANY($1::text[]) OR "to" = ANY($1::text[])`,
		`DELETE FROM public.wallets WHERE user_id = ANY($1::text[]::uuid[])`,
		`DELETE FROM public.users WHERE id = ANY($1::text[]::uuid[])`,
	} {
		if _, err := e.pool.Exec(context.Background(), sql, e.users); err != nil {
			e.t.Errorf("cleanup: %v", err)
		}
	}
}

// user adds a Candidate with a wallet of balance; a nil balance means no wallet row.
func (e *itEnv) user(balance *int32) uuid.UUID {
	e.t.Helper()
	id := uuid.New()
	if _, err := e.pool.Exec(e.ctx, `INSERT INTO public.users (id, name, email, role, email_verified)
		VALUES ($1, 'ledger transfer', $2, 'candidate', true)`, id, fmt.Sprintf("transfer-%s@example.test", id)); err != nil {
		e.t.Fatal(err)
	}
	e.users = append(e.users, id.String())
	if balance != nil {
		if _, err := e.pool.Exec(e.ctx, `INSERT INTO public.wallets (user_id, balance) VALUES ($1, $2)`, id, *balance); err != nil {
			e.t.Fatal(err)
		}
	}
	return id
}

func coinsOf(n int32) *int32 { return &n }

// balance is -1 when the user has no wallet row.
func (e *itEnv) balance(id uuid.UUID) int32 {
	e.t.Helper()
	var b int32
	err := e.pool.QueryRow(e.ctx, `SELECT balance FROM public.wallets WHERE user_id = $1`, id).Scan(&b)
	if errors.Is(err, pgx.ErrNoRows) {
		return -1
	}
	if err != nil {
		e.t.Fatal(err)
	}
	return b
}

func (e *itEnv) rowCount(id uuid.UUID) int {
	e.t.Helper()
	var n int
	if err := e.pool.QueryRow(e.ctx, `SELECT count(*) FROM public.transactions WHERE "from" = $1 OR "to" = $1`, id.String()).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

// run executes fn in its own transaction and commits when fn returns nil.
func (e *itEnv) run(fn func(tx pgx.Tx) error) error {
	return pgx.BeginFunc(e.ctx, e.pool, fn)
}

func (e *itEnv) transfer(buyer, seller uuid.UUID, amount int32, nonce string) (MarketplaceReceipt, error) {
	e.t.Helper()
	key, err := NewMarketplaceKey(listing, amount, nonce)
	if err != nil {
		e.t.Fatal(err)
	}
	return e.transferKeyed(key, listing, buyer, seller, amount)
}

func (e *itEnv) transferKeyed(key string, listingID, buyer, seller uuid.UUID, amount int32) (MarketplaceReceipt, error) {
	var receipt MarketplaceReceipt
	err := e.run(func(tx pgx.Tx) error {
		var err error
		receipt, err = MarketplaceTransfer(e.ctx, tx, MarketplaceTransferInput{
			Key: key, ListingID: listingID, Buyer: buyer, Seller: seller, Amount: amount, Description: "Nam bought avatar Alex from Bao",
		})
		return err
	})
	return receipt, err
}

func nonceN(prefix string, n int) string { return fmt.Sprintf("%s%016d", prefix, n) }

func TestTransferBuyerFiftySellerTenBuysTwenty(t *testing.T) {
	e := newItEnv(t)
	buyer, seller := e.user(coinsOf(50)), e.user(coinsOf(10))
	got, err := e.transfer(buyer, seller, 20, nonce16)
	if err != nil {
		t.Fatal(err)
	}
	if got.Replayed || got.Amount != 20 {
		t.Fatalf("receipt = %+v", got)
	}
	if b, s := e.balance(buyer), e.balance(seller); b != 30 || s != 30 {
		t.Fatalf("balances = %d/%d, want 30/30", b, s)
	}
	var from, to, currency, status, key string
	var amount int32
	var description *string
	if err := e.pool.QueryRow(e.ctx, `SELECT "from", "to", currency, status::text, amount, description, idempotency_key
		FROM public.transactions WHERE id = $1`, got.TransactionID).Scan(&from, &to, &currency, &status, &amount, &description, &key); err != nil {
		t.Fatal(err)
	}
	wantKey, _ := NewMarketplaceKey(listing, 20, nonce16)
	if from != buyer.String() || to != seller.String() || currency != Currency || status != "success" || amount != 20 ||
		description == nil || *description != "Nam bought avatar Alex from Bao" || key != wantKey {
		t.Fatalf("row = %s -> %s %d %s %s %v %s", from, to, amount, currency, status, description, key)
	}
	if e.rowCount(buyer) != 1 || e.rowCount(seller) != 1 {
		t.Fatal("want exactly one transactions row for the pair")
	}
}

func TestTransferRefusalsChangeNothing(t *testing.T) {
	e := newItEnv(t)
	for _, tc := range []struct {
		name          string
		buyer, seller *int32
		amount        int32
		code          apperror.Code
	}{
		{"insufficient", coinsOf(19), coinsOf(10), 20, apperror.CodeInsufficientBalance},
		{"buyer has no wallet", nil, coinsOf(10), 20, apperror.CodeInsufficientBalance},
		{"seller would pass max int32", coinsOf(50), coinsOf(math.MaxInt32 - 19), 20, apperror.CodeCoinRecipientBalanceLimit},
		{"zero", coinsOf(50), coinsOf(10), 0, apperror.CodeValidation},
		{"negative", coinsOf(50), coinsOf(10), -5, apperror.CodeValidation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			buyer, seller := e.user(tc.buyer), e.user(tc.seller)
			beforeB, beforeS := e.balance(buyer), e.balance(seller)
			var err error
			if tc.amount <= 0 {
				_, err = e.transferKeyed(fmt.Sprintf("mkt1:%s:20:%s", listing, nonce16), listing, buyer, seller, tc.amount)
			} else {
				_, err = e.transfer(buyer, seller, tc.amount, nonce16)
			}
			if !apperror.IsCode(err, tc.code) {
				t.Fatalf("error = %v, want %s", err, tc.code)
			}
			if e.balance(buyer) != beforeB || e.balance(seller) != beforeS || e.rowCount(buyer) != 0 || e.rowCount(seller) != 0 {
				t.Fatalf("a refused transfer changed state: %d/%d rows %d/%d", e.balance(buyer), e.balance(seller), e.rowCount(buyer), e.rowCount(seller))
			}
		})
	}
}

func TestTransferToSelfIsRefused(t *testing.T) {
	e := newItEnv(t)
	u := e.user(coinsOf(50))
	if _, err := e.transfer(u, u, 20, nonce16); !apperror.IsCode(err, apperror.CodeValidation) {
		t.Fatalf("error = %v, want VALIDATION_ERROR", err)
	}
	if e.balance(u) != 50 || e.rowCount(u) != 0 {
		t.Fatal("a self transfer changed state")
	}
}

func TestTransferCreatesAMissingSellerWallet(t *testing.T) {
	e := newItEnv(t)
	buyer, seller := e.user(coinsOf(50)), e.user(nil)
	if _, err := e.transfer(buyer, seller, 20, nonce16); err != nil {
		t.Fatal(err)
	}
	if e.balance(buyer) != 30 || e.balance(seller) != 20 {
		t.Fatalf("balances = %d/%d, want 30/20", e.balance(buyer), e.balance(seller))
	}
}

func TestTransferRollsBackWithTheCallersTransaction(t *testing.T) {
	e := newItEnv(t)
	buyer, seller := e.user(coinsOf(50)), e.user(nil)
	boom := errors.New("avatar grant failed")
	err := e.run(func(tx pgx.Tx) error {
		key, _ := NewMarketplaceKey(listing, 20, nonce16)
		if _, err := MarketplaceTransfer(e.ctx, tx, MarketplaceTransferInput{
			Key: key, ListingID: listing, Buyer: buyer, Seller: seller, Amount: 20, Description: "rolled back",
		}); err != nil {
			t.Fatal(err)
		}
		return boom // the caller's later step fails
	})
	if !errors.Is(err, boom) {
		t.Fatalf("error = %v", err)
	}
	// Both balances, the created seller wallet and the ledger are exactly as before.
	if e.balance(buyer) != 50 || e.balance(seller) != -1 || e.rowCount(buyer) != 0 || e.rowCount(seller) != 0 {
		t.Fatalf("after rollback: buyer %d seller %d rows %d/%d", e.balance(buyer), e.balance(seller), e.rowCount(buyer), e.rowCount(seller))
	}
	// The same key can then be used for the retried purchase.
	got, err := e.transfer(buyer, seller, 20, nonce16)
	if err != nil || got.Replayed {
		t.Fatalf("retry after rollback = %+v, %v; want a first success", got, err)
	}
}

func TestTransferReplayChangesNothingAndReturnsTheOriginalRow(t *testing.T) {
	e := newItEnv(t)
	buyer, seller := e.user(coinsOf(50)), e.user(coinsOf(10))
	first, err := e.transfer(buyer, seller, 20, nonce16)
	if err != nil {
		t.Fatal(err)
	}
	// The buyer spends down to 5 coins before the lost-response retry arrives.
	if _, err := e.pool.Exec(e.ctx, `UPDATE public.wallets SET balance = 5 WHERE user_id = $1`, buyer); err != nil {
		t.Fatal(err)
	}
	again, err := e.transfer(buyer, seller, 20, nonce16)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if !again.Replayed || again.TransactionID != first.TransactionID || !again.CreatedAt.Equal(first.CreatedAt) || again.Description != first.Description {
		t.Fatalf("replay = %+v, first = %+v", again, first)
	}
	if e.balance(buyer) != 5 || e.balance(seller) != 30 || e.rowCount(buyer) != 1 {
		t.Fatalf("a replay changed state: %d/%d rows %d", e.balance(buyer), e.balance(seller), e.rowCount(buyer))
	}
	// A new purchase intent is a new nonce, so a new charge.
	if _, err := e.transfer(buyer, seller, 20, nonce16+"b"); !apperror.IsCode(err, apperror.CodeInsufficientBalance) {
		t.Fatalf("new nonce with 5 coins: error = %v, want INSUFFICIENT_BALANCE", err)
	}
}

func TestTransferSameKeyConflicts(t *testing.T) {
	e := newItEnv(t)
	buyer, seller, other := e.user(coinsOf(100)), e.user(coinsOf(0)), e.user(coinsOf(0))
	key, _ := NewMarketplaceKey(listing, 20, nonce16)
	if _, err := e.transferKeyed(key, listing, buyer, seller, 20); err != nil {
		t.Fatal(err)
	}
	otherListing := uuid.New()
	for _, tc := range []struct {
		name      string
		listingID uuid.UUID
		seller    uuid.UUID
		amount    int32
	}{
		{"other listing", otherListing, seller, 20},
		{"other price", listing, seller, 25},
		{"other seller", listing, other, 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := e.transferKeyed(key, tc.listingID, buyer, tc.seller, tc.amount); !apperror.IsCode(err, apperror.CodeCoinTransferKeyConflict) {
				t.Fatalf("error = %v, want %s", err, apperror.CodeCoinTransferKeyConflict)
			}
		})
	}
	if e.balance(buyer) != 80 || e.balance(seller) != 20 || e.balance(other) != 0 || e.rowCount(buyer) != 1 {
		t.Fatalf("a conflict changed state: %d/%d/%d rows %d", e.balance(buyer), e.balance(seller), e.balance(other), e.rowCount(buyer))
	}
}

// A slot purchase takes any client string as its key. If it ever stored a Marketplace-shaped key, the
// Marketplace must not take that row for its own.
func TestTransferKeyFromAnotherOperationIsNeverAReplay(t *testing.T) {
	e := newItEnv(t)
	buyer, seller := e.user(coinsOf(100)), e.user(coinsOf(0))
	key, _ := NewMarketplaceKey(listing, 20, nonce16)
	if _, err := e.pool.Exec(e.ctx, `INSERT INTO public.transactions ("from", "to", amount, currency, description, status, idempotency_key)
		VALUES ($1, 'system', 20, 'COIN', 'Avatar inventory, 20 slots', 'success', $2)`, buyer.String(), key); err != nil {
		t.Fatal(err)
	}
	if _, err := e.transferKeyed(key, listing, buyer, seller, 20); !apperror.IsCode(err, apperror.CodeCoinTransferKeyConflict) {
		t.Fatalf("error = %v, want %s", err, apperror.CodeCoinTransferKeyConflict)
	}
	if e.balance(buyer) != 100 || e.balance(seller) != 0 {
		t.Fatal("a cross-purpose key moved coins")
	}
	for _, foreign := range []string{"my-slot-key-0001", "a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d", "slot:" + key} {
		if _, err := e.transferKeyed(foreign, listing, buyer, seller, 20); !apperror.IsCode(err, apperror.CodeCoinTransferKeyInvalid) {
			t.Fatalf("foreign key %q: error = %v, want %s", foreign, err, apperror.CodeCoinTransferKeyInvalid)
		}
	}
}

func TestTransferKeysAreScopedToTheBuyer(t *testing.T) {
	e := newItEnv(t)
	a, b, seller := e.user(coinsOf(50)), e.user(coinsOf(50)), e.user(coinsOf(0))
	if _, err := e.transfer(a, seller, 20, nonce16); err != nil {
		t.Fatal(err)
	}
	got, err := e.transfer(b, seller, 20, nonce16)
	if err != nil || got.Replayed {
		t.Fatalf("second buyer, same key text: %+v, %v; want a first success of its own", got, err)
	}
	if e.balance(seller) != 40 {
		t.Fatalf("seller = %d, want 40", e.balance(seller))
	}
}

func TestTransferSameKeyInParallelChargesOnce(t *testing.T) {
	e := newItEnv(t)
	buyer, seller := e.user(coinsOf(50)), e.user(coinsOf(10))
	const workers = 8
	results := make([]MarketplaceReceipt, workers)
	errs := make([]error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = e.transfer(buyer, seller, 20, nonce16)
		}()
	}
	wg.Wait()
	firsts := 0
	for i := range results {
		if errs[i] != nil {
			t.Fatalf("worker %d: %v", i, errs[i])
		}
		if !results[i].Replayed {
			firsts++
		}
		if results[i].TransactionID != results[0].TransactionID {
			t.Fatalf("workers saw different rows: %v vs %v", results[i].TransactionID, results[0].TransactionID)
		}
	}
	if firsts != 1 || e.balance(buyer) != 30 || e.balance(seller) != 30 || e.rowCount(buyer) != 1 {
		t.Fatalf("firsts %d balances %d/%d rows %d, want one charge", firsts, e.balance(buyer), e.balance(seller), e.rowCount(buyer))
	}
}

// Opposite transfers A->B and B->A, and user<->system debits and credits on the same wallets, all at
// once: no deadlock, no lost update, coins conserved.
func TestTransferOppositeAndMixedOperationsNeitherDeadlockNorLoseCoins(t *testing.T) {
	e := newItEnv(t)
	a, b := e.user(coinsOf(1000)), e.user(coinsOf(1000))
	const each = 25
	var mu sync.Mutex
	var failures []error
	record := func(err error) {
		if err != nil {
			mu.Lock()
			failures = append(failures, err)
			mu.Unlock()
		}
	}
	var wg sync.WaitGroup
	for i := 0; i < each; i++ {
		i := i
		wg.Add(4)
		go func() { defer wg.Done(); _, err := e.transfer(a, b, 3, nonceN("ab", i)); record(err) }()
		go func() { defer wg.Done(); _, err := e.transfer(b, a, 2, nonceN("ba", i)); record(err) }()
		go func() {
			defer wg.Done()
			record(e.run(func(tx pgx.Tx) error { return Debit(e.ctx, tx, a, 1, "mixed debit") }))
		}()
		go func() {
			defer wg.Done()
			record(e.run(func(tx pgx.Tx) error { return Credit(e.ctx, tx, b, 1, "mixed credit") }))
		}()
	}
	wg.Wait()
	for _, err := range failures {
		if IsRetryable(err) {
			t.Fatalf("deadlock or serialization failure: %v", err)
		}
		t.Fatalf("unexpected failure: %v", err)
	}
	// a: -3 per A->B, +2 per B->A, -1 per debit; b: +3, -2, +1 per credit.
	wantA := int32(1000 + each*(-3+2-1))
	wantB := int32(1000 + each*(3-2+1))
	if got := e.balance(a); got != wantA {
		t.Errorf("a = %d, want %d", got, wantA)
	}
	if got := e.balance(b); got != wantB {
		t.Errorf("b = %d, want %d", got, wantB)
	}
	// The transfers conserve coins; the system debit and credit are the only coins in or out.
	if e.balance(a)+e.balance(b) != 2000+each*(-1+1) {
		t.Errorf("sum = %d, want 2000", e.balance(a)+e.balance(b))
	}
}
