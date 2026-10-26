package repository

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"

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
	row, err := q.InsertTransaction(ctx, InsertTransactionParams{
		TxFrom:         formatUUIDPtr(tx.From),
		TxTo:           formatUUIDPtr(tx.To),
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

func (r *Repository) ListAdminTransactions(ctx context.Context, filter domain.AdminTransactionFilter) (domain.AdminTransactionPage, error) {
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	sortBy := strings.ToLower(strings.TrimSpace(filter.SortBy))
	if sortBy == "" {
		sortBy = "id"
	}
	sortOrder := strings.ToLower(strings.TrimSpace(filter.SortOrder))
	if sortOrder != "asc" {
		sortOrder = "desc"
	}

	searchPtr := escapeILIKE(filter.Search)
	statusPtr := optionalString(filter.Status)
	typePtr := optionalString(filter.Type)
	pkgPtr := escapeILIKE(filter.PackageName)

	rows, err := r.queries.ListAdminTransactions(ctx, ListAdminTransactionsParams{
		Search:      searchPtr,
		Status:      statusPtr,
		TxType:      typePtr,
		PackageName: pkgPtr,
		AmountMin:   filter.AmountMin,
		AmountMax:   filter.AmountMax,
		SortBy:      sortBy,
		SortOrder:   sortOrder,
		PageOffset:  int32(offset),
		PageLimit:   int32(pageSize),
	})
	if err != nil {
		return domain.AdminTransactionPage{}, apperror.Wrap(apperror.CodeInternal, "failed to list admin transactions", err)
	}

	// When rows are returned, COUNT(*) OVER() provides the exact filtered total in a single query.
	// When rows is empty and offset == 0 (page 1), no rows match the filter so total is 0.
	// Only when rows is empty and offset > 0 (an out-of-range page beyond the last item) do we run CountAdminTransactions.
	var total int64
	if len(rows) > 0 {
		total = rows[0].TotalCount
	} else if offset > 0 {
		total, err = r.queries.CountAdminTransactions(ctx, CountAdminTransactionsParams{
			Search:      searchPtr,
			Status:      statusPtr,
			TxType:      typePtr,
			PackageName: pkgPtr,
			AmountMin:   filter.AmountMin,
			AmountMax:   filter.AmountMax,
		})
		if err != nil {
			return domain.AdminTransactionPage{}, apperror.Wrap(apperror.CodeInternal, "failed to count admin transactions", err)
		}
	}

	items := make([]domain.AdminTransactionItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapAdminTransactionRow(
			row.ID,
			row.Amount,
			row.TxFrom,
			row.TxTo,
			row.Description,
			row.Currency,
			row.Status,
			row.PayosOrderCode,
			row.TxType,
			row.UserID,
			row.UserName,
			row.UserEmail,
			row.PackageOrFeature,
			row.PaidVnd,
			row.CoinAmount,
		))
	}

	return domain.AdminTransactionPage{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (r *Repository) GetAdminTransactionByID(ctx context.Context, id uuid.UUID) (domain.AdminTransactionItem, error) {
	row, err := r.queries.GetAdminTransactionByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.AdminTransactionItem{}, apperror.New(apperror.CodeTransactionNotFound, "transaction not found")
		}
		return domain.AdminTransactionItem{}, apperror.Wrap(apperror.CodeInternal, "failed to get admin transaction", err)
	}
	return mapAdminTransactionRow(
		row.ID,
		row.Amount,
		row.TxFrom,
		row.TxTo,
		row.Description,
		row.Currency,
		row.Status,
		row.PayosOrderCode,
		row.TxType,
		row.UserID,
		row.UserName,
		row.UserEmail,
		row.PackageOrFeature,
		row.PaidVnd,
		row.CoinAmount,
	), nil
}

func (r *Repository) GetBillingSummary(ctx context.Context, filter domain.AdminTransactionFilter) (domain.BillingSummary, error) {
	searchPtr := escapeILIKE(filter.Search)
	statusPtr := optionalString(filter.Status)
	typePtr := optionalString(filter.Type)
	pkgPtr := escapeILIKE(filter.PackageName)

	row, err := r.queries.GetAdminBillingSummary(ctx, GetAdminBillingSummaryParams{
		Search:      searchPtr,
		Status:      statusPtr,
		TxType:      typePtr,
		PackageName: pkgPtr,
		AmountMin:   filter.AmountMin,
		AmountMax:   filter.AmountMax,
	})
	if err != nil {
		return domain.BillingSummary{}, apperror.Wrap(apperror.CodeInternal, "failed to get admin billing summary", err)
	}

	var avgDeposit int64
	if row.SuccessfulDeposits > 0 {
		avgDeposit = row.TotalRevenue / row.SuccessfulDeposits
	}

	var successRate float64
	if row.TotalTransactions > 0 {
		successRate = math.Round((float64(row.SuccessCount)/float64(row.TotalTransactions))*1000) / 10
	}

	pkgBreakdown := make([]domain.PackageRevenueItem, 0)
	if (filter.Status == "" || filter.Status == string(domain.StatusSuccess)) &&
		(filter.Type == "" || filter.Type == "topup") {
		pkgRows, err := r.queries.GetAdminPackageRevenueBreakdown(ctx, GetAdminPackageRevenueBreakdownParams{
			Search:      searchPtr,
			PackageName: pkgPtr,
			AmountMin:   filter.AmountMin,
			AmountMax:   filter.AmountMax,
		})
		if err != nil {
			return domain.BillingSummary{}, apperror.Wrap(apperror.CodeInternal, "failed to get package revenue breakdown", err)
		}
		for _, pr := range pkgRows {
			pkgBreakdown = append(pkgBreakdown, domain.PackageRevenueItem{
				PackageName:  pr.PackageName,
				RevenueVND:   pr.RevenueVnd,
				DepositCount: pr.DepositCount,
				CoinsSold:    pr.CoinsSold,
			})
		}
	}

	featureBreakdown := make([]domain.FeatureCoinItem, 0)
	if (filter.Status == "" || filter.Status == string(domain.StatusSuccess)) &&
		(filter.Type == "" || filter.Type == "spend") {
		featRows, err := r.queries.GetAdminFeatureCoinBreakdown(ctx, GetAdminFeatureCoinBreakdownParams{
			Search:      searchPtr,
			PackageName: pkgPtr,
			AmountMin:   filter.AmountMin,
			AmountMax:   filter.AmountMax,
		})
		if err != nil {
			return domain.BillingSummary{}, apperror.Wrap(apperror.CodeInternal, "failed to get feature coin breakdown", err)
		}
		for _, fr := range featRows {
			featureBreakdown = append(featureBreakdown, domain.FeatureCoinItem{
				FeatureName: fr.FeatureName,
				CoinsSpent:  fr.CoinsSpent,
				SpendCount:  fr.SpendCount,
			})
		}
	}

	return domain.BillingSummary{
		TotalRevenue:       row.TotalRevenue,
		SuccessfulDeposits: row.SuccessfulDeposits,
		SuccessCount:       row.SuccessCount,
		PendingCount:       row.PendingCount,
		FailedCount:        row.FailedCount,
		CancelledCount:     row.CancelledCount,
		TotalTransactions:  row.TotalTransactions,
		AverageDeposit:     avgDeposit,
		UniquePayers:       row.UniquePayers,
		CoinsSold:          row.CoinsSold,
		CoinsSpent:         row.CoinsSpent,
		CoinsRefunded:      row.CoinsRefunded,
		CoinsNet:           row.CoinsSold - row.CoinsSpent + row.CoinsRefunded,
		SuccessRate:        successRate,
		PackageBreakdown:   pkgBreakdown,
		FeatureBreakdown:   featureBreakdown,
	}, nil
}

var ilikeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// escapeILIKE trims whitespace and escapes PostgreSQL LIKE/ILIKE wildcard characters (%, _, \)
// so user-provided search and package strings match literally.
func escapeILIKE(s string) *string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil
	}
	escaped := ilikeEscaper.Replace(trimmed)
	return &escaped
}

func optionalString(s string) *string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func mapAdminTransactionRow(
	id uuid.UUID,
	amount int32,
	txFrom string,
	txTo string,
	description string,
	currency string,
	status string,
	payosOrderCode string,
	txType string,
	userID string,
	userName string,
	userEmail string,
	packageOrFeature string,
	paidVND int64,
	coinAmount int64,
) domain.AdminTransactionItem {
	return domain.AdminTransactionItem{
		ID:               id,
		From:             txFrom,
		To:               txTo,
		UserID:           userID,
		UserName:         userName,
		UserEmail:        userEmail,
		Type:             txType,
		PackageOrFeature: packageOrFeature,
		PaidVND:          paidVND,
		CoinAmount:       coinAmount,
		Amount:           amount,
		Currency:         currency,
		Description:      description,
		Status:           domain.TransactionStatus(status),
		PayOSOrderCode:   parseInt64Ptr(&payosOrderCode),
	}
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
