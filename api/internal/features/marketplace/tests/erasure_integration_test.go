//go:build integration

package tests

import (
	"context"
	"sync"
	"testing"
	"time"

	accountrepo "github.com/cog-forge/rolecue/api/internal/features/account/repository"
	accountsvc "github.com/cog-forge/rolecue/api/internal/features/account/service"
	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// KAN-373: account erasure against shared assets. The checkout service is the real one; only the object store
// is a recording fake.

type objectStore struct {
	mu      sync.Mutex
	deleted []string
}

func (s *objectStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleted = append(s.deleted, key)
	return nil
}

func (s *objectStore) PresignGetWithOverrides(context.Context, string, time.Duration, storage.GetOverrides) (storage.SignedRequest, error) {
	return storage.SignedRequest{}, nil
}

func (s *objectStore) deletedKeys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.deleted...)
}

func (w *world) eraser() (*accountsvc.Service, *objectStore) {
	store := &objectStore{}
	return accountsvc.NewService(accountrepo.NewRepository(w.pool), store), store
}

func (w *world) erase(svc *accountsvc.Service, user uuid.UUID) error {
	return svc.Erase(w.ctx, authdomain.User{ID: user, Role: "candidate"}, "DELETE")
}

func (w *world) assetKey(asset uuid.UUID) string {
	w.t.Helper()
	var key string
	if err := w.pool.QueryRow(w.ctx, `SELECT model_path FROM public.avatar_assets WHERE id = $1`, asset).Scan(&key); err != nil {
		w.t.Fatal(err)
	}
	return key
}

func (w *world) assetExists(asset uuid.UUID) bool {
	return w.count(`SELECT count(*) FROM public.avatar_assets WHERE id = $1`, asset) == 1
}

// A seller with several buyers is erased: the listing goes, the buyers keep avatar, asset and object; the
// object is removed only when the last holder is gone.
func TestEraseSellerWithSeveralBuyersKeepsTheSharedAsset(t *testing.T) {
	w := newWorld(t)
	svc, store := w.eraser()
	s := w.shop(20)
	key := w.assetKey(s.listing.AssetID)
	buyers := []uuid.UUID{w.buyer(50), w.buyer(50), w.buyer(50)}
	for _, b := range buyers {
		if _, err := w.buy(b, s, 20, newKey(t, s.listing.ID, 20)); err != nil {
			t.Fatal(err)
		}
	}

	if err := w.erase(svc, s.seller); err != nil {
		t.Fatal(err)
	}
	if len(store.deletedKeys()) != 0 {
		t.Fatalf("the shared object was deleted: %v", store.deletedKeys())
	}
	if !w.assetExists(s.listing.AssetID) || w.count(`SELECT count(*) FROM public.avatar_assets WHERE id = $1 AND created_by IS NULL`, s.listing.AssetID) != 1 {
		t.Fatal("the asset must stay with an anonymised creator")
	}
	if w.count(`SELECT count(*) FROM public.avatar_listings WHERE id = $1`, s.listing.ID) != 0 {
		t.Fatal("the seller's listing was kept")
	}
	for _, b := range buyers {
		if w.owned(b, s.listing.AssetID) != 1 {
			t.Fatal("a buyer lost the avatar")
		}
	}
	// The buyers leave one by one: the object goes with the last holder, once.
	for i, b := range buyers {
		if err := w.erase(svc, b); err != nil {
			t.Fatal(err)
		}
		if got := store.deletedKeys(); i < 2 && len(got) != 0 {
			t.Fatalf("buyer %d: object deleted while another buyer holds it: %v", i, got)
		}
	}
	if got := store.deletedKeys(); len(got) != 1 || got[0] != key {
		t.Fatalf("deleted = %v, want the shared object once", got)
	}
	if w.assetExists(s.listing.AssetID) {
		t.Fatal("the orphan asset was kept")
	}
}

// A buyer erased alone never touches the seller's model, avatar or listing.
func TestEraseBuyerAloneLeavesTheSellerIntact(t *testing.T) {
	w := newWorld(t)
	svc, store := w.eraser()
	s := w.shop(20)
	buyer := w.buyer(50)
	if _, err := w.buy(buyer, s, 20, newKey(t, s.listing.ID, 20)); err != nil {
		t.Fatal(err)
	}
	if err := w.erase(svc, buyer); err != nil {
		t.Fatal(err)
	}
	if len(store.deletedKeys()) != 0 || !w.assetExists(s.listing.AssetID) || w.owned(s.seller, s.listing.AssetID) != 1 ||
		w.count(`SELECT count(*) FROM public.avatar_listings WHERE id = $1 AND status = 'active'`, s.listing.ID) != 1 {
		t.Fatalf("the seller's side changed: deleted %v", store.deletedKeys())
	}
}

// A listing that is the only thing keeping an asset alive goes with the seller, asset and object included.
func TestEraseSellerRemovesAnAssetOnlyHisListingKept(t *testing.T) {
	w := newWorld(t)
	svc, store := w.eraser()
	s := w.shop(20)
	key := w.assetKey(s.listing.AssetID)
	w.exec(`DELETE FROM public.avatars WHERE id = $1`, s.avatar) // the seller deleted the source avatar: the listing keeps the asset
	if err := w.erase(svc, s.seller); err != nil {
		t.Fatal(err)
	}
	if got := store.deletedKeys(); len(got) != 1 || got[0] != key || w.assetExists(s.listing.AssetID) {
		t.Fatalf("deleted = %v, asset kept = %v", got, w.assetExists(s.listing.AssetID))
	}
}

// Legacy assets may share one object key: the object stays while any asset still stores it.
func TestEraseKeepsAnObjectAnotherAssetStillStores(t *testing.T) {
	w := newWorld(t)
	svc, store := w.eraser()
	a, b := w.user("candidate"), w.user("candidate")
	_, assetA := w.avatar(a)
	key := w.assetKey(assetA)
	var assetB uuid.UUID
	if err := w.pool.QueryRow(w.ctx, `INSERT INTO public.avatar_assets (created_by, model_path) VALUES ($1, $2) RETURNING id`, b, key).Scan(&assetB); err != nil {
		t.Fatal(err)
	}
	w.exec(`INSERT INTO public.avatars (inventory_id, name, asset_id, personality) VALUES ($1, 'Legacy', $2, 'calm')`, w.inventory(b), assetB)

	if err := w.erase(svc, a); err != nil {
		t.Fatal(err)
	}
	if len(store.deletedKeys()) != 0 || !w.assetExists(assetB) || w.assetExists(assetA) {
		t.Fatalf("shared key: deleted %v, A kept %v, B kept %v", store.deletedKeys(), w.assetExists(assetA), w.assetExists(assetB))
	}
	if err := w.erase(svc, b); err != nil {
		t.Fatal(err)
	}
	if got := store.deletedKeys(); len(got) != 1 || got[0] != key {
		t.Fatalf("deleted = %v, want the object once after its last asset", got)
	}
}

// Normal moderation and delisting never delete a buyer's avatar, the asset or the file; an asset in use or
// listed cannot be deleted at the database level either.
func TestDelistSuspendAndRestrictionsNeverTouchABuyersCopy(t *testing.T) {
	w := newWorld(t)
	_, store := w.eraser()
	s := w.shop(20)
	buyer := w.buyer(50)
	if _, err := w.buy(buyer, s, 20, newKey(t, s.listing.ID, 20)); err != nil {
		t.Fatal(err)
	}
	if _, err := w.svc.Suspend(w.ctx, s.admin, "admin", s.listing.ID, "review"); err != nil {
		t.Fatal(err)
	}
	if w.count(`SELECT count(*) FROM public.avatar_assets WHERE id = $1 AND status = 'ready'`, s.listing.AssetID) != 1 {
		t.Fatal("suspending a listing blocked the shared asset")
	}
	if w.owned(buyer, s.listing.AssetID) != 1 || len(store.deletedKeys()) != 0 {
		t.Fatal("suspend touched the buyer's copy")
	}
	if _, err := w.pool.Exec(w.ctx, `DELETE FROM public.avatar_assets WHERE id = $1`, s.listing.AssetID); err == nil {
		t.Fatal("an asset used by avatars and a listing was deletable")
	}
	w.exec(`DELETE FROM public.avatar_listings WHERE id = $1`, s.listing.ID)
	w.exec(`DELETE FROM public.avatars WHERE id = $1`, s.avatar)
	if _, err := w.pool.Exec(w.ctx, `DELETE FROM public.avatar_assets WHERE id = $1`, s.listing.AssetID); err == nil {
		t.Fatal("an asset still used by the buyer's avatar was deletable")
	}
	if w.owned(buyer, s.listing.AssetID) != 1 {
		t.Fatal("the buyer's avatar disappeared")
	}
}

// The erase-vs-purchase race: whichever side wins, a committed avatar never points at a deleted object and an
// object is deleted only together with its asset row.
func TestEraseRacingPurchaseNeverLosesTheBuyersObject(t *testing.T) {
	w := newWorld(t)
	svc, store := w.eraser()
	bought, gone := 0, 0
	for i := 0; i < 40; i++ {
		s := w.shop(20)
		key := w.assetKey(s.listing.AssetID)
		buyer := w.buyer(50)
		before := len(store.deletedKeys())
		var wg sync.WaitGroup
		var buyErr, eraseErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			time.Sleep(time.Duration(i%10) * 700 * time.Microsecond) // sweeps the purchase across the erasure's statements
			_, buyErr = w.buy(buyer, s, 20, newKey(t, s.listing.ID, 20))
		}()
		go func() { defer wg.Done(); eraseErr = w.erase(svc, s.seller) }()
		wg.Wait()
		if eraseErr != nil {
			t.Fatalf("iteration %d: erase failed: %v", i, eraseErr)
		}
		deleted := len(store.deletedKeys()) > before
		switch {
		case buyErr == nil:
			bought++
			if deleted || !w.assetExists(s.listing.AssetID) || w.owned(buyer, s.listing.AssetID) != 1 || w.balance(buyer) != 30 {
				t.Fatalf("iteration %d: the buyer paid but lost the model (object deleted: %v)", i, deleted)
			}
		case apperror.IsCode(buyErr, apperror.CodeMarketplaceListingUnavailable) || apperror.IsCode(buyErr, apperror.CodeMarketplaceListingNotFound):
			gone++
			if !deleted || store.deletedKeys()[before] != key || w.assetExists(s.listing.AssetID) || w.balance(buyer) != 50 || w.txRows(buyer) != 0 {
				t.Fatalf("iteration %d: refused purchase left a half state (object deleted: %v)", i, deleted)
			}
		default:
			t.Fatalf("iteration %d: %v", i, buyErr)
		}
	}
	t.Logf("purchase won %d, erasure won %d", bought, gone)
}

// Deterministic window: a purchase holds the listing and the asset (checkout's locks) and has granted the avatar
// but not committed. The erasure of the seller must wait, then see the new avatar and keep the asset and object.
func TestEraseWaitsForAPurchaseInFlightAndKeepsTheModel(t *testing.T) {
	w := newWorld(t)
	svc, store := w.eraser()
	s := w.shop(20)
	buyer := w.buyer(50)
	tx, err := w.pool.Begin(w.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(w.ctx, `SELECT id FROM public.avatar_listings WHERE id = $1 FOR SHARE`, s.listing.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(w.ctx, `SELECT id FROM public.avatar_assets WHERE id = $1 FOR SHARE`, s.listing.AssetID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(w.ctx, `INSERT INTO public.avatars (inventory_id, name, asset_id, personality) VALUES ($1, 'Bought', $2, 'calm')`,
		w.inventory(buyer), s.listing.AssetID); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- w.erase(svc, s.seller) }()
	select {
	case err := <-done:
		t.Fatalf("the erasure did not wait for the purchase in flight: %v", err)
	case <-time.After(400 * time.Millisecond):
	}
	if err := tx.Commit(w.ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if len(store.deletedKeys()) != 0 || !w.assetExists(s.listing.AssetID) || w.owned(buyer, s.listing.AssetID) != 1 {
		t.Fatalf("the buyer's model was lost: deleted %v", store.deletedKeys())
	}
}
