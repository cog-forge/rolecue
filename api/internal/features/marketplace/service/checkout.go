package service

import (
	"context"
	"strconv"

	"github.com/cog-forge/rolecue/api/internal/features/marketplace/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/ledger"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/google/uuid"
)

// Purchase buys one listing for the buyer (KAN-370, AM-11). The request is validated without storage first (price
// range, key shape, key bound to this listing and price). The repository then runs the whole purchase in one
// transaction; a deadlock or serialization failure rolls that transaction back and the whole call is rerun with
// the same key, at most domain.MaxCheckoutAttempts attempts, after which the error (a 500) is returned. Replaying
// is safe: a stored success is answered from its row with no second debit and no second grant.
func (s *Service) Purchase(ctx context.Context, buyer, listingID uuid.UUID, confirmedPrice int32, key string) (domain.PurchaseResult, error) {
	req, err := domain.NewPurchaseRequest(buyer, listingID, confirmedPrice, key)
	if err != nil {
		return domain.PurchaseResult{}, err
	}
	var result domain.PurchaseResult
	for attempt := 1; ; attempt++ {
		result, err = s.repo.Checkout(ctx, req)
		if err == nil {
			break
		}
		if !ledger.IsRetryable(err) || attempt >= domain.MaxCheckoutAttempts || ctx.Err() != nil {
			return domain.PurchaseResult{}, err
		}
		logger.FromContext(ctx).Warn("marketplace checkout retried",
			logger.Op("marketplace.checkout"), logger.Int("attempt", attempt), logger.Cause(err))
	}
	logger.FromContext(ctx).Info("marketplace purchase",
		logger.Op("marketplace.checkout"),
		logger.String("listing_id", listingID.String()),
		logger.String("transaction_id", result.Receipt.TransactionID.String()),
		logger.Int("amount", int(result.Receipt.Amount)),
		logger.String("replayed", strconv.FormatBool(result.Replayed)),
	)
	return result, nil
}
