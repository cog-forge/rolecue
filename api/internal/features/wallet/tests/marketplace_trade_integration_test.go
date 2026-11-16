//go:build integration

package tests

import (
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/wallet/repository"
	"github.com/cog-forge/rolecue/api/internal/pkg/ledger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// KAN-372: a user-to-user COIN trade (Marketplace) in the wallet history and the admin finance figures. The
// ledger row is written by the real MarketplaceTransfer; the listing exists only to prove its later price and
// title changes never reach the history.

func TestMarketplaceTradeInHistoryAndAdminFinance(t *testing.T) {
	ctx, tx := openIntegrationTx(t)
	_, err := tx.Exec(ctx, `DELETE FROM public.transactions`)
	require.NoError(t, err)

	buyer, seller, stranger := uuid.New(), uuid.New(), uuid.New()
	for i, id := range []uuid.UUID{buyer, seller, stranger} {
		_, err = tx.Exec(ctx, `INSERT INTO public.users (id, name, email, role, email_verified) VALUES ($1, $2, $3, 'candidate', true)`,
			id, "Trade user "+id.String()[:8], "trade-"+id.String()+"@example.test")
		require.NoError(t, err, i)
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.wallets (user_id, balance) VALUES ($1, 100), ($2, 5)`, buyer, seller)
	require.NoError(t, err)
	var asset, listing uuid.UUID
	require.NoError(t, tx.QueryRow(ctx, `INSERT INTO public.avatar_assets (created_by, model_path) VALUES ($1, $2) RETURNING id`,
		seller, "trade-test/"+uuid.NewString()+".vrm").Scan(&asset))
	require.NoError(t, tx.QueryRow(ctx, `INSERT INTO public.avatar_listings (seller_id, asset_id, title, default_personality, price_coins, status, rights_note)
		VALUES ($1, $2, 'Refund Bot', 'calm', 30, 'active', 'mine') RETURNING id`, seller, asset).Scan(&listing))

	// A VND top-up is the only revenue.
	q := repository.New(tx)
	order, topup := "910001", "Deposit: Trade Pack (50000 VND)"
	_, err = q.InsertTransaction(ctx, repository.InsertTransactionParams{TxTo: buyer.String(), Amount: 100, Currency: "VND",
		Description: &topup, Status: repository.TransactionStatusSuccess, PayosOrderCode: &order})
	require.NoError(t, err)

	// The title deliberately contains "refund": a description must never decide what a trade is.
	key, err := ledger.NewMarketplaceKey(listing, 30, "nonce-0123456789abcdef")
	require.NoError(t, err)
	receipt, err := ledger.MarketplaceTransfer(ctx, tx, ledger.MarketplaceTransferInput{
		Key: key, ListingID: listing, Buyer: buyer, Seller: seller, Amount: 30, Description: "Purchased avatar: Refund Bot"})
	require.NoError(t, err)
	require.False(t, receipt.Replayed)

	// The listing changes after the sale: the history keeps the stored row.
	_, err = tx.Exec(ctx, `UPDATE public.avatar_listings SET price_coins = 999, title = 'Renamed' WHERE id = $1`, listing)
	require.NoError(t, err)

	t.Run("buyer and seller see one row each way, a stranger none", func(t *testing.T) {
		for who, want := range map[uuid.UUID]int64{buyer: 2, seller: 1, stranger: 0} { // buyer also has the top-up
			rows, total, err := repository.NewRepositoryWithDB(tx).ListTransactionsByUserID(ctx, who, 20, 0)
			require.NoError(t, err)
			require.Equal(t, want, total)
			require.Len(t, rows, int(want))
		}
		rows, _, err := repository.NewRepositoryWithDB(tx).ListTransactionsByUserID(ctx, seller, 20, 0)
		require.NoError(t, err)
		row := rows[0]
		require.NotNil(t, row.From)
		require.NotNil(t, row.To)
		require.Equal(t, buyer, *row.From)
		require.Equal(t, seller, *row.To)
		require.Equal(t, int32(30), row.Amount, "the stored amount, not the listing's current price")
		require.Equal(t, "COIN", row.Currency)
		require.Equal(t, "success", string(row.Status))
		require.Equal(t, "Purchased avatar: Refund Bot", row.Description, "the stored description, not the listing's current title")
		require.Nil(t, row.PayOSOrderCode)
	})

	t.Run("admin revenue is the VND top-up only and the trade is no refund", func(t *testing.T) {
		summary, err := q.GetAdminBillingSummary(ctx, repository.GetAdminBillingSummaryParams{})
		require.NoError(t, err)
		require.Equal(t, int64(50000), summary.TotalRevenue)
		require.Equal(t, int64(1), summary.SuccessfulDeposits)
		require.Equal(t, int64(1), summary.TotalDeposits)
		require.Equal(t, int64(1), summary.UniquePayers)
		require.Equal(t, int64(100), summary.CoinsSold)
		require.Equal(t, int64(0), summary.CoinsRefunded, "a trade whose title says refund is not a refund")
		require.Equal(t, int64(30), summary.CoinsSpent)
		require.Equal(t, int64(2), summary.TotalTransactions)

		refunds := "refund"
		page, err := q.ListAdminTransactions(ctx, repository.ListAdminTransactionsParams{TxType: &refunds, SortBy: "created_at", SortOrder: "desc", PageLimit: 20})
		require.NoError(t, err)
		require.Empty(t, page, "the trade must not be listed as a refund")

		spends := "spend"
		page, err = q.ListAdminTransactions(ctx, repository.ListAdminTransactionsParams{TxType: &spends, SortBy: "created_at", SortOrder: "desc", PageLimit: 20})
		require.NoError(t, err)
		require.Len(t, page, 1)
		require.Equal(t, buyer.String(), page[0].TxFrom)
		require.Equal(t, seller.String(), page[0].TxTo)
		require.Equal(t, int64(0), page[0].PaidVnd, "a COIN trade carries no VND")
		require.Equal(t, int64(-30), page[0].CoinAmount)
	})
}
