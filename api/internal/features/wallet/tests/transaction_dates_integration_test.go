//go:build integration

package tests

import (
	"context"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/wallet/domain"
	"github.com/cog-forge/rolecue/api/internal/features/wallet/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

// October 2026 in Vietnam time: [2026-10-01T00:00+07:00, 2026-11-01T00:00+07:00).
var (
	octFrom = time.Date(2026, 9, 30, 17, 0, 0, 0, time.UTC)
	octTo   = time.Date(2026, 10, 31, 17, 0, 0, 0, time.UTC)
)

func ts(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

type ledgerRow struct {
	from, to, currency, description, status string
	orderCode                               *string
	amount                                  int32
	createdAt                               time.Time
}

func insertLedgerRow(ctx context.Context, t *testing.T, tx pgx.Tx, row ledgerRow) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO public.transactions ("from", "to", amount, currency, description, status, payos_order_code, created_at)
		VALUES ($1, $2, $3, $4, $5, $6::transaction_status, $7, $8)
		RETURNING id`,
		row.from, row.to, row.amount, row.currency, row.description, row.status, row.orderCode, row.createdAt,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

func strPtr(s string) *string { return &s }

// seedOctober writes rows around the October window edges and returns the ID of the row created exactly
// at the start of the window.
func seedOctober(ctx context.Context, t *testing.T, tx pgx.Tx) (aliceID uuid.UUID, atStartID uuid.UUID) {
	t.Helper()
	_, err := tx.Exec(ctx, `DELETE FROM public.transactions`)
	require.NoError(t, err)

	aliceID, bobID := uuid.New(), uuid.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO public.users (id, name, email, role, email_verified)
		VALUES ($1, 'Alice Window', 'alice.window@example.com', 'candidate', true),
		       ($2, 'Bob Window', 'bob.window@example.com', 'recruiter', true)`, aliceID, bobID)
	require.NoError(t, err)

	starter := "Deposit: Window Starter (50000 VND)"
	pro := "Deposit: Window Pro (100000 VND)"
	alice, bob := aliceID.String(), bobID.String()

	// In the window: one paid top-up at the exact start, a failed and a pending top-up, a spend and a refund.
	atStartID = insertLedgerRow(ctx, t, tx, ledgerRow{to: alice, amount: 100, currency: "VND", description: starter,
		status: "success", orderCode: strPtr("700001"), createdAt: octFrom})
	insertLedgerRow(ctx, t, tx, ledgerRow{to: alice, amount: 200, currency: "VND", description: pro,
		status: "failed", orderCode: strPtr("700002"), createdAt: octFrom.Add(1 * time.Hour)})
	insertLedgerRow(ctx, t, tx, ledgerRow{to: bob, amount: 200, currency: "VND", description: pro,
		status: "pending", orderCode: strPtr("700003"), createdAt: octFrom.Add(2 * time.Hour)})
	insertLedgerRow(ctx, t, tx, ledgerRow{from: alice, to: "system", amount: 30, currency: "COIN",
		description: "Practice interview charge", status: "success", createdAt: octFrom.Add(3 * time.Hour)})
	insertLedgerRow(ctx, t, tx, ledgerRow{from: "system", to: bob, amount: 15, currency: "COIN",
		description: "Interview capacity refund", status: "success", createdAt: octFrom.Add(4 * time.Hour)})

	// Outside: a paid top-up exactly at the (exclusive) end, one a second before the start, and an old spend.
	insertLedgerRow(ctx, t, tx, ledgerRow{to: bob, amount: 200, currency: "VND", description: pro,
		status: "success", orderCode: strPtr("700004"), createdAt: octTo})
	insertLedgerRow(ctx, t, tx, ledgerRow{to: alice, amount: 200, currency: "VND", description: pro,
		status: "success", orderCode: strPtr("700005"), createdAt: octFrom.Add(-time.Second)})
	insertLedgerRow(ctx, t, tx, ledgerRow{from: alice, to: "system", amount: 99, currency: "COIN",
		description: "Old charge", status: "success", createdAt: octFrom.Add(-48 * time.Hour)})

	return aliceID, atStartID
}

func TestAdminBillingQueries_DateWindowIsHalfOpenAndSharedByEveryQuery(t *testing.T) {
	ctx, tx := openIntegrationTx(t)
	_, atStartID := seedOctober(ctx, t, tx)
	q := repository.New(tx)
	from, to := ts(octFrom), ts(octTo)

	rows, err := q.ListAdminTransactions(ctx, repository.ListAdminTransactionsParams{
		DateFrom: from, DateTo: to, SortBy: "created_at", SortOrder: "desc", PageLimit: 20,
	})
	require.NoError(t, err)
	require.Len(t, rows, 5, "start is inclusive, end is exclusive")
	require.Equal(t, int64(5), rows[0].TotalCount)
	require.True(t, rows[0].CreatedAt.Time.Equal(octFrom.Add(4*time.Hour)), "newest first")
	require.Equal(t, atStartID, rows[4].ID, "the row at date_from is included")

	count, err := q.CountAdminTransactions(ctx, repository.CountAdminTransactionsParams{DateFrom: from, DateTo: to})
	require.NoError(t, err)
	require.Equal(t, int64(5), count)

	summary, err := q.GetAdminBillingSummary(ctx, repository.GetAdminBillingSummaryParams{DateFrom: from, DateTo: to})
	require.NoError(t, err)
	require.Equal(t, int64(50000), summary.TotalRevenue, "only the paid PayOS top-up inside the window is revenue")
	require.Equal(t, int64(1), summary.SuccessfulDeposits)
	require.Equal(t, int64(3), summary.TotalDeposits)
	require.Equal(t, int64(3), summary.SuccessCount)
	require.Equal(t, int64(1), summary.PendingCount)
	require.Equal(t, int64(1), summary.FailedCount)
	require.Equal(t, int64(5), summary.TotalTransactions)
	require.Equal(t, int64(100), summary.CoinsSold)
	require.Equal(t, int64(30), summary.CoinsSpent)
	require.Equal(t, int64(15), summary.CoinsRefunded)

	packages, err := q.GetAdminPackageRevenueBreakdown(ctx, repository.GetAdminPackageRevenueBreakdownParams{DateFrom: from, DateTo: to})
	require.NoError(t, err)
	require.Equal(t, []repository.GetAdminPackageRevenueBreakdownRow{
		{PackageName: "Window Starter", RevenueVnd: 50000, DepositCount: 1, CoinsSold: 100},
	}, packages)

	features, err := q.GetAdminFeatureCoinBreakdown(ctx, repository.GetAdminFeatureCoinBreakdownParams{DateFrom: from, DateTo: to})
	require.NoError(t, err)
	require.Equal(t, []repository.GetAdminFeatureCoinBreakdownRow{
		{FeatureName: "Practice interview charge", CoinsSpent: 30, SpendCount: 1},
	}, features)

	detail, err := q.GetAdminTransactionByID(ctx, atStartID)
	require.NoError(t, err)
	require.True(t, detail.CreatedAt.Time.Equal(octFrom))
}

func TestAdminBillingQueries_OpenEndedWindowsAndCreatedAtSort(t *testing.T) {
	ctx, tx := openIntegrationTx(t)
	_, atStartID := seedOctober(ctx, t, tx)
	q := repository.New(tx)

	fromOnly, err := q.GetAdminBillingSummary(ctx, repository.GetAdminBillingSummaryParams{DateFrom: ts(octFrom)})
	require.NoError(t, err)
	require.Equal(t, int64(150000), fromOnly.TotalRevenue, "without date_to the top-up at the October end counts")

	toOnly, err := q.GetAdminBillingSummary(ctx, repository.GetAdminBillingSummaryParams{DateTo: ts(octTo)})
	require.NoError(t, err)
	require.Equal(t, int64(150000), toOnly.TotalRevenue, "without date_from the top-up before October counts")
	require.Equal(t, int64(129), toOnly.CoinsSpent)

	all, err := q.GetAdminBillingSummary(ctx, repository.GetAdminBillingSummaryParams{})
	require.NoError(t, err)
	require.Equal(t, int64(8), all.TotalTransactions)

	asc, err := q.ListAdminTransactions(ctx, repository.ListAdminTransactionsParams{
		DateFrom: ts(octFrom), DateTo: ts(octTo), SortBy: "created_at", SortOrder: "asc", PageLimit: 20,
	})
	require.NoError(t, err)
	require.Equal(t, atStartID, asc[0].ID)

	// Every other sort still breaks ties newest first.
	byStatus, err := q.ListAdminTransactions(ctx, repository.ListAdminTransactionsParams{
		Status: strPtr("success"), SortBy: "status", SortOrder: "asc", PageLimit: 20,
	})
	require.NoError(t, err)
	for i := 1; i < len(byStatus); i++ {
		require.False(t, byStatus[i].CreatedAt.Time.After(byStatus[i-1].CreatedAt.Time), "tie-break is created_at DESC")
	}
}

func TestRepository_AdminItemsCarryCreatedAtInUTC(t *testing.T) {
	ctx, tx := openIntegrationTx(t)
	_, atStartID := seedOctober(ctx, t, tx)
	repo := repository.NewRepositoryWithDB(tx)
	from, to := octFrom, octTo

	page, err := repo.ListAdminTransactions(ctx, domain.AdminTransactionFilter{
		Page: 1, PageSize: 20, DateFrom: &from, DateTo: &to, SortBy: "created_at", SortOrder: "asc",
	})
	require.NoError(t, err)
	require.Equal(t, int64(5), page.Total)
	require.Equal(t, octFrom, page.Items[0].CreatedAt, "UTC, same instant")

	item, err := repo.GetAdminTransactionByID(ctx, atStartID)
	require.NoError(t, err)
	require.Equal(t, octFrom, item.CreatedAt)

	summary, err := repo.GetBillingSummary(ctx, domain.AdminTransactionFilter{DateFrom: &from, DateTo: &to})
	require.NoError(t, err)
	require.Equal(t, int64(50000), summary.TotalRevenue)
	require.Len(t, summary.PackageBreakdown, 1)
	require.Len(t, summary.FeatureBreakdown, 1)
}

func TestListTransactionsByUserID_NewestFirstWithCreatedAt(t *testing.T) {
	ctx, tx := openIntegrationTx(t)
	aliceID, atStartID := seedOctober(ctx, t, tx)
	repo := repository.NewRepositoryWithDB(tx)

	items, total, err := repo.ListTransactionsByUserID(ctx, aliceID, 20, 0)
	require.NoError(t, err)
	require.Equal(t, int64(5), total)
	require.Len(t, items, 5)
	for i := 1; i < len(items); i++ {
		require.True(t, items[i].CreatedAt.Before(items[i-1].CreatedAt), "strictly newest first")
	}
	require.Equal(t, octFrom.Add(3*time.Hour), items[0].CreatedAt, "the spend is Alice's newest row")
	require.Equal(t, octFrom.Add(-48*time.Hour), items[4].CreatedAt)

	var found bool
	for _, item := range items {
		if item.ID == atStartID {
			found = true
			require.Equal(t, octFrom, item.CreatedAt)
		}
	}
	require.True(t, found)
}

func TestInsertTransaction_GetsCreatedAtFromTheDefault(t *testing.T) {
	ctx, tx := openIntegrationTx(t)
	q := repository.New(tx)
	before := time.Now().Add(-time.Minute)

	row, err := q.InsertTransaction(ctx, repository.InsertTransactionParams{
		TxFrom: "", TxTo: uuid.NewString(), Amount: 10, Currency: "VND",
		Status: repository.TransactionStatusPending, PayosOrderCode: strPtr("700099"),
	})
	require.NoError(t, err)
	require.True(t, row.CreatedAt.Valid)
	require.True(t, row.CreatedAt.Time.After(before))
}
