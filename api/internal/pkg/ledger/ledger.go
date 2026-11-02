// Package ledger moves interview coins between a user's wallet and the system inside the caller's
// database transaction. It writes the wallet balance and one COIN row in the transactions table, except
// CreditForOrder, whose caller already holds the order row. It never calls the payment provider (KTD3).
// It does no deduplication: callers make their writes one-shot through the state they hold under a
// row lock.
package ledger

import (
	"context"
	"errors"
	"math"

	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
)

// Currency labels coin movements. It is not VND: coins are internal, not money.
const Currency = "COIN"

// Counterparty is the system side of every coin movement, as the inventory purchase writes it.
const Counterparty = "system"

// DBTX is the part of a pgx transaction or pool that the ledger uses. Callers pass the open
// transaction they hold a lock in, so the wallet write and the caller's own writes commit together.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Debit takes amount coins out of the user's wallet and records one COIN row from the user to the
// system. The wallet row is locked first. A missing wallet or a balance below amount is
// INSUFFICIENT_BALANCE and changes nothing.
func Debit(ctx context.Context, db DBTX, userID uuid.UUID, amount int32, description string) error {
	if err := checkAmount(amount); err != nil {
		return err
	}
	walletID, balance, found, err := lockWallet(ctx, db, userID)
	if err != nil {
		return err
	}
	if !found || balance < amount {
		return insufficientBalance()
	}
	if _, err := db.Exec(ctx, `UPDATE public.wallets SET balance = $2 WHERE id = $1`, walletID, balance-amount); err != nil {
		return persistence(err)
	}
	if err := insertRow(ctx, db, userID.String(), Counterparty, amount, description); err != nil {
		return err
	}
	// The description is not logged: callers build it from posting titles, and the COIN row already keeps it.
	logger.FromContext(ctx).Info("wallet debit applied in transaction", logger.Op("wallet.debit"),
		zap.String("user_id", userID.String()), zap.Int32("amount", amount), zap.Int32("balance_after", balance-amount),
		zap.String("currency", Currency))
	return nil
}

// Credit adds amount coins to the user's wallet and records one COIN row from the system to the
// user. A missing wallet is created at zero. A balance that would pass MaxInt32 is refused and
// changes nothing.
func Credit(ctx context.Context, db DBTX, userID uuid.UUID, amount int32, description string) error {
	if err := checkAmount(amount); err != nil {
		return err
	}
	balanceAfter, err := addToWallet(ctx, db, userID, amount)
	if err != nil {
		return err
	}
	if err := insertRow(ctx, db, Counterparty, userID.String(), amount, description); err != nil {
		return err
	}
	logCredit(ctx, "wallet.credit", userID, amount, balanceAfter, "")
	return nil
}

// CreditForOrder adds amount coins to the user's wallet for an order whose transactions row already
// exists, such as a paid PayOS top-up, so it writes no row of its own. A missing wallet is created at
// zero and a balance that would pass MaxInt32 is refused and changes nothing. The caller holds the lock
// on the order row and moves it to success in the same database transaction.
func CreditForOrder(ctx context.Context, db DBTX, userID uuid.UUID, amount int32) error {
	if err := checkAmount(amount); err != nil {
		return err
	}
	balanceAfter, err := addToWallet(ctx, db, userID, amount)
	if err != nil {
		return err
	}
	logCredit(ctx, "wallet.credit_order", userID, amount, balanceAfter, "paid_order")
	return nil
}

// logCredit records a coin credit. The write belongs to the caller's transaction, so the line is
// final only once that transaction commits.
func logCredit(ctx context.Context, op string, userID uuid.UUID, amount, balanceAfter int32, reason string) {
	fields := []zap.Field{logger.Op(op), zap.String("user_id", userID.String()),
		zap.Int32("amount", amount), zap.Int32("balance_after", balanceAfter), zap.String("currency", Currency)}
	if reason != "" {
		fields = append(fields, logger.String("reason", reason))
	}
	logger.FromContext(ctx).Info("wallet credit applied in transaction", fields...)
}

// addToWallet creates the wallet if it is missing, locks it, raises the balance by amount and returns the new balance.
func addToWallet(ctx context.Context, db DBTX, userID uuid.UUID, amount int32) (int32, error) {
	if _, err := db.Exec(ctx,
		`INSERT INTO public.wallets (user_id, balance) VALUES ($1, 0) ON CONFLICT (user_id) DO NOTHING`, userID); err != nil {
		return 0, persistence(err)
	}
	walletID, balance, found, err := lockWallet(ctx, db, userID)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, persistence(pgx.ErrNoRows)
	}
	if int64(balance)+int64(amount) > math.MaxInt32 {
		return 0, apperror.New(apperror.CodeInternal, "wallet balance would exceed the maximum")
	}
	if _, err := db.Exec(ctx, `UPDATE public.wallets SET balance = $2 WHERE id = $1`, walletID, balance+amount); err != nil {
		return 0, persistence(err)
	}
	return balance + amount, nil
}

func insufficientBalance() error {
	return apperror.New(apperror.CodeInsufficientBalance, "insufficient coin balance")
}

func checkAmount(amount int32) error {
	if amount <= 0 {
		return apperror.New(apperror.CodeValidation, "coin amount must be positive")
	}
	return nil
}

// lockWallet reads and locks the user's wallet row. found is false when the user has no wallet.
func lockWallet(ctx context.Context, db DBTX, userID uuid.UUID) (walletID uuid.UUID, balance int32, found bool, err error) {
	err = db.QueryRow(ctx,
		`SELECT id, balance FROM public.wallets WHERE user_id = $1 FOR UPDATE`, userID).Scan(&walletID, &balance)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, 0, false, nil
	}
	if err != nil {
		return uuid.Nil, 0, false, persistence(err)
	}
	return walletID, balance, true, nil
}

func insertRow(ctx context.Context, db DBTX, from, to string, amount int32, description string) error {
	if _, err := db.Exec(ctx,
		`INSERT INTO public.transactions ("from", "to", amount, currency, description, status)
		 VALUES ($1, $2, $3, $4, $5, 'success')`,
		from, to, amount, Currency, description); err != nil {
		return persistence(err)
	}
	return nil
}

// persistence keeps AppErrors and maps anything else from the database to an internal error.
func persistence(err error) error {
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return err
	}
	return apperror.Wrap(apperror.CodeInternal, "coin ledger persistence failed", err)
}
