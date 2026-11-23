package ledger

import (
	"bytes"
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	listing = uuid.MustParse(listingA)
	// lowUser sorts before highUser by bytes, so the order of the two wallet locks is observable.
	lowUser  = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	highUser = uuid.MustParse("ffffffff-ffff-4fff-8fff-ffffffffffff")
)

// transferDB is a scripted transaction: it answers the wallet locks from fixed balances, the key lookup
// from an optional stored row, and records every statement in order.
type transferDB struct {
	balances  map[uuid.UUID]int32 // missing user = no wallet row
	stored    *keyedRow
	insertErr error
	log       []string
	lockedBy  []uuid.UUID
	execArgs  [][]any
}

type scanRow func(dest ...any) error

func (f scanRow) Scan(dest ...any) error { return f(dest...) }

func (d *transferDB) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	d.log = append(d.log, sql)
	d.execArgs = append(d.execArgs, args)
	return pgconn.NewCommandTag("OK 1"), nil
}

func (d *transferDB) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	d.log = append(d.log, sql)
	switch {
	case strings.Contains(sql, "FROM public.wallets"):
		user := args[0].(uuid.UUID)
		d.lockedBy = append(d.lockedBy, user)
		balance, ok := d.balances[user]
		return scanRow(func(dest ...any) error {
			if !ok {
				return pgx.ErrNoRows
			}
			*(dest[0].(*uuid.UUID)) = user
			*(dest[1].(*int32)) = balance
			return nil
		})
	case strings.Contains(sql, "FROM public.transactions"):
		return scanRow(func(dest ...any) error {
			if d.stored == nil {
				return pgx.ErrNoRows
			}
			*(dest[0].(*uuid.UUID)) = d.stored.id
			*(dest[1].(*string)) = d.stored.to
			*(dest[2].(*int32)) = d.stored.amount
			*(dest[3].(*string)) = d.stored.currency
			*(dest[4].(*string)) = d.stored.status
			*(dest[5].(**string)) = d.stored.description
			*(dest[6].(*time.Time)) = d.stored.createdAt
			return nil
		})
	default: // the INSERT ... RETURNING
		return scanRow(func(dest ...any) error {
			if d.insertErr != nil {
				return d.insertErr
			}
			*(dest[0].(*uuid.UUID)) = uuid.MustParse("99999999-9999-4999-8999-999999999999")
			*(dest[1].(*time.Time)) = time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
			return nil
		})
	}
}

func (d *transferDB) count(fragment string) int {
	n := 0
	for _, s := range d.log {
		if strings.Contains(s, fragment) {
			n++
		}
	}
	return n
}

func input(buyer, seller uuid.UUID, amount int32) MarketplaceTransferInput {
	key, err := NewMarketplaceKey(listing, amount, nonce16)
	if err != nil {
		panic(err)
	}
	return MarketplaceTransferInput{Key: key, ListingID: listing, Buyer: buyer, Seller: seller, Amount: amount, Description: "Nam bought avatar Alex from Bao"}
}

func TestMarketplaceTransferRejectsBadInputBeforeTheDatabase(t *testing.T) {
	good := input(lowUser, highUser, 20)
	otherListing := good
	otherListing.ListingID = uuid.MustParse("11111111-2222-4333-8444-555555555555")
	otherPrice := good
	otherPrice.Amount = 21
	for _, tc := range []struct {
		name string
		in   MarketplaceTransferInput
		code apperror.Code
	}{
		{"zero amount", func() MarketplaceTransferInput { i := good; i.Amount = 0; return i }(), apperror.CodeValidation},
		{"negative amount", func() MarketplaceTransferInput { i := good; i.Amount = -20; return i }(), apperror.CodeValidation},
		{"self transfer", func() MarketplaceTransferInput { i := good; i.Seller = i.Buyer; return i }(), apperror.CodeValidation},
		{"nil buyer", func() MarketplaceTransferInput { i := good; i.Buyer = uuid.Nil; return i }(), apperror.CodeValidation},
		{"nil seller", func() MarketplaceTransferInput { i := good; i.Seller = uuid.Nil; return i }(), apperror.CodeValidation},
		{"nil listing", func() MarketplaceTransferInput { i := good; i.ListingID = uuid.Nil; return i }(), apperror.CodeValidation},
		{"empty description", func() MarketplaceTransferInput { i := good; i.Description = "  "; return i }(), apperror.CodeValidation},
		{"long description", func() MarketplaceTransferInput { i := good; i.Description = strings.Repeat("é", 256); return i }(), apperror.CodeValidation},
		{"no key", func() MarketplaceTransferInput { i := good; i.Key = ""; return i }(), apperror.CodeCoinTransferKeyInvalid},
		{"slot style key", func() MarketplaceTransferInput { i := good; i.Key = "my-slot-key-1"; return i }(), apperror.CodeCoinTransferKeyInvalid},
		{"key for another listing", otherListing, apperror.CodeCoinTransferKeyConflict},
		{"key for another price", otherPrice, apperror.CodeCoinTransferKeyConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := &transferDB{balances: map[uuid.UUID]int32{lowUser: 50, highUser: 10}}
			_, err := MarketplaceTransfer(context.Background(), db, tc.in)
			assertCode(t, err, tc.code)
			if len(db.log) != 0 {
				t.Fatalf("a rejected input reached the database: %v", db.log)
			}
		})
	}
}

func TestMarketplaceTransferLocksWalletsInAscendingUserIDWhoeverBuys(t *testing.T) {
	for _, tc := range []struct {
		name          string
		buyer, seller uuid.UUID
	}{
		{"low buys from high", lowUser, highUser},
		{"high buys from low", highUser, lowUser},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := &transferDB{balances: map[uuid.UUID]int32{lowUser: 50, highUser: 50}}
			if _, err := MarketplaceTransfer(context.Background(), db, input(tc.buyer, tc.seller, 20)); err != nil {
				t.Fatal(err)
			}
			if len(db.lockedBy) != 2 || bytes.Compare(db.lockedBy[0][:], db.lockedBy[1][:]) >= 0 {
				t.Fatalf("wallet locks = %v, want ascending user id", db.lockedBy)
			}
			// The seller wallet is created before the first lock, and the key is read after both locks.
			if !strings.Contains(db.log[0], "INSERT INTO public.wallets") {
				t.Fatalf("first statement = %q, want the seller wallet insert", db.log[0])
			}
			if !strings.Contains(db.log[3], "idempotency_key") {
				t.Fatalf("statements = %v, want the key lookup after both locks", db.log)
			}
		})
	}
}

func TestMarketplaceTransferWritesBothBalancesAndOneRow(t *testing.T) {
	db := &transferDB{balances: map[uuid.UUID]int32{lowUser: 50, highUser: 10}}
	in := input(lowUser, highUser, 20)
	got, err := MarketplaceTransfer(context.Background(), db, in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Replayed || got.Buyer != lowUser || got.Seller != highUser || got.Amount != 20 || got.Description != in.Description || got.TransactionID == uuid.Nil {
		t.Fatalf("receipt = %+v", got)
	}
	if db.count("UPDATE public.wallets") != 2 || db.count("INSERT INTO public.transactions") != 1 {
		t.Fatalf("statements = %v, want two wallet updates and one row", db.log)
	}
	balances := map[uuid.UUID]int32{}
	for _, args := range db.execArgs {
		if len(args) == 2 {
			if id, ok := args[1].(uuid.UUID); ok {
				balances[id] = args[0].(int32)
			}
		}
	}
	if balances[lowUser] != 30 || balances[highUser] != 30 {
		t.Fatalf("new balances = %v, want 30 and 30", balances)
	}
}

func TestMarketplaceTransferRefusalsWriteNothing(t *testing.T) {
	for _, tc := range []struct {
		name     string
		balances map[uuid.UUID]int32
		code     apperror.Code
	}{
		{"buyer short", map[uuid.UUID]int32{lowUser: 19, highUser: 10}, apperror.CodeInsufficientBalance},
		{"buyer has no wallet", map[uuid.UUID]int32{highUser: 10}, apperror.CodeInsufficientBalance},
		{"seller would pass max int32", map[uuid.UUID]int32{lowUser: 50, highUser: math.MaxInt32 - 19}, apperror.CodeCoinRecipientBalanceLimit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := &transferDB{balances: tc.balances}
			_, err := MarketplaceTransfer(context.Background(), db, input(lowUser, highUser, 20))
			assertCode(t, err, tc.code)
			if db.count("UPDATE public.wallets") != 0 || db.count("INSERT INTO public.transactions") != 0 {
				t.Fatalf("statements = %v, want no balance change or row", db.log)
			}
		})
	}
}

func TestMarketplaceTransferExactBalanceAndMaxBoundaryPass(t *testing.T) {
	db := &transferDB{balances: map[uuid.UUID]int32{lowUser: 20, highUser: math.MaxInt32 - 20}}
	if _, err := MarketplaceTransfer(context.Background(), db, input(lowUser, highUser, 20)); err != nil {
		t.Fatalf("spending the whole balance into a wallet that lands on MaxInt32: %v", err)
	}
}

func TestMarketplaceTransferReplayWritesNothingAndReturnsTheStoredRow(t *testing.T) {
	desc := "stored description"
	created := time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC)
	stored := uuid.MustParse("12345678-1234-4234-8234-123456789012")
	// The buyer now has less than the price: a replay must not turn into INSUFFICIENT_BALANCE.
	db := &transferDB{
		balances: map[uuid.UUID]int32{lowUser: 1, highUser: 30},
		stored:   &keyedRow{id: stored, to: highUser.String(), amount: 20, currency: Currency, status: "success", description: &desc, createdAt: created},
	}
	got, err := MarketplaceTransfer(context.Background(), db, input(lowUser, highUser, 20))
	if err != nil {
		t.Fatal(err)
	}
	if !got.Replayed || got.TransactionID != stored || got.Description != desc || !got.CreatedAt.Equal(created) {
		t.Fatalf("receipt = %+v, want the stored row flagged as a replay", got)
	}
	if db.count("UPDATE public.wallets") != 0 || db.count("INSERT INTO public.transactions") != 0 {
		t.Fatalf("a replay wrote: %v", db.log)
	}
}

func TestMarketplaceTransferStoredRowOfAnotherPurposeIsAConflict(t *testing.T) {
	for _, tc := range []struct {
		name string
		row  keyedRow
	}{
		{"slot purchase row (to system)", keyedRow{to: Counterparty, amount: 20, currency: Currency, status: "success"}},
		{"another seller", keyedRow{to: uuid.New().String(), amount: 20, currency: Currency, status: "success"}},
		{"another amount", keyedRow{to: highUser.String(), amount: 19, currency: Currency, status: "success"}},
		{"not COIN", keyedRow{to: highUser.String(), amount: 20, currency: "VND", status: "success"}},
		{"pending row", keyedRow{to: highUser.String(), amount: 20, currency: Currency, status: "pending"}},
		{"failed row", keyedRow{to: highUser.String(), amount: 20, currency: Currency, status: "failed"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := tc.row
			db := &transferDB{balances: map[uuid.UUID]int32{lowUser: 50, highUser: 10}, stored: &row}
			_, err := MarketplaceTransfer(context.Background(), db, input(lowUser, highUser, 20))
			assertCode(t, err, apperror.CodeCoinTransferKeyConflict)
			if db.count("UPDATE public.wallets") != 0 || db.count("INSERT INTO public.transactions") != 0 {
				t.Fatalf("a conflict wrote: %v", db.log)
			}
		})
	}
}

func TestMarketplaceTransferUniqueViolationOnInsertIsAConflict(t *testing.T) {
	db := &transferDB{
		balances:  map[uuid.UUID]int32{lowUser: 50, highUser: 10},
		insertErr: &pgconn.PgError{Code: "23505", ConstraintName: "transactions_from_idempotency_key_key"},
	}
	_, err := MarketplaceTransfer(context.Background(), db, input(lowUser, highUser, 20))
	assertCode(t, err, apperror.CodeCoinTransferKeyConflict)
}

func TestMarketplaceTransferOtherInsertErrorsAreInternalAndRetryableOnesStayVisible(t *testing.T) {
	db := &transferDB{
		balances:  map[uuid.UUID]int32{lowUser: 50, highUser: 10},
		insertErr: &pgconn.PgError{Code: "40P01"},
	}
	_, err := MarketplaceTransfer(context.Background(), db, input(lowUser, highUser, 20))
	assertCode(t, err, apperror.CodeInternal)
	if !IsRetryable(err) {
		t.Fatalf("IsRetryable(%v) = false, want true for a deadlock", err)
	}
}

func TestIsRetryable(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"deadlock", &pgconn.PgError{Code: "40P01"}, true},
		{"serialization", &pgconn.PgError{Code: "40001"}, true},
		{"wrapped", apperror.Wrap(apperror.CodeInternal, "x", &pgconn.PgError{Code: "40P01"}), true},
		{"unique violation", &pgconn.PgError{Code: "23505"}, false},
		{"plain", errors.New("boom"), false},
		{"nil", nil, false},
	} {
		if got := IsRetryable(tc.err); got != tc.want {
			t.Errorf("%s: IsRetryable = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestMarketplaceTransferLogDoesNotCarryKeyOrDescription(t *testing.T) {
	ctx, logs := observedContext()
	db := &transferDB{balances: map[uuid.UUID]int32{lowUser: 50, highUser: 10}}
	in := input(lowUser, highUser, 20)
	in.Description = sentinelDescription
	if _, err := MarketplaceTransfer(ctx, db, in); err != nil {
		t.Fatal(err)
	}
	got := onlyLog(t, logs, "wallet transfer applied in transaction")
	if got["op"] != "wallet.transfer" || got["currency"] != Currency {
		t.Fatalf("log fields = %v", got)
	}
	rendered := renderLogs(logs)
	if strings.Contains(rendered, sentinelDescription) || strings.Contains(rendered, nonce16) {
		t.Fatalf("description or key nonce reached the log:\n%s", rendered)
	}
}
