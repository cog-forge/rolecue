//go:build integration

package tests

import (
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/wallet/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

// KAN-266: the database refuses the values the application never writes. Each statement runs in a
// savepoint, so a rejected row does not abort the test transaction.
func TestWalletInvariants_ConstraintsRejectBadRows(t *testing.T) {
	ctx, tx := openIntegrationTx(t)
	userID := uuid.New()
	_, err := tx.Exec(ctx, `INSERT INTO public.users (id, name, email, role, email_verified)
		VALUES ($1, 'Invariant User', 'invariant.user@example.com', 'candidate', true)`, userID)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `INSERT INTO public.wallets (user_id, balance) VALUES ($1, 5)`, userID)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `INSERT INTO public.inventories (user_id, slot) VALUES ($1, 3)`, userID)
	require.NoError(t, err)

	cases := []struct {
		name       string
		constraint string
		sql        string
		args       []any
	}{
		{"negative balance", "wallets_balance_nonnegative",
			`UPDATE public.wallets SET balance = -1 WHERE user_id = $1`, []any{userID}},
		{"zero transaction amount", "transactions_amount_positive",
			`INSERT INTO public.transactions ("from", "to", amount, currency) VALUES ('system', $1, 0, 'COIN')`, []any{userID.String()}},
		{"negative transaction amount", "transactions_amount_positive",
			`INSERT INTO public.transactions ("from", "to", amount, currency) VALUES ('system', $1, -5, 'COIN')`, []any{userID.String()}},
		{"unknown sender", "transactions_from_party_check",
			`INSERT INTO public.transactions ("from", "to", amount, currency) VALUES ('mallory', $1, 5, 'COIN')`, []any{userID.String()}},
		{"unknown recipient", "transactions_to_party_check",
			`INSERT INTO public.transactions ("from", "to", amount, currency) VALUES ($1, 'bank', 5, 'COIN')`, []any{userID.String()}},
		{"zero package price", "coin_packages_price_positive",
			`INSERT INTO public.coin_packages (name, price, amount, currency) VALUES ('Free', 0, 10, 'VND')`, nil},
		{"negative package price", "coin_packages_price_positive",
			`INSERT INTO public.coin_packages (name, price, amount, currency) VALUES ('Refund', -1, 10, 'VND')`, nil},
		{"zero package coins", "coin_packages_amount_positive",
			`INSERT INTO public.coin_packages (name, price, amount, currency) VALUES ('Empty', 1000, 0, 'VND')`, nil},
		{"slots above the maximum", "inventories_slot_max",
			`UPDATE public.inventories SET slot = 101 WHERE user_id = $1`, []any{userID}},
		{"negative slots", "inventories_slot_nonnegative",
			`UPDATE public.inventories SET slot = -1 WHERE user_id = $1`, []any{userID}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sp, err := tx.Begin(ctx)
			require.NoError(t, err)
			_, err = sp.Exec(ctx, tc.sql, tc.args...)
			_ = sp.Rollback(ctx)
			var pgErr *pgconn.PgError
			require.ErrorAs(t, err, &pgErr)
			require.Equal(t, "23514", pgErr.Code)
			require.Equal(t, tc.constraint, pgErr.ConstraintName)
		})
	}
}

// The values the code writes today still pass: the legacy empty party, the system side and user ids.
func TestWalletInvariants_AcceptsLedgerShapes(t *testing.T) {
	ctx, tx := openIntegrationTx(t)
	userID := uuid.New().String()
	for _, row := range [][2]string{{"", userID}, {userID, ""}, {"system", userID}, {userID, "system"}} {
		_, err := tx.Exec(ctx, `INSERT INTO public.transactions ("from", "to", amount, currency) VALUES ($1, $2, 1, 'COIN')`, row[0], row[1])
		require.NoError(t, err, "from=%q to=%q", row[0], row[1])
	}
}

// The package list and the checkout read is_active: an inactive package is not offered.
func TestWalletInvariants_InactivePackageIsHidden(t *testing.T) {
	ctx, tx := openIntegrationTx(t)
	activeID, inactiveID := uuid.New(), uuid.New()
	_, err := tx.Exec(ctx, `INSERT INTO public.coin_packages (id, name, price, amount, currency, is_active)
		VALUES ($1, 'Invariant Active', 10000, 10, 'VND', true), ($2, 'Invariant Retired', 20000, 20, 'VND', false)`, activeID, inactiveID)
	require.NoError(t, err)

	repo := repository.NewRepositoryWithDB(tx)
	listed, err := repo.ListCoinPackages(ctx)
	require.NoError(t, err)
	var ids []uuid.UUID
	for _, p := range listed {
		ids = append(ids, p.ID)
	}
	require.Contains(t, ids, activeID)
	require.NotContains(t, ids, inactiveID)

	retired, err := repo.GetCoinPackageByID(ctx, inactiveID)
	require.NoError(t, err)
	require.False(t, retired.IsActive, "the checkout must see the package is inactive")
	active, err := repo.GetCoinPackageByID(ctx, activeID)
	require.NoError(t, err)
	require.True(t, active.IsActive)
}
