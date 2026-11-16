package domain_test

import (
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/marketplace/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/ledger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

func codeOf(err error) apperror.Code {
	if err == nil {
		return ""
	}
	if ae, ok := err.(*apperror.AppError); ok {
		return ae.Code
	}
	return "other"
}

func TestNewPurchaseRequest(t *testing.T) {
	buyer, listing := uuid.New(), uuid.New()
	key, err := ledger.NewMarketplaceKey(listing, 20, "q7Zt3KfA9xLmP2Vd")
	if err != nil {
		t.Fatal(err)
	}
	other, _ := ledger.NewMarketplaceKey(uuid.New(), 20, "q7Zt3KfA9xLmP2Vd")
	dear, _ := ledger.NewMarketplaceKey(listing, 21, "q7Zt3KfA9xLmP2Vd")
	for name, c := range map[string]struct {
		listing uuid.UUID
		price   int32
		key     string
		want    apperror.Code
	}{
		"ok":                 {listing, 20, key, ""},
		"empty key":          {listing, 20, "", apperror.CodeCoinTransferKeyInvalid},
		"slot style key":     {listing, 20, "slot-purchase-0001", apperror.CodeCoinTransferKeyInvalid},
		"other listing":      {listing, 20, other, apperror.CodeCoinTransferKeyConflict},
		"other price in key": {listing, 20, dear, apperror.CodeCoinTransferKeyConflict},
		"price vs key":       {listing, 21, key, apperror.CodeCoinTransferKeyConflict},
		"zero price":         {listing, 0, key, apperror.CodeValidation},
		"price over cap":     {listing, domain.MaxPriceCoins + 1, key, apperror.CodeValidation},
		"nil listing":        {uuid.Nil, 20, key, apperror.CodeValidation},
	} {
		t.Run(name, func(t *testing.T) {
			req, err := domain.NewPurchaseRequest(buyer, c.listing, c.price, c.key)
			if got := codeOf(err); got != c.want {
				t.Fatalf("code %q want %q (%v)", got, c.want, err)
			}
			if err == nil && (req.KeyText != key || req.Key.Price != 20 || req.Buyer != buyer) {
				t.Errorf("request not filled: %+v", req)
			}
		})
	}
}

func TestPurchaseDescriptionBounds(t *testing.T) {
	if got := domain.PurchaseDescription("  Alex "); got != "Marketplace avatar: Alex" {
		t.Errorf("got %q", got)
	}
	if n := len([]rune(domain.PurchaseDescription(strings.Repeat("é", 400)))); n != ledger.MaxDescriptionRunes {
		t.Errorf("not truncated to %d: %d", ledger.MaxDescriptionRunes, n)
	}
}

func TestCheckStoredTransfer(t *testing.T) {
	key := ledger.MarketplaceKey{ListingID: uuid.New(), Price: 20, Nonce: "q7Zt3KfA9xLmP2Vd"}
	good := domain.StoredTransfer{To: uuid.NewString(), Amount: 20, Currency: "COIN", Status: "success"}
	if err := domain.CheckStoredTransfer(good, key); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*domain.StoredTransfer){
		"slot row":     func(r *domain.StoredTransfer) { r.To = "system" },
		"other amount": func(r *domain.StoredTransfer) { r.Amount = 5 },
		"other cur":    func(r *domain.StoredTransfer) { r.Currency = "VND" },
		"pending":      func(r *domain.StoredTransfer) { r.Status = "pending" },
		"nil seller":   func(r *domain.StoredTransfer) { r.To = uuid.Nil.String() },
	} {
		row := good
		mutate(&row)
		if got := codeOf(domain.CheckStoredTransfer(row, key)); got != apperror.CodeCoinTransferKeyConflict {
			t.Errorf("%s: %q", name, got)
		}
	}
}

func TestDecidePurchaseOrder(t *testing.T) {
	buyer, seller := uuid.New(), uuid.New()
	base := domain.PurchaseFacts{ListingStatus: domain.StatusActive, AssetStatus: "ready", Seller: seller, Buyer: buyer, ListingPrice: 20, ConfirmedPrice: 20}
	if err := domain.DecidePurchase(base); err != nil {
		t.Fatal(err)
	}
	with := func(f func(*domain.PurchaseFacts)) domain.PurchaseFacts { c := base; f(&c); return c }
	for name, c := range map[string]struct {
		facts domain.PurchaseFacts
		want  apperror.Code
	}{
		"draft":          {with(func(f *domain.PurchaseFacts) { f.ListingStatus = domain.StatusDraft }), apperror.CodeMarketplaceListingNotFound},
		"pending":        {with(func(f *domain.PurchaseFacts) { f.ListingStatus = domain.StatusPending }), apperror.CodeMarketplaceListingNotFound},
		"rejected":       {with(func(f *domain.PurchaseFacts) { f.ListingStatus = domain.StatusRejected }), apperror.CodeMarketplaceListingNotFound},
		"delisted":       {with(func(f *domain.PurchaseFacts) { f.ListingStatus = domain.StatusDelisted }), apperror.CodeMarketplaceListingUnavailable},
		"suspended":      {with(func(f *domain.PurchaseFacts) { f.ListingStatus = domain.StatusSuspended }), apperror.CodeMarketplaceListingUnavailable},
		"blocked":        {with(func(f *domain.PurchaseFacts) { f.AssetStatus = "blocked" }), apperror.CodeMarketplaceAssetNotReady},
		"own listing":    {with(func(f *domain.PurchaseFacts) { f.Seller = buyer; f.AlreadyOwned = true }), apperror.CodeMarketplaceOwnListing},
		"price changed":  {with(func(f *domain.PurchaseFacts) { f.ConfirmedPrice = 19; f.AlreadyOwned = true }), apperror.CodeMarketplacePriceChanged},
		"already owned":  {with(func(f *domain.PurchaseFacts) { f.AlreadyOwned = true }), apperror.CodeMarketplaceAlreadyOwned},
		"unavailable>nr": {with(func(f *domain.PurchaseFacts) { f.ListingStatus = domain.StatusSuspended; f.AssetStatus = "blocked" }), apperror.CodeMarketplaceListingUnavailable},
	} {
		if got := codeOf(domain.DecidePurchase(c.facts)); got != c.want {
			t.Errorf("%s: got %q want %q", name, got, c.want)
		}
	}
}
