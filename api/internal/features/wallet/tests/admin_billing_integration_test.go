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
		INSERT INTO public.users (id, name, email, role, email_verified)
		VALUES
			($1, 'Alice Billing', 'alice.billing@example.com', 'candidate', true),
			($2, 'Bob Billing', 'bob.billing@example.com', 'recruiter', true)
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
	featRows, err := q.GetAdminFeatureCoinBreakdown(ctx, repository.GetAdminFeatureCoinBreakdownParams{})
	require.NoError(t, err)
	require.Len(t, featRows, 1)
	require.Equal(t, "Practice interview charge", featRows[0].FeatureName)
	require.Equal(t, int64(30), featRows[0].CoinsSpent)

	// Verify Filtering by User Email & Type & Pagination (including window TotalCount)
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
	require.Equal(t, int64(3), aliceTopups[0].TotalCount)

	// Verify GetAdminTransactionByID
	detail, err := q.GetAdminTransactionByID(ctx, tx1.ID)
	require.NoError(t, err)
	require.Equal(t, "Alice Billing", detail.UserName)
	require.Equal(t, int64(50000), detail.PaidVnd)
	require.Equal(t, int64(100), detail.CoinAmount)
}

func TestAdminBillingSQLCQueries_MixedPackageInferenceAndWildcardEscape(t *testing.T) {
	ctx, tx := openIntegrationTx(t)

	_, err := tx.Exec(ctx, `DELETE FROM public.transactions`)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `DELETE FROM public.coin_packages`)
	require.NoError(t, err)

	userID := uuid.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO public.users (id, name, email, role, email_verified)
		VALUES ($1, 'Carol 100% User_1', 'carol_1@example.com', 'candidate', true)
	`, userID)
	require.NoError(t, err)

	// Seed packages where "Micro Trial Pack" has VND price = 1000 and coin amount = 2,
	// and "Starter Pack v2" represents a renamed/repriced package (amount = 100, price = 60,000 VND).
	// Note: there is intentionally NO package with amount = 1000, so a 1000-coin transaction
	// must NOT falsely match "Micro Trial Pack" via price = 1000.
	pkgB := uuid.New()
	pkgC := uuid.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO public.coin_packages (id, name, price, description, amount, currency)
		VALUES
			($1, 'Micro Trial Pack', 1000, '2 coins for 1000 VND', 2, 'VND'),
			($2, 'Starter Pack v2', 60000, 'Renamed and repriced starter', 100, 'VND')
	`, pkgB, pkgC)
	require.NoError(t, err)

	q := repository.New(tx)

	// Case 1: Historical row before package was renamed and repriced ("Legacy Starter Pack (40000 VND)").
	// Must preserve historical package name ("Legacy Starter Pack") and historical price (40,000 VND) from description.
	order1 := "800001"
	descRenamed := "Deposit: Legacy Starter Pack (40000 VND)"
	txRenamed, err := q.InsertTransaction(ctx, repository.InsertTransactionParams{
		TxFrom:         "",
		TxTo:           userID.String(),
		Amount:         100,
		Currency:       "VND",
		Description:    &descRenamed,
		Status:         repository.TransactionStatusSuccess,
		PayosOrderCode: &order1,
	})
	require.NoError(t, err)

	// Case 2: Row with missing "(N VND)" suffix ("Deposit: Starter Pack v2").
	// Must fall back to matching coin_packages row ("Starter Pack v2" -> 60,000 VND).
	order2 := "800002"
	descNoSuffix := "Deposit: Starter Pack v2"
	txNoSuffix, err := q.InsertTransaction(ctx, repository.InsertTransactionParams{
		TxFrom:         "",
		TxTo:           userID.String(),
		Amount:         100,
		Currency:       "VND",
		Description:    &descNoSuffix,
		Status:         repository.TransactionStatusSuccess,
		PayosOrderCode: &order2,
	})
	require.NoError(t, err)

	// Case 3: Row where coin amount (1000) equals another package's VND price (1000 for "Micro Trial Pack").
	// Must NOT match "Micro Trial Pack" by price; must preserve its own package name and 1000 coin amount.
	order3 := "800003"
	descRetired := "Deposit: Retired 1000-Coin Pack (500000 VND)"
	txCollision, err := q.InsertTransaction(ctx, repository.InsertTransactionParams{
		TxFrom:         "",
		TxTo:           userID.String(),
		Amount:         1000,
		Currency:       "VND",
		Description:    &descRetired,
		Status:         repository.TransactionStatusSuccess,
		PayosOrderCode: &order3,
	})
	require.NoError(t, err)

	gotRenamed, err := q.GetAdminTransactionByID(ctx, txRenamed.ID)
	require.NoError(t, err)
	require.Equal(t, "Legacy Starter Pack", gotRenamed.PackageOrFeature)
	require.Equal(t, int64(40000), gotRenamed.PaidVnd)
	require.Equal(t, int64(100), gotRenamed.CoinAmount)

	gotNoSuffix, err := q.GetAdminTransactionByID(ctx, txNoSuffix.ID)
	require.NoError(t, err)
	require.Equal(t, "Starter Pack v2", gotNoSuffix.PackageOrFeature)
	require.Equal(t, int64(60000), gotNoSuffix.PaidVnd)
	require.Equal(t, int64(100), gotNoSuffix.CoinAmount)

	gotCollision, err := q.GetAdminTransactionByID(ctx, txCollision.ID)
	require.NoError(t, err)
	require.Equal(t, "Retired 1000-Coin Pack", gotCollision.PackageOrFeature)
	require.Equal(t, int64(500000), gotCollision.PaidVnd)
	require.Equal(t, int64(1000), gotCollision.CoinAmount, "coin amount must not be overwritten by Micro Trial Pack's 2 coins")

	// Total revenue across the 3 deposits: 40,000 + 60,000 + 500,000 = 600,000 VND
	// Total coins sold across the 3 deposits: 100 + 100 + 1,000 = 1,200 coins
	summary, err := q.GetAdminBillingSummary(ctx, repository.GetAdminBillingSummaryParams{})
	require.NoError(t, err)
	require.Equal(t, int64(600000), summary.TotalRevenue)
	require.Equal(t, int64(3), summary.SuccessfulDeposits)
	require.Equal(t, int64(1200), summary.CoinsSold)

	// Per-package revenue and coin totals must also preserve each package's distinct VND price and coin amount
	pkgBreakdown, err := q.GetAdminPackageRevenueBreakdown(ctx, repository.GetAdminPackageRevenueBreakdownParams{})
	require.NoError(t, err)
	require.Len(t, pkgBreakdown, 3)
	require.Equal(t, repository.GetAdminPackageRevenueBreakdownRow{
		PackageName:  "Retired 1000-Coin Pack",
		RevenueVnd:   500000,
		DepositCount: 1,
		CoinsSold:    1000,
	}, pkgBreakdown[0])
	require.Equal(t, repository.GetAdminPackageRevenueBreakdownRow{
		PackageName:  "Starter Pack v2",
		RevenueVnd:   60000,
		DepositCount: 1,
		CoinsSold:    100,
	}, pkgBreakdown[1])
	require.Equal(t, repository.GetAdminPackageRevenueBreakdownRow{
		PackageName:  "Legacy Starter Pack",
		RevenueVnd:   40000,
		DepositCount: 1,
		CoinsSold:    100,
	}, pkgBreakdown[2])

	// Escaped wildcard search: literal `100\%` must match "Carol 100% User_1", whereas `999\%` must match nothing.
	escapedHit := `100\%`
	rowsHit, err := q.ListAdminTransactions(ctx, repository.ListAdminTransactionsParams{
		Search:    &escapedHit,
		SortBy:    "id",
		SortOrder: "desc",
		PageLimit: 20,
	})
	require.NoError(t, err)
	require.Len(t, rowsHit, 3)

	escapedMiss := `999\%`
	rowsMiss, err := q.ListAdminTransactions(ctx, repository.ListAdminTransactionsParams{
		Search:    &escapedMiss,
		SortBy:    "id",
		SortOrder: "desc",
		PageLimit: 20,
	})
	require.NoError(t, err)
	require.Empty(t, rowsMiss)
}
