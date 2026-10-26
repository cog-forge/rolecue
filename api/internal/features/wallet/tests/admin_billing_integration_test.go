//go:build integration

package tests

import (
	"context"
	"os"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/wallet/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func openIntegrationTx(t *testing.T) (context.Context, pgx.Tx) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/rolecue?sslmode=disable"
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("Integration DB unavailable: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("Integration DB unreachable: %v", err)
	}
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	return ctx, tx
}

func TestAdminBillingSQLCQueries_RevenueAndFilters(t *testing.T) {
	ctx, tx := openIntegrationTx(t)

	// Isolate from any existing seed data inside the rolled-back transaction.
	_, err := tx.Exec(ctx, `DELETE FROM public.transactions`)
	require.NoError(t, err)

	aliceID := uuid.New()
	bobID := uuid.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO public.users (id, email, password, role, name, status, is_verified)
		VALUES
			($1, 'alice.billing@example.com', 'hash', 'candidate', 'Alice Billing', 'active', true),
			($2, 'bob.billing@example.com', 'hash', 'recruiter', 'Bob Billing', 'active', true)
	`, aliceID, bobID)
	require.NoError(t, err)

	starterPkgID := uuid.New()
	proPkgID := uuid.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO public.coin_packages (id, name, price, description, amount, currency)
		VALUES
			($1, 'Test Starter Pack', 50000, 'Starter', 100, 'VND'),
			($2, 'Test Pro Pack', 100000, 'Pro', 200, 'VND')
	`, starterPkgID, proPkgID)
	require.NoError(t, err)

	q := repository.New(tx)

	orderSuccess1 := "900001"
	orderSuccess2 := "900002"
	orderPending := "900003"
	orderFailed := "900004"
	orderCancelled := "900005"

	descStarter := "Deposit: Test Starter Pack (50000 VND)"
	descPro := "Deposit: Test Pro Pack (100000 VND)"
	descSpend := "Practice interview charge"
	descRefund := "Interview capacity refund"

	// 1. Successful PayOS top-up #1 (50,000 VND, 100 coins)
	tx1, err := q.InsertTransaction(ctx, repository.InsertTransactionParams{
		TxFrom:         "",
		TxTo:           aliceID.String(),
		Amount:         100,
		Currency:       "VND",
		Description:    &descStarter,
		Status:         repository.TransactionStatusSuccess,
		PayosOrderCode: &orderSuccess1,
	})
	require.NoError(t, err)

	// 2. Successful PayOS top-up #2 (100,000 VND, 200 coins)
	_, err = q.InsertTransaction(ctx, repository.InsertTransactionParams{
		TxFrom:         "",
		TxTo:           bobID.String(),
		Amount:         200,
		Currency:       "VND",
		Description:    &descPro,
		Status:         repository.TransactionStatusSuccess,
		PayosOrderCode: &orderSuccess2,
	})
	require.NoError(t, err)

	// 3. Pending PayOS top-up (must NOT be counted in revenue)
	_, err = q.InsertTransaction(ctx, repository.InsertTransactionParams{
		TxFrom:         "",
		TxTo:           aliceID.String(),
		Amount:         200,
		Currency:       "VND",
		Description:    &descPro,
		Status:         repository.TransactionStatusPending,
		PayosOrderCode: &orderPending,
	})
	require.NoError(t, err)

	// 4. Failed PayOS top-up (must NOT be counted in revenue)
	_, err = q.InsertTransaction(ctx, repository.InsertTransactionParams{
		TxFrom:         "",
		TxTo:           aliceID.String(),
		Amount:         100,
		Currency:       "VND",
		Description:    &descStarter,
		Status:         repository.TransactionStatusFailed,
		PayosOrderCode: &orderFailed,
	})
	require.NoError(t, err)

	// 5. Cancelled PayOS top-up (must NOT be counted in revenue)
	_, err = q.InsertTransaction(ctx, repository.InsertTransactionParams{
		TxFrom:         "",
		TxTo:           bobID.String(),
		Amount:         100,
		Currency:       "VND",
		Description:    &descStarter,
		Status:         repository.TransactionStatusCancelled,
		PayosOrderCode: &orderCancelled,
	})
	require.NoError(t, err)

	// 6. Internal coin spend (status='success', no payos_order_code -> must NOT affect VND revenue)
	_, err = q.InsertTransaction(ctx, repository.InsertTransactionParams{
		TxFrom:         aliceID.String(),
		TxTo:           "",
		Amount:         30,
		Currency:       "VND",
		Description:    &descSpend,
		Status:         repository.TransactionStatusSuccess,
		PayosOrderCode: nil,
	})
	require.NoError(t, err)

	// 7. Internal slot refund (status='success', from='system', no payos_order_code -> must NOT be subtracted from VND revenue)
	_, err = q.InsertTransaction(ctx, repository.InsertTransactionParams{
		TxFrom:         "system",
		TxTo:           bobID.String(),
		Amount:         15,
		Currency:       "COIN",
		Description:    &descRefund,
		Status:         repository.TransactionStatusSuccess,
		PayosOrderCode: nil,
	})
	require.NoError(t, err)

	// Verify Summary (KAN-192)
	summary, err := q.GetAdminBillingSummary(ctx, repository.GetAdminBillingSummaryParams{})
	require.NoError(t, err)
	require.Equal(t, int64(150000), summary.TotalRevenue, "revenue must only sum success PayOS top-ups")
	require.Equal(t, int64(2), summary.SuccessfulDeposits, "successful_deposits must only count success PayOS top-ups")
	require.Equal(t, int64(4), summary.SuccessCount, "2 top-ups + 1 spend + 1 refund")
	require.Equal(t, int64(1), summary.PendingCount)
	require.Equal(t, int64(1), summary.FailedCount)
	require.Equal(t, int64(1), summary.CancelledCount)
	require.Equal(t, int64(7), summary.TotalTransactions)
	require.Equal(t, int64(2), summary.UniquePayers)
	require.Equal(t, int64(300), summary.CoinsSold)
	require.Equal(t, int64(30), summary.CoinsSpent)
	require.Equal(t, int64(15), summary.CoinsRefunded)

	// Verify Package Breakdown
	pkgRows, err := q.GetAdminPackageRevenueBreakdown(ctx, repository.GetAdminPackageRevenueBreakdownParams{})
	require.NoError(t, err)
	require.Len(t, pkgRows, 2)
	require.Equal(t, "Test Pro Pack", pkgRows[0].PackageName)
	require.Equal(t, int64(100000), pkgRows[0].RevenueVnd)
	require.Equal(t, "Test Starter Pack", pkgRows[1].PackageName)
	require.Equal(t, int64(50000), pkgRows[1].RevenueVnd)

	// Verify Feature Breakdown
	featRows, err := q.GetAdminFeatureCoinBreakdown(ctx, nil)
	require.NoError(t, err)
	require.Len(t, featRows, 1)
	require.Equal(t, "Practice interview charge", featRows[0].FeatureName)
	require.Equal(t, int64(30), featRows[0].CoinsSpent)

	// Verify Filtering by User Email & Type & Pagination
	searchAlice := "alice.billing@example.com"
	topupType := "topup"
	aliceTopups, err := q.ListAdminTransactions(ctx, repository.ListAdminTransactionsParams{
		Search:     &searchAlice,
		TxType:     &topupType,
		SortBy:     "amount",
		SortOrder:  "desc",
		PageOffset: 0,
		PageLimit:  20,
	})
	require.NoError(t, err)
	require.Len(t, aliceTopups, 3)

	// Verify GetAdminTransactionByID
	detail, err := q.GetAdminTransactionByID(ctx, tx1.ID)
	require.NoError(t, err)
	require.Equal(t, "Alice Billing", detail.UserName)
	require.Equal(t, int64(50000), detail.PaidVnd)
	require.Equal(t, int64(100), detail.CoinAmount)
}
