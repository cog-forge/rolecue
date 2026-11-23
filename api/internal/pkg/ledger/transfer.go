package ledger

import (
	"bytes"
	"context"
	"errors"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cog-forge/rolecue/api/internal/pkg/ledger/repository"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
)

// MaxDescriptionRunes bounds the human-readable text of a transfer row.
const MaxDescriptionRunes = 255

const (
	statusSuccess      = "success"
	pgUniqueViolation  = "23505"
	pgDeadlockDetected = "40P01"
	pgSerialization    = "40001"
)

// MarketplaceTransferInput is one purchase intent: Buyer pays Amount coins to Seller for ListingID, under
// Key. Key must be a Marketplace key (see ParseMarketplaceKey) whose listing id is ListingID and whose
// price is Amount; the caller takes Amount from the listing it re-read under its own lock, never from the
// request. Description is for people reading the history; the ledger never reads it back.
type MarketplaceTransferInput struct {
	Key         string
	ListingID   uuid.UUID
	Buyer       uuid.UUID
	Seller      uuid.UUID
	Amount      int32
	Description string
}

// MarketplaceReceipt is the single COIN row of one purchase intent. Replayed is false when this call wrote
// the row and true when the row already existed and this call changed nothing. On a replay the receipt is
// the stored row, so Amount, parties, Description and CreatedAt are those of the first success.
type MarketplaceReceipt struct {
	TransactionID uuid.UUID
	Buyer         uuid.UUID
	Seller        uuid.UUID
	Amount        int32
	Description   string
	CreatedAt     time.Time
	Replayed      bool
}

// MarketplaceTransfer moves Amount coins from Buyer to Seller, in full and with no fee, and writes one
// COIN row (from Buyer, to Seller, status success, the key in idempotency_key), all inside the caller's
// transaction. It never begins, commits or rolls back: when it returns an error the caller must roll its
// transaction back, and the balances and rows are then as they were. A nil error is not proof of commit.
//
// Order of work, none of it reaching the database until the checks before it pass:
//  1. the amount, the parties and the description are checked (VALIDATION_ERROR);
//  2. the key is parsed (COIN_TRANSFER_KEY_INVALID) and its listing id and price must equal the input
//     (COIN_TRANSFER_KEY_CONFLICT);
//  3. the Seller wallet is created if it is missing, then both wallet rows are locked in ascending user id,
//     the order documented in api/docs/marketplace-coins.md;
//  4. the Buyer's row for the key is read under those locks. A row that is this transfer is returned with
//     Replayed true and nothing is written, even if the balance has changed since. A row for another
//     purpose or another party or amount is COIN_TRANSFER_KEY_CONFLICT;
//  5. otherwise the Buyer's balance is checked (INSUFFICIENT_BALANCE, also for a missing wallet), the
//     Seller's balance must stay within MaxInt32 (COIN_RECIPIENT_BALANCE_LIMIT), both wallets are updated
//     and the row is inserted.
//
// The caller must already hold every row lock it needs besides wallets (buyer inventory, listing) because
// wallet locks come last. A caller that gets a deadlock or serialization failure (IsRetryable) rolls back
// and may rerun the whole transaction a bounded number of times with the same key.
func MarketplaceTransfer(ctx context.Context, db DBTX, in MarketplaceTransferInput) (MarketplaceReceipt, error) {
	key, err := checkTransferInput(in)
	if err != nil {
		return MarketplaceReceipt{}, err
	}
	// The seller wallet is created before any FOR UPDATE so the locking below is plain and sorted.
	if err := ensureWallet(ctx, db, in.Seller); err != nil {
		return MarketplaceReceipt{}, err
	}
	var buyer, seller lockedWallet
	for _, id := range lockOrder(in.Buyer, in.Seller) {
		w, err := lockWalletRow(ctx, db, id)
		if err != nil {
			return MarketplaceReceipt{}, err
		}
		if id == in.Buyer {
			buyer = w
		} else {
			seller = w
		}
	}
	existing, found, err := findKeyedRow(ctx, db, in.Buyer, in.Key)
	if err != nil {
		return MarketplaceReceipt{}, err
	}
	if found {
		return replayOf(existing, in)
	}
	if !buyer.found || buyer.balance < in.Amount {
		return MarketplaceReceipt{}, insufficientBalance()
	}
	if !seller.found {
		return MarketplaceReceipt{}, persistence(pgx.ErrNoRows)
	}
	if int64(seller.balance)+int64(in.Amount) > math.MaxInt32 {
		return MarketplaceReceipt{}, apperror.New(apperror.CodeCoinRecipientBalanceLimit, "recipient wallet cannot hold this amount")
	}
	for _, move := range []struct {
		w       lockedWallet
		balance int32
	}{{buyer, buyer.balance - in.Amount}, {seller, seller.balance + in.Amount}} {
		if err := setBalance(ctx, db, move.w.id, move.balance); err != nil {
			return MarketplaceReceipt{}, err
		}
	}
	receipt := MarketplaceReceipt{Buyer: in.Buyer, Seller: in.Seller, Amount: in.Amount, Description: in.Description}
	inserted, err := queries(db).InsertKeyedCoinTransactionReturning(ctx, repository.InsertKeyedCoinTransactionReturningParams{
		FromParty: in.Buyer.String(), ToParty: in.Seller.String(), Amount: in.Amount, Currency: Currency,
		Description: textPtr(in.Description), IdempotencyKey: textPtr(in.Key)})
	if err != nil {
		return MarketplaceReceipt{}, keyInsertError(err)
	}
	receipt.TransactionID, receipt.CreatedAt = inserted.ID, inserted.CreatedAt
	// The line says "in transaction": it is final only if the caller's transaction commits. The key and the
	// description are not logged.
	logger.FromContext(ctx).Info("wallet transfer applied in transaction", logger.Op("wallet.transfer"),
		zap.String("buyer_id", in.Buyer.String()), zap.String("seller_id", in.Seller.String()),
		zap.String("listing_id", key.ListingID.String()), zap.Int32("amount", in.Amount), zap.String("currency", Currency))
	return receipt, nil
}

// IsRetryable reports a PostgreSQL deadlock (40P01) or serialization failure (40001) anywhere in the error
// chain. MarketplaceTransfer takes the wallet locks in one fixed order, so it cannot deadlock with
// itself or with the single-wallet operations; the check is for locks the caller took in another order.
func IsRetryable(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && (pgErr.Code == pgDeadlockDetected || pgErr.Code == pgSerialization)
}

func checkTransferInput(in MarketplaceTransferInput) (MarketplaceKey, error) {
	switch {
	case in.Amount <= 0:
		return MarketplaceKey{}, apperror.New(apperror.CodeValidation, "coin amount must be positive")
	case in.Buyer == uuid.Nil || in.Seller == uuid.Nil || in.ListingID == uuid.Nil:
		return MarketplaceKey{}, apperror.New(apperror.CodeValidation, "buyer, seller and listing are required")
	case in.Buyer == in.Seller:
		return MarketplaceKey{}, apperror.New(apperror.CodeValidation, "buyer and seller must differ")
	case strings.TrimSpace(in.Description) == "" || utf8.RuneCountInString(in.Description) > MaxDescriptionRunes || !utf8.ValidString(in.Description):
		return MarketplaceKey{}, apperror.New(apperror.CodeValidation, "transfer description must be 1 to 255 characters")
	}
	key, err := ParseMarketplaceKey(in.Key)
	if err != nil {
		return MarketplaceKey{}, err
	}
	if key.ListingID != in.ListingID || key.Price != in.Amount {
		return MarketplaceKey{}, keyConflict()
	}
	return key, nil
}

// lockOrder is ascending user id by raw bytes, the order every two-wallet operation must use.
func lockOrder(a, b uuid.UUID) [2]uuid.UUID {
	if bytes.Compare(a[:], b[:]) <= 0 {
		return [2]uuid.UUID{a, b}
	}
	return [2]uuid.UUID{b, a}
}

type lockedWallet struct {
	id      uuid.UUID
	balance int32
	found   bool
}

func lockWalletRow(ctx context.Context, db DBTX, userID uuid.UUID) (lockedWallet, error) {
	id, balance, found, err := lockWallet(ctx, db, userID)
	return lockedWallet{id: id, balance: balance, found: found}, err
}

// keyedRow is the stored part of an earlier row that carries the key.
type keyedRow struct {
	id          uuid.UUID
	to          string
	amount      int32
	currency    string
	status      string
	description *string
	createdAt   time.Time
}

// findKeyedRow reads the buyer's row for the key. The unique index is on ("from", idempotency_key), so
// only the buyer's own rows can match.
func findKeyedRow(ctx context.Context, db DBTX, buyer uuid.UUID, key string) (keyedRow, bool, error) {
	row, err := queries(db).FindKeyedTransaction(ctx, repository.FindKeyedTransactionParams{
		FromParty: buyer.String(), IdempotencyKey: textPtr(key)})
	if errors.Is(err, pgx.ErrNoRows) {
		return keyedRow{}, false, nil
	}
	if err != nil {
		return keyedRow{}, false, persistence(err)
	}
	return keyedRow{id: row.ID, to: row.To, amount: row.Amount, currency: row.Currency, status: row.Status,
		description: row.Description, createdAt: row.CreatedAt}, true, nil
}

// replayOf accepts the stored row only when it is this very transfer: a completed COIN row from the buyer
// (guaranteed by the lookup) to the same seller for the same amount. A slot purchase row has "system" as
// its recipient and can never match.
func replayOf(row keyedRow, in MarketplaceTransferInput) (MarketplaceReceipt, error) {
	if row.to != in.Seller.String() || row.amount != in.Amount || row.currency != Currency || row.status != statusSuccess {
		return MarketplaceReceipt{}, keyConflict()
	}
	receipt := MarketplaceReceipt{
		TransactionID: row.id, Buyer: in.Buyer, Seller: in.Seller, Amount: row.amount, CreatedAt: row.createdAt, Replayed: true,
	}
	if row.description != nil {
		receipt.Description = *row.description
	}
	return receipt, nil
}

// keyInsertError turns the unique index on the key into a conflict. The lookup under the wallet locks
// makes it unreachable for one buyer; it is the backstop. The statement failed inside the caller's
// transaction, which is now aborted: the caller rolls back.
func keyInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return keyConflict()
	}
	return persistence(err)
}

func keyConflict() error {
	return apperror.New(apperror.CodeCoinTransferKeyConflict, "idempotency key is already used for a different transfer")
}
