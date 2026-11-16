//go:build integration

package tests

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
)

// asset adds an asset created by the user and returns its id; the fixture cleanup removes it with the accounts.
func (f *fixture) asset(creator uuid.UUID, key string) uuid.UUID {
	var id uuid.UUID
	if err := f.pool.QueryRow(f.ctx, `INSERT INTO public.avatar_assets (created_by, model_path) VALUES ($1, $2) RETURNING id`, creator, key).Scan(&id); err != nil {
		f.t.Fatal(err)
	}
	f.t.Cleanup(func() {
		_, _ = f.pool.Exec(f.ctx, `DELETE FROM public.avatar_listings WHERE asset_id = $1`, id)
		_, _ = f.pool.Exec(f.ctx, `DELETE FROM public.avatar_assets WHERE id = $1`, id)
	})
	return id
}

// avatarOn gives the user an avatar (and an inventory when missing) that points at the asset.
func (f *fixture) avatarOn(user, asset uuid.UUID) {
	var inv uuid.UUID
	if err := f.pool.QueryRow(f.ctx, `INSERT INTO public.inventories (user_id, slot) VALUES ($1, 3)
		ON CONFLICT (user_id) DO UPDATE SET slot = public.inventories.slot RETURNING id`, user).Scan(&inv); err != nil {
		f.t.Fatal(err)
	}
	f.exec(`INSERT INTO public.avatars (inventory_id, name, asset_id, personality) VALUES ($1, 'Avatar', $2, 'calm')`, inv, asset)
}

// TestEraseSellerKeepsTheSharedAssetOfABuyer erases a seller whose model another account also uses: the seller's
// private model and listing go, the shared object and asset stay for the buyer and the creator is anonymised.
func TestEraseSellerKeepsTheSharedAssetOfABuyer(t *testing.T) {
	f := newFixture(t)
	seller, buyer := f.user("candidate"), f.user("candidate")
	sharedKey, privateKey, listedOnlyKey := "shared-"+uuid.NewString(), "private-"+uuid.NewString(), "listed-"+uuid.NewString()
	shared, private, listedOnly := f.asset(seller.ID, sharedKey), f.asset(seller.ID, privateKey), f.asset(seller.ID, listedOnlyKey)
	f.avatarOn(seller.ID, shared)
	f.avatarOn(seller.ID, private)
	f.avatarOn(buyer.ID, shared)
	for _, asset := range []uuid.UUID{shared, listedOnly} {
		f.exec(`INSERT INTO public.avatar_listings (seller_id, asset_id, title, default_personality, price_coins, rights_note)
			VALUES ($1, $2, 'Listed', 'calm', 5, 'mine')`, seller.ID, asset)
	}

	if err := f.svc.Erase(f.ctx, seller, "DELETE"); err != nil {
		t.Fatal(err)
	}

	// Objects: the seller's private model and the model only a listing kept alive go; the shared one stays.
	want := sorted([]string{privateKey, listedOnlyKey})
	if got := sorted(f.store.deleted); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("deleted keys = %v, want %v", got, want)
	}
	if f.count(`SELECT count(*) FROM public.avatar_assets WHERE id = $1`, shared) != 1 {
		t.Fatal("the shared asset was deleted")
	}
	if f.count(`SELECT count(*) FROM public.avatar_assets WHERE id = $1 AND created_by IS NULL`, shared) != 1 {
		t.Fatal("the creator of the shared asset was not anonymised")
	}
	if f.count(`SELECT count(*) FROM public.avatar_assets WHERE id = ANY($1::uuid[])`, []uuid.UUID{private, listedOnly}) != 0 {
		t.Fatal("assets nobody references were kept")
	}
	if f.count(`SELECT count(*) FROM public.avatar_listings WHERE seller_id = $1`, seller.ID) != 0 {
		t.Fatal("the seller's listings were kept")
	}
	if f.count(`SELECT count(*) FROM public.avatars av JOIN public.inventories inv ON inv.id = av.inventory_id WHERE inv.user_id = $1 AND av.asset_id = $2`, buyer.ID, shared) != 1 {
		t.Fatal("the buyer's avatar was touched")
	}
}

// TestEraseBuyerNeverDeletesTheSellersModel erases the account that only holds a copy.
func TestEraseBuyerNeverDeletesTheSellersModel(t *testing.T) {
	f := newFixture(t)
	seller, buyer := f.user("candidate"), f.user("candidate")
	key := "shared-" + uuid.NewString()
	asset := f.asset(seller.ID, key)
	f.avatarOn(seller.ID, asset)
	f.avatarOn(buyer.ID, asset)

	if err := f.svc.Erase(f.ctx, buyer, "DELETE"); err != nil {
		t.Fatal(err)
	}
	if len(f.store.deleted) != 0 {
		t.Fatalf("the buyer's erasure deleted objects: %v", f.store.deleted)
	}
	if f.count(`SELECT count(*) FROM public.avatar_assets WHERE id = $1 AND created_by = $2`, asset, seller.ID) != 1 {
		t.Fatal("the seller's asset changed")
	}
	if f.count(`SELECT count(*) FROM public.avatars av JOIN public.inventories inv ON inv.id = av.inventory_id WHERE inv.user_id = $1`, seller.ID) != 1 {
		t.Fatal("the seller's avatar was touched")
	}
}
