package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cog-forge/rolecue/api/internal/pkg/ledger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// MaxCheckoutAttempts bounds the reruns of one checkout transaction after a deadlock or serialization failure.
// Every rerun uses the same idempotency key, so it is safe.
const MaxCheckoutAttempts = 3

// CoinCurrency is the only currency of a Marketplace purchase.
const CoinCurrency = "COIN"

// PurchaseRequest is one purchase intent after validation: who buys which listing, at which confirmed price,
// under which idempotency key. Key is the parsed form of KeyText.
type PurchaseRequest struct {
	Buyer          uuid.UUID
	ListingID      uuid.UUID
	ConfirmedPrice int32
	Key            ledger.MarketplaceKey
	KeyText        string
}

// NewPurchaseRequest checks the request without touching storage: the confirmed price must be a valid
// listing price, the key must be a well formed Marketplace key (COIN_TRANSFER_KEY_INVALID), and it must name
// exactly this listing and this price (COIN_TRANSFER_KEY_CONFLICT).
func NewPurchaseRequest(buyer, listingID uuid.UUID, confirmedPrice int32, key string) (PurchaseRequest, error) {
	if buyer == uuid.Nil || listingID == uuid.Nil || confirmedPrice < MinPriceCoins || confirmedPrice > MaxPriceCoins {
		return PurchaseRequest{}, invalid("price_coins must be between 1 and 1000000")
	}
	parsed, err := ledger.ParseMarketplaceKey(key)
	if err != nil {
		return PurchaseRequest{}, err
	}
	if parsed.ListingID != listingID || parsed.Price != confirmedPrice {
		return PurchaseRequest{}, apperror.New(apperror.CodeCoinTransferKeyConflict, "the idempotency key names another listing or price than this request")
	}
	return PurchaseRequest{Buyer: buyer, ListingID: listingID, ConfirmedPrice: confirmedPrice, Key: parsed, KeyText: key}, nil
}

// PurchaseDescription is the history text of the coin transfer row: never read back, 1..255 characters.
func PurchaseDescription(title string) string {
	text := "Marketplace avatar: " + strings.TrimSpace(title)
	if utf8.RuneCountInString(text) > ledger.MaxDescriptionRunes {
		return string([]rune(text)[:ledger.MaxDescriptionRunes])
	}
	return text
}

// StoredTransfer is the buyer's ledger row found under the purchase key.
type StoredTransfer struct {
	ID        uuid.UUID
	To        string
	Amount    int32
	Currency  string
	Status    string
	CreatedAt time.Time
}

// CheckStoredTransfer decides whether a row found under the key is the success of this very purchase intent.
// Anything else (a slot row, another currency or status, another price) is COIN_TRANSFER_KEY_CONFLICT.
func CheckStoredTransfer(row StoredTransfer, key ledger.MarketplaceKey) error {
	seller, err := uuid.Parse(row.To)
	if err != nil || seller == uuid.Nil || row.Currency != CoinCurrency || row.Status != "success" || row.Amount != key.Price {
		return apperror.New(apperror.CodeCoinTransferKeyConflict, "the idempotency key is already used by another operation")
	}
	return nil
}

// PurchaseFacts are the values the checkout re-reads under its locks before it moves coins.
type PurchaseFacts struct {
	ListingStatus  Status
	AssetStatus    string
	Seller         uuid.UUID
	Buyer          uuid.UUID
	ListingPrice   int32
	ConfirmedPrice int32
	AlreadyOwned   bool
}

// DecidePurchase is the refusal order of the checkout, first match wins:
//
//  1. a draft, pending or rejected listing was never public: not found;
//  2. a delisted or suspended listing: MARKETPLACE_LISTING_UNAVAILABLE;
//  3. a model that is not ready: MARKETPLACE_ASSET_NOT_READY;
//  4. the buyer is the seller: MARKETPLACE_OWN_LISTING (before already-owned, the seller owns the original);
//  5. the confirmed price is not the listing price: MARKETPLACE_PRICE_CHANGED;
//  6. the buyer already owns an avatar of the asset: MARKETPLACE_ALREADY_OWNED.
//
// A full inventory (INVENTORY_FULL) and a low balance (INSUFFICIENT_BALANCE) follow in the storage step.
func DecidePurchase(f PurchaseFacts) error {
	switch f.ListingStatus {
	case StatusActive:
	case StatusDelisted, StatusSuspended:
		return apperror.New(apperror.CodeMarketplaceListingUnavailable, "this listing is no longer available")
	default:
		return apperror.New(apperror.CodeMarketplaceListingNotFound, "listing not found")
	}
	switch {
	case f.AssetStatus != "ready":
		return apperror.New(apperror.CodeMarketplaceAssetNotReady, "the avatar model is not available")
	case f.Seller == f.Buyer:
		return apperror.New(apperror.CodeMarketplaceOwnListing, "you cannot buy your own listing")
	case f.ListingPrice != f.ConfirmedPrice:
		return apperror.New(apperror.CodeMarketplacePriceChanged, "the price of this listing is not the price you confirmed")
	case f.AlreadyOwned:
		return apperror.New(apperror.CodeMarketplaceAlreadyOwned, "you already own this avatar")
	}
	return nil
}

// Receipt is the coin transfer of a purchase. On a replay it is the row written by the first success.
type Receipt struct {
	TransactionID uuid.UUID
	Amount        int32
	Currency      string
	CreatedAt     time.Time
}

// PurchasedAvatar is the buyer's avatar row of the purchased asset (same fields as the inventory DTO).
type PurchasedAvatar struct {
	ID          uuid.UUID
	Name        string
	Personality string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// PurchaseResult is the outcome of a checkout. Avatar is nil only on a replay when the buyer no longer owns an
// avatar of the asset: a replay never restores it. Balance is the buyer's coin balance at the time of the call.
type PurchaseResult struct {
	Avatar   *PurchasedAvatar
	Receipt  Receipt
	Balance  int32
	Replayed bool
}
