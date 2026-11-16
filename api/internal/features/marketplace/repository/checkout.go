package repository

import (
	"context"
	"errors"

	invdomain "github.com/cog-forge/rolecue/api/internal/features/inventory/domain"
	"github.com/cog-forge/rolecue/api/internal/features/marketplace/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/ledger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/jackc/pgx/v5"
)

// Checkout runs ONE attempt of a purchase in one database transaction. The caller reruns the whole call on
// ledger.IsRetryable errors with the same key. Lock order (api/docs/marketplace-checkout-api.md): buyer user
// (shared), buyer inventory, listing (shared), asset (shared), then both wallets inside the ledger primitive.
//
//  1. A row stored under the key is a replay: it is answered from that row alone, before any listing check, so a
//     purchase that succeeded once stays answerable after a suspend, a delist or the buyer's own delete. A replay
//     writes nothing and never restores a deleted avatar.
//  2. Otherwise the listing, asset, ownership and price are re-read under the locks and decided by
//     domain.DecidePurchase; then the avatar row is granted (INVENTORY_FULL when no slot is free), the coins move
//     and the balance is read. Every refusal returns before commit, so nothing is written.
//
// No storage or network IO happens inside the transaction.
func (r *Repository) Checkout(ctx context.Context, req domain.PurchaseRequest) (domain.PurchaseResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.PurchaseResult{}, persistenceError(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	q := New(tx)

	if _, err := q.LockMarketplaceUser(ctx, req.Buyer); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.PurchaseResult{}, apperror.New(apperror.CodeForbidden, "account is not permitted to use the Marketplace")
		}
		return domain.PurchaseResult{}, persistenceError(err)
	}
	if err := q.EnsureBuyerInventory(ctx, EnsureBuyerInventoryParams{UserID: req.Buyer, InitialCapacity: invdomain.InitialCapacity}); err != nil {
		return domain.PurchaseResult{}, persistenceError(err)
	}
	if _, err := q.LockBuyerInventory(ctx, req.Buyer); err != nil {
		return domain.PurchaseResult{}, persistenceError(err)
	}

	stored, err := q.GetPurchaseTransferByKey(ctx, GetPurchaseTransferByKeyParams{Buyer: req.Buyer.String(), IdempotencyKey: req.KeyText})
	switch {
	case err == nil:
		return r.replay(ctx, tx, q, req, stored)
	case !errors.Is(err, pgx.ErrNoRows):
		return domain.PurchaseResult{}, persistenceError(err)
	}

	listing, err := q.LockListingForPurchase(ctx, req.ListingID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PurchaseResult{}, apperror.New(apperror.CodeMarketplaceListingNotFound, "listing not found")
	}
	if err != nil {
		return domain.PurchaseResult{}, persistenceError(err)
	}
	assetStatus, err := q.LockAssetForPurchase(ctx, listing.AssetID)
	if err != nil {
		return domain.PurchaseResult{}, persistenceError(err)
	}
	owned, err := q.BuyerOwnsAsset(ctx, BuyerOwnsAssetParams{UserID: req.Buyer, AssetID: listing.AssetID})
	if err != nil {
		return domain.PurchaseResult{}, persistenceError(err)
	}
	if err := domain.DecidePurchase(domain.PurchaseFacts{
		ListingStatus: domain.Status(listing.Status), AssetStatus: assetStatus, Seller: listing.SellerID, Buyer: req.Buyer,
		ListingPrice: listing.PriceCoins, ConfirmedPrice: req.ConfirmedPrice, AlreadyOwned: owned,
	}); err != nil {
		return domain.PurchaseResult{}, err
	}

	avatar, err := q.GrantPurchasedAvatar(ctx, GrantPurchasedAvatarParams{
		Name: listing.Title, AssetID: listing.AssetID, Personality: listing.DefaultPersonality, UserID: req.Buyer,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PurchaseResult{}, apperror.New(apperror.CodeInventoryFull, "avatar inventory is full")
	}
	if err != nil {
		return domain.PurchaseResult{}, persistenceError(err)
	}
	receipt, err := ledger.MarketplaceTransfer(ctx, tx, ledger.MarketplaceTransferInput{
		Key: req.KeyText, ListingID: req.ListingID, Buyer: req.Buyer, Seller: listing.SellerID, Amount: listing.PriceCoins,
		Description: domain.PurchaseDescription(listing.Title),
	})
	if err != nil {
		return domain.PurchaseResult{}, err
	}
	if receipt.Replayed {
		// Unreachable under the inventory lock and the lookup above; if it ever happens the avatar insert must not commit.
		return domain.PurchaseResult{}, apperror.New(apperror.CodeInternal, "purchase was already recorded during checkout")
	}
	balance, err := q.GetBuyerBalance(ctx, req.Buyer)
	if err != nil {
		return domain.PurchaseResult{}, persistenceError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.PurchaseResult{}, persistenceError(err)
	}
	return domain.PurchaseResult{
		Avatar:  &domain.PurchasedAvatar{ID: avatar.ID, Name: avatar.Name, Personality: avatar.Personality, CreatedAt: avatar.CreatedAt.Time, UpdatedAt: avatar.UpdatedAt.Time},
		Receipt: domain.Receipt{TransactionID: receipt.TransactionID, Amount: receipt.Amount, Currency: domain.CoinCurrency, CreatedAt: receipt.CreatedAt},
		Balance: balance,
	}, nil
}

// replay answers from the stored row. The current avatar of the asset is returned when the buyer still owns one.
// The asset id comes from the listing; when the listing is gone (the seller erased their account) the avatar is nil.
func (r *Repository) replay(ctx context.Context, tx pgx.Tx, q *Queries, req domain.PurchaseRequest, stored GetPurchaseTransferByKeyRow) (domain.PurchaseResult, error) {
	row := domain.StoredTransfer{ID: stored.ID, To: stored.To, Amount: stored.Amount, Currency: stored.Currency, Status: stored.Status, CreatedAt: stored.CreatedAt.Time}
	if err := domain.CheckStoredTransfer(row, req.Key); err != nil {
		return domain.PurchaseResult{}, err
	}
	result := domain.PurchaseResult{
		Receipt:  domain.Receipt{TransactionID: stored.ID, Amount: stored.Amount, Currency: stored.Currency, CreatedAt: stored.CreatedAt.Time},
		Replayed: true,
	}
	if assetID, err := q.GetListingAsset(ctx, req.ListingID); err == nil {
		avatar, err := q.GetBuyerAvatarOfAsset(ctx, GetBuyerAvatarOfAssetParams{UserID: req.Buyer, AssetID: assetID})
		switch {
		case err == nil:
			result.Avatar = &domain.PurchasedAvatar{ID: avatar.ID, Name: avatar.Name, Personality: avatar.Personality, CreatedAt: avatar.CreatedAt.Time, UpdatedAt: avatar.UpdatedAt.Time}
		case !errors.Is(err, pgx.ErrNoRows):
			return domain.PurchaseResult{}, persistenceError(err)
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return domain.PurchaseResult{}, persistenceError(err)
	}
	balance, err := q.GetBuyerBalance(ctx, req.Buyer)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return domain.PurchaseResult{}, persistenceError(err)
	}
	result.Balance = balance
	if err := tx.Commit(ctx); err != nil {
		return domain.PurchaseResult{}, persistenceError(err)
	}
	return result, nil
}
