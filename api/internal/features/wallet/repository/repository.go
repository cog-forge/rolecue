package repository

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/wallet/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool    *pgxpool.Pool
	queries *Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{
		pool:    pool,
		queries: New(pool),
	}
}

func (r *Repository) ExecTx(ctx context.Context, fn func(q *Queries) error) error {
	if r.pool == nil {
		return fn(r.queries)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "failed to begin transaction", err)
	}
	q := New(tx)
	if err := fn(q); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "failed to commit transaction", err)
	}
	return nil
}

func (r *Repository) GetQueries() *Queries {
	return r.queries
}

func (r *Repository) GetWalletByUserID(ctx context.Context, userID uuid.UUID) (domain.Wallet, error) {
	row, err := r.queries.GetWalletByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Wallet{}, apperror.New(apperror.CodeWalletNotFound, "wallet not found")
		}
		return domain.Wallet{}, apperror.Wrap(apperror.CodeInternal, "failed to get wallet", err)
	}
	return domain.Wallet{
		ID:      row.ID,
		UserID:  row.UserID,
		Balance: row.Balance,
	}, nil
}

func (r *Repository) CreateWallet(ctx context.Context, q *Queries, userID uuid.UUID) (domain.Wallet, error) {
	if q == nil {
		q = r.queries
	}
	row, err := q.CreateWallet(ctx, userID)
	if err != nil {
		return domain.Wallet{}, apperror.Wrap(apperror.CodeInternal, "failed to create wallet", err)
	}
	return domain.Wallet{
		ID:      row.ID,
		UserID:  row.UserID,
		Balance: row.Balance,
	}, nil
}

func (r *Repository) GetWalletForUpdate(ctx context.Context, q *Queries, userID uuid.UUID) (domain.Wallet, error) {
	if q == nil {
		q = r.queries
	}
	row, err := q.GetWalletByUserIDForUpdate(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Wallet{}, apperror.New(apperror.CodeWalletNotFound, "wallet not found")
		}
		return domain.Wallet{}, apperror.Wrap(apperror.CodeInternal, "failed to get wallet for update", err)
	}
	return domain.Wallet{
		ID:      row.ID,
		UserID:  row.UserID,
		Balance: row.Balance,
	}, nil
}

func (r *Repository) UpdateWalletBalance(ctx context.Context, q *Queries, walletID uuid.UUID, amount int32) (domain.Wallet, error) {
	if q == nil {
		q = r.queries
	}
	row, err := q.UpdateWalletBalance(ctx, UpdateWalletBalanceParams{
		ID:      walletID,
		Balance: amount,
	})
	if err != nil {
		return domain.Wallet{}, apperror.Wrap(apperror.CodeInternal, "failed to update wallet balance", err)
	}
	return domain.Wallet{
		ID:      row.ID,
		UserID:  row.UserID,
		Balance: row.Balance,
	}, nil
}

func (r *Repository) InsertTransaction(ctx context.Context, q *Queries, tx domain.Transaction) (domain.Transaction, error) {
	if q == nil {
		q = r.queries
	}
	var desc *string
	if tx.Description != "" {
		desc = &tx.Description
	}
	fromStr := formatUUIDPtr(tx.From)
	if fromStr == "" {
		if tx.PayOSOrderCode != nil {
			fromStr = "Banking (PayOS)"
		} else {
			fromStr = "System"
		}
	}
	toStr := formatUUIDPtr(tx.To)
	if toStr == "" {
		toStr = "System"
	}

	row, err := q.InsertTransaction(ctx, InsertTransactionParams{
		TxFrom:         fromStr,
		TxTo:           toStr,
		Amount:         tx.Amount,
		Currency:       tx.Currency,
		Description:    desc,
		Status:         TransactionStatus(tx.Status),
		PayosOrderCode: formatInt64Ptr(tx.PayOSOrderCode),
	})
	if err != nil {
		return domain.Transaction{}, apperror.Wrap(apperror.CodeInternal, "failed to insert transaction", err)
	}
	return toDomainTransaction(row.ID, row.From, row.To, row.Amount, row.Currency, row.Description, row.Status, row.PayosOrderCode), nil
}

func (r *Repository) GetTransactionByID(ctx context.Context, id uuid.UUID) (domain.Transaction, error) {
	row, err := r.queries.GetTransactionByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Transaction{}, apperror.New(apperror.CodeTransactionNotFound, "transaction not found")
		}
		return domain.Transaction{}, apperror.Wrap(apperror.CodeInternal, "failed to get transaction", err)
	}
	return toDomainTransaction(row.ID, row.From, row.To, row.Amount, row.Currency, row.Description, row.Status, row.PayosOrderCode), nil
}

func (r *Repository) GetTransactionByPayOSOrderCode(ctx context.Context, orderCode int64) (domain.Transaction, error) {
	orderCodeStr := strconv.FormatInt(orderCode, 10)
	row, err := r.queries.GetTransactionByPayOSOrderCode(ctx, &orderCodeStr)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Transaction{}, apperror.New(apperror.CodeTransactionNotFound, "transaction not found")
		}
		return domain.Transaction{}, apperror.Wrap(apperror.CodeInternal, "failed to get transaction by order code", err)
	}
	return toDomainTransaction(row.ID, row.From, row.To, row.Amount, row.Currency, row.Description, row.Status, row.PayosOrderCode), nil
}

func (r *Repository) GetTransactionByPayOSOrderCodeForUpdate(ctx context.Context, q *Queries, orderCode int64) (domain.Transaction, error) {
	if q == nil {
		q = r.queries
	}
	orderCodeStr := strconv.FormatInt(orderCode, 10)
	row, err := q.GetTransactionByPayOSOrderCodeForUpdate(ctx, &orderCodeStr)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Transaction{}, apperror.New(apperror.CodeTransactionNotFound, "transaction not found")
		}
		return domain.Transaction{}, apperror.Wrap(apperror.CodeInternal, "failed to get transaction for update", err)
	}
	return toDomainTransaction(row.ID, row.From, row.To, row.Amount, row.Currency, row.Description, row.Status, row.PayosOrderCode), nil
}

func (r *Repository) UpdateTransactionStatus(ctx context.Context, q *Queries, id uuid.UUID, status domain.TransactionStatus) (domain.Transaction, error) {
	if q == nil {
		q = r.queries
	}
	row, err := q.UpdateTransactionStatus(ctx, UpdateTransactionStatusParams{
		ID:     id,
		Status: TransactionStatus(status),
	})
	if err != nil {
		return domain.Transaction{}, apperror.Wrap(apperror.CodeInternal, "failed to update transaction status", err)
	}
	return toDomainTransaction(row.ID, row.From, row.To, row.Amount, row.Currency, row.Description, row.Status, row.PayosOrderCode), nil
}

func (r *Repository) ListTransactionsByUserID(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]domain.Transaction, int64, error) {
	rows, err := r.queries.ListTransactionsByUserID(ctx, ListTransactionsByUserIDParams{
		TxUserID:   userID.String(),
		PageLimit:  limit,
		PageOffset: offset,
	})
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeInternal, "failed to list transactions", err)
	}

	total, err := r.queries.CountTransactionsByUserID(ctx, userID.String())
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeInternal, "failed to count transactions", err)
	}

	txs := make([]domain.Transaction, 0, len(rows))
	for _, row := range rows {
		txs = append(txs, toDomainTransaction(row.ID, row.From, row.To, row.Amount, row.Currency, row.Description, row.Status, row.PayosOrderCode))
	}
	return txs, total, nil
}

func (r *Repository) ListCoinPackages(ctx context.Context) ([]domain.CoinPackage, error) {
	rows, err := r.queries.ListActiveCoinPackages(ctx)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to list coin packages", err)
	}
	packages := make([]domain.CoinPackage, 0, len(rows))
	for _, row := range rows {
		desc := ""
		if row.Description != nil {
			desc = *row.Description
		}
		packages = append(packages, domain.CoinPackage{
			ID:          row.ID,
			Name:        row.Name,
			PriceVND:    row.Price,
			Description: desc,
			CoinAmount:  row.Amount,
			Currency:    row.Currency,
			IsActive:    true,
		})
	}
	return packages, nil
}

func (r *Repository) GetCoinPackageByID(ctx context.Context, id uuid.UUID) (domain.CoinPackage, error) {
	row, err := r.queries.GetCoinPackageByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.CoinPackage{}, apperror.New(apperror.CodeCoinPackageNotFound, "coin package not found")
		}
		return domain.CoinPackage{}, apperror.Wrap(apperror.CodeInternal, "failed to get coin package", err)
	}
	desc := ""
	if row.Description != nil {
		desc = *row.Description
	}
	return domain.CoinPackage{
		ID:          row.ID,
		Name:        row.Name,
		PriceVND:    row.Price,
		Description: desc,
		CoinAmount:  row.Amount,
		Currency:    row.Currency,
		IsActive:    true,
	}, nil
}

func parseUUIDPtr(s string) *uuid.UUID {
	if s == "" {
		return nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil
	}
	return &id
}

func formatUUIDPtr(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

func parseInt64Ptr(s *string) *int64 {
	if s == nil || *s == "" {
		return nil
	}
	val, err := strconv.ParseInt(*s, 10, 64)
	if err != nil {
		return nil
	}
	return &val
}

func formatInt64Ptr(val *int64) *string {
	if val == nil {
		return nil
	}
	s := strconv.FormatInt(*val, 10)
	return &s
}

func toDomainTransaction(id uuid.UUID, fromStr, toStr string, amount int32, currency string, descPtr *string, status TransactionStatus, orderCodePtr *string) domain.Transaction {
	desc := ""
	if descPtr != nil {
		desc = *descPtr
	}
	return domain.Transaction{
		ID:             id,
		From:           parseUUIDPtr(fromStr),
		To:             parseUUIDPtr(toStr),
		Amount:         amount,
		Currency:       currency,
		Description:    desc,
		Status:         domain.TransactionStatus(status),
		PayOSOrderCode: parseInt64Ptr(orderCodePtr),
	}
}

func (r *Repository) ListAllTransactions(ctx context.Context, arg ListAllTransactionsParams) ([]AdminTransaction, error) {
	rows, err := r.queries.ListAllTransactions(ctx, arg)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to list all transactions", err)
	}
	return rows, nil
}

func (r *Repository) CountAllTransactions(ctx context.Context, arg ListAllTransactionsParams) (int64, error) {
	count, err := r.queries.CountAllTransactions(ctx, arg)
	if err != nil {
		return 0, apperror.Wrap(apperror.CodeInternal, "failed to count all transactions", err)
	}
	return count, nil
}

func (r *Repository) GetRevenueStats(ctx context.Context, dateFrom, dateTo *time.Time) (RevenueStats, error) {
	stats, err := r.queries.GetRevenueStats(ctx, dateFrom, dateTo)
	if err != nil {
		return RevenueStats{}, apperror.Wrap(apperror.CodeInternal, "failed to get revenue stats", err)
	}
	return stats, nil
}

func (r *Repository) GetRevenueSummary(ctx context.Context, dateFrom, dateTo *time.Time) ([]RevenueSummaryRow, error) {
	rows, err := r.queries.GetRevenueSummary(ctx, dateFrom, dateTo)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to get revenue summary", err)
	}
	return rows, nil
}

func (r *Repository) GetBillingSummary(ctx context.Context, filter domain.AdminTransactionFilter) (domain.BillingSummary, error) {
	summary, err := r.queries.GetBillingSummary(ctx, filter)
	if err != nil {
		return domain.BillingSummary{}, apperror.Wrap(apperror.CodeInternal, "failed to get billing summary", err)
	}
	return summary, nil
}

func (r *Repository) GetBillingDailySeries(ctx context.Context, filter domain.AdminTransactionFilter) ([]domain.BillingDailyItem, error) {
	items, err := r.queries.GetBillingDailySeries(ctx, filter)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to get billing daily series", err)
	}
	return items, nil
}

func (r *Repository) ListBillingTransactions(ctx context.Context, filter domain.AdminTransactionFilter) ([]domain.AdminTransactionItem, int64, error) {
	items, total, err := r.queries.ListBillingTransactions(ctx, filter)
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeInternal, "failed to list billing transactions", err)
	}
	return items, total, nil
}

func (r *Repository) GetBillingTransactionByID(ctx context.Context, id uuid.UUID) (*domain.AdminTransactionItem, error) {
	item, err := r.queries.GetBillingTransactionByID(ctx, id)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeTransactionNotFound, "transaction not found", err)
	}
	return item, nil
}

func (r *Repository) CancelTransaction(ctx context.Context, id uuid.UUID, adminID uuid.UUID, reason string) (*domain.AdminTransactionItem, error) {
	item, err := r.queries.CancelTransaction(ctx, id, adminID, reason)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeValidation, err.Error(), err)
	}
	return item, nil
}

func (r *Repository) SyncPayOSTransaction(ctx context.Context, id uuid.UUID, adminID uuid.UUID) (*domain.AdminTransactionItem, error) {
	item, err := r.queries.SyncPayOSTransaction(ctx, id, adminID)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to sync transaction with PayOS", err)
	}
	return item, nil
}

func (r *Repository) CreateRefundTransaction(ctx context.Context, req domain.IssueRefundRequest, adminID uuid.UUID) (*domain.AdminTransactionItem, error) {
	item, err := r.queries.CreateRefundTransaction(ctx, req, adminID)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to create refund transaction", err)
	}
	return item, nil
}
