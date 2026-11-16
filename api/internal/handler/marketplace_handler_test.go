package handler

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/marketplace/domain"
	"github.com/google/uuid"
)

func keysOf(t *testing.T, v any) []string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sampleListing(status domain.Status) domain.Listing {
	note, desc := "internal reviewer note", "d"
	now := time.Now()
	admin := uuid.New()
	return domain.Listing{ID: uuid.New(), SellerID: uuid.New(), SellerName: "Seller", AssetID: uuid.New(), AssetStatus: "ready",
		Payload: domain.Payload{Title: "t", Description: &desc, DefaultPersonality: "p", RightsNote: "secret rights", PriceCoins: 5},
		Status:  status, ReviewedBy: &admin, ReviewedAt: &now, ReviewNote: &note, CreatedAt: now, UpdatedAt: now}
}

// The catalog DTO is an allow-list: a new field must be added here on purpose.
func TestCatalogViewExposesOnlyBuyerFields(t *testing.T) {
	got := strings.Join(keysOf(t, catalogItemView(domain.CatalogItem{ID: uuid.New(), AssetID: uuid.New()})), ",")
	want := "already_owned,created_at,default_personality,description,id,is_own_listing,preview_url,price_coins,seller,title"
	if got != want {
		t.Fatalf("catalog keys\n got %s\nwant %s", got, want)
	}
	raw, _ := json.Marshal(catalogItemView(domain.CatalogItem{AssetID: uuid.New()}))
	for _, banned := range []string{"rights_note", "review_note", "reviewed", "email", "model", ".vrm", "seller_id"} {
		if strings.Contains(string(raw), banned) {
			t.Errorf("catalog JSON must not contain %q: %s", banned, raw)
		}
	}
	if k := domain.PreviewKey(uuid.Nil); !strings.HasPrefix(k, "previews/") || !strings.HasSuffix(k, ".webp") {
		t.Errorf("preview key convention: %s", k)
	}
	id := uuid.New()
	if u := catalogItemView(domain.CatalogItem{ID: id}).PreviewURL; u != "/marketplace/listings/"+id.String()+"/preview" {
		t.Errorf("preview url: %s", u)
	}
}

func TestSellerViewHidesReviewNoteUnlessRejectedOrSuspended(t *testing.T) {
	for status, visible := range map[domain.Status]bool{domain.StatusRejected: true, domain.StatusSuspended: true,
		domain.StatusDraft: false, domain.StatusPending: false, domain.StatusActive: false, domain.StatusDelisted: false} {
		v := sellerListingView(sampleListing(status))
		if (v.ReviewNote != nil) != visible {
			t.Errorf("%s: review_note visible=%v want %v", status, v.ReviewNote != nil, visible)
		}
	}
	raw, _ := json.Marshal(sellerListingView(sampleListing(domain.StatusActive)))
	for _, banned := range []string{"email", "model_path", "reviewed_by", ".vrm"} {
		if strings.Contains(string(raw), banned) {
			t.Errorf("seller JSON must not contain %q", banned)
		}
	}
}

func TestAdminViewCarriesReviewPayloadWithoutEmail(t *testing.T) {
	raw, _ := json.Marshal(adminListingView(sampleListing(domain.StatusPending)))
	for _, want := range []string{"rights_note", "asset_ready", "updated_at", "display_name"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("admin JSON must contain %q", want)
		}
	}
	for _, banned := range []string{"email", "model_path", ".vrm"} {
		if strings.Contains(string(raw), banned) {
			t.Errorf("admin JSON must not contain %q", banned)
		}
	}
}
