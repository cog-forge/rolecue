package ledger

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var userID = uuid.MustParse("11111111-1111-4111-8111-111111111111")

// fakeDB records the statements the ledger runs and answers the wallet read from a fixed balance.
// A nil wallet means the user has no wallet row yet. The fake creates the row when the ledger
// inserts it, the way the database would.
type fakeDB struct {
	wallet     *int32
	queryErr   error
	execErr    error
	statements []string
	args       [][]any
}

type fakeRow struct {
	found   bool
	balance int32
	err     error
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if !r.found {
		return pgx.ErrNoRows
	}
	*(dest[0].(*uuid.UUID)) = uuid.MustParse("22222222-2222-4222-8222-222222222222")
	*(dest[1].(*int32)) = r.balance
	return nil
}

func (f *fakeDB) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	if f.queryErr != nil {
		return fakeRow{err: f.queryErr}
	}
	if f.wallet == nil {
		return fakeRow{}
	}
	return fakeRow{found: true, balance: *f.wallet}
}

func (f *fakeDB) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.statements = append(f.statements, sql)
	f.args = append(f.args, args)
	if f.execErr != nil {
		return pgconn.CommandTag{}, f.execErr
	}
	if strings.Contains(sql, "INSERT INTO public.wallets") && f.wallet == nil {
		zero := int32(0)
		f.wallet = &zero
	}
	return pgconn.NewCommandTag("OK 1"), nil
}

func walletBalance(n int32) *int32 { return &n }

func (f *fakeDB) count(fragment string) int {
	n := 0
	for _, sql := range f.statements {
		if strings.Contains(sql, fragment) {
			n++
		}
	}
	return n
}

func assertCode(t *testing.T, err error, want apperror.Code) {
	t.Helper()
	if !apperror.IsCode(err, want) {
		t.Fatalf("error = %v, want code %s", err, want)
	}
}

func TestDebitRejectsNonPositiveAmounts(t *testing.T) {
	for _, amount := range []int32{0, -1} {
		db := &fakeDB{wallet: walletBalance(100)}
		assertCode(t, Debit(context.Background(), db, userID, amount, "x"), apperror.CodeValidation)
		if len(db.statements) != 0 {
			t.Fatalf("amount %d reached the database: %v", amount, db.statements)
		}
	}
}

func TestDebitLowersBalanceAndWritesOneCoinRow(t *testing.T) {
	db := &fakeDB{wallet: walletBalance(100)}
	if err := Debit(context.Background(), db, userID, 5, "Interview capacity for posting"); err != nil {
		t.Fatal(err)
	}
	if db.count("UPDATE public.wallets") != 1 || db.count("INSERT INTO public.transactions") != 1 {
		t.Fatalf("statements = %v, want one wallet update and one transaction row", db.statements)
	}
	// Exec calls are recorded in order: the wallet UPDATE args are (wallet id, new balance).
	if got := db.args[0]; got[1] != int32(95) {
		t.Fatalf("UPDATE args = %v, want new balance 95", got)
	}
	row := db.args[1]
	if row[0] != userID.String() || row[1] != Counterparty || row[2] != int32(5) || row[3] != Currency {
		t.Fatalf("transaction args = %v, want from user, to system, 5 COIN", row)
	}
}

func TestDebitOverBalanceChangesNothing(t *testing.T) {
	db := &fakeDB{wallet: walletBalance(3)}
	assertCode(t, Debit(context.Background(), db, userID, 5, "x"), apperror.CodeInsufficientBalance)
	if len(db.statements) != 0 {
		t.Fatalf("statements = %v, want none after an insufficient balance", db.statements)
	}
}

func TestDebitWithoutWalletIsInsufficient(t *testing.T) {
	db := &fakeDB{}
	assertCode(t, Debit(context.Background(), db, userID, 1, "x"), apperror.CodeInsufficientBalance)
	if len(db.statements) != 0 {
		t.Fatalf("statements = %v, want none for a missing wallet", db.statements)
	}
}

func TestDebitReadErrorIsInternal(t *testing.T) {
	db := &fakeDB{queryErr: errors.New("connection reset")}
	assertCode(t, Debit(context.Background(), db, userID, 1, "x"), apperror.CodeInternal)
}

func TestCreditCreatesMissingWalletThenAddsCoins(t *testing.T) {
	db := &fakeDB{}
	if err := Credit(context.Background(), db, userID, 4, "Refund of unused interview capacity"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(db.statements[0], "ON CONFLICT (user_id) DO NOTHING") {
		t.Fatalf("first statement = %q, want the wallet insert", db.statements[0])
	}
	if db.count("UPDATE public.wallets") != 1 {
		t.Fatalf("statements = %v, want one wallet update", db.statements)
	}
	row := db.args[len(db.args)-1]
	if row[0] != Counterparty || row[1] != userID.String() || row[2] != int32(4) {
		t.Fatalf("transaction args = %v, want from system to user, 4 COIN", row)
	}
}

func TestCreditPastMaxInt32ChangesNothing(t *testing.T) {
	db := &fakeDB{wallet: walletBalance(math.MaxInt32 - 1)}
	assertCode(t, Credit(context.Background(), db, userID, 5, "x"), apperror.CodeInternal)
	if db.count("UPDATE public.wallets") != 0 || db.count("INSERT INTO public.transactions") != 0 {
		t.Fatalf("statements = %v, want no balance change or row on overflow", db.statements)
	}
}

func TestCreditRejectsNonPositiveAmounts(t *testing.T) {
	db := &fakeDB{}
	assertCode(t, Credit(context.Background(), db, userID, 0, "x"), apperror.CodeValidation)
	if len(db.statements) != 0 {
		t.Fatalf("statements = %v, want none", db.statements)
	}
}

func TestWriteErrorsAreInternal(t *testing.T) {
	db := &fakeDB{wallet: walletBalance(100), execErr: errors.New("disk full")}
	assertCode(t, Debit(context.Background(), db, userID, 1, "x"), apperror.CodeInternal)
}
