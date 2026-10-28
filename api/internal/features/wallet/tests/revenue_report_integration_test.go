//go:build integration

package tests

import (
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/wallet/repository"
	"github.com/stretchr/testify/require"
)

// seedOctober (transaction_dates_integration_test.go) gives Alice (candidate) a paid top-up on 1 Oct
// and one before October, and Bob (recruiter) a paid top-up exactly at the end of October.
func TestRevenuePeriod_TopupsRolesAndNewPayers(t *testing.T) {
	ctx, tx := openIntegrationTx(t)
	aliceID, _ := seedOctober(ctx, t, tx)
	_, err := tx.Exec(ctx, `DELETE FROM public.wallets`)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `INSERT INTO public.wallets (user_id, balance) VALUES ($1, 70)`, aliceID)
	require.NoError(t, err)
	repo := repository.NewRepositoryWithDB(tx)

	october, err := repo.GetRevenuePeriod(ctx, octFrom, octTo)
	require.NoError(t, err)
	require.Equal(t, int64(50000), october.Summary.TotalRevenue)
	require.Len(t, october.Topups, 1)
	require.Equal(t, "candidate", october.Topups[0].UserRole)
	require.Equal(t, "Window Starter", october.Topups[0].PackageName)
	require.Equal(t, int64(50000), october.Topups[0].PaidVND)
	require.Equal(t, octFrom, october.Topups[0].CreatedAt)
	require.Equal(t, int64(0), october.NewPayers, "Alice paid before October, so she returns")

	november, err := repo.GetRevenuePeriod(ctx, octTo, octTo.AddDate(0, 1, 0))
	require.NoError(t, err)
	require.Len(t, november.Topups, 1)
	require.Equal(t, "recruiter", november.Topups[0].UserRole)
	require.Equal(t, int64(1), november.NewPayers, "Bob's first paid top-up is at the start of November")

	outstanding, revenue, coinsSold, err := repo.GetCoinTotals(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(70), outstanding)
	require.Equal(t, int64(250000), revenue)
	require.Equal(t, int64(500), coinsSold)
}
