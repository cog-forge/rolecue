//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/features/inventory/repository"
	"github.com/cog-forge/rolecue/api/internal/features/inventory/service"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// signingStore records the keys it signs and deletes; nothing here talks to a real bucket.
type signingStore struct {
	mu      sync.Mutex
	signed  []string
	deleted []string
}

func (s *signingStore) PresignGet(_ context.Context, key string, _ time.Duration) (storage.SignedRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.signed = append(s.signed, key)
	return storage.SignedRequest{URL: "https://storage.test/signed", Method: "GET", ExpiresAt: time.Now().Add(time.Minute)}, nil
}

func (s *signingStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleted = append(s.deleted, key)
	return nil
}

func newKey() string { return "avatars/" + uuid.NewString() + ".vrm" }

// assetRow is the avatar_assets row behind a key (the newest when several share it).
type assetRow struct {
	ID        uuid.UUID
	CreatedBy *uuid.UUID
	Status    string
}

func (s *seed) assetByKey(key string) (assetRow, bool) {
	s.t.Helper()
	var row assetRow
	rows, err := s.pool.Query(s.ctx, `SELECT id, created_by, status FROM public.avatar_assets WHERE model_path = $1 ORDER BY created_at DESC`, key)
	if err != nil {
		s.t.Fatal(err)
	}
	defer rows.Close()
	if !rows.Next() {
		return row, false
	}
	if err := rows.Scan(&row.ID, &row.CreatedBy, &row.Status); err != nil {
		s.t.Fatal(err)
	}
	return row, true
}

func (s *seed) countAssets(key string) int {
	s.t.Helper()
	var n int
	if err := s.pool.QueryRow(s.ctx, `SELECT count(*) FROM public.avatar_assets WHERE model_path = $1`, key).Scan(&n); err != nil {
		s.t.Fatal(err)
	}
	return n
}

// registerKey records a key whose assets cleanupAvatars removes at the end of the test.
func (s *seed) registerKey(key string) { s.keys = append(s.keys, key) }

// cleanupAvatars removes the avatars of the test users' inventories, then the listings and assets stored under the
// registered keys. It is registered right after newSeed, so it runs before the users are deleted.
func (s *seed) cleanupAvatars() {
	s.t.Cleanup(func() {
		ctx := context.Background()
		_, _ = s.pool.Exec(ctx, `DELETE FROM public.avatars WHERE inventory_id IN (SELECT id FROM public.inventories WHERE user_id = ANY($1::text[]::uuid[]))`, s.users)
		_, _ = s.pool.Exec(ctx, `DELETE FROM public.avatar_listings WHERE asset_id IN (SELECT id FROM public.avatar_assets WHERE model_path = ANY($1::text[]))`, s.keys)
		_, _ = s.pool.Exec(ctx, `DELETE FROM public.avatar_assets WHERE model_path = ANY($1::text[])`, s.keys)
	})
}

// grant adds an avatar row to the user's inventory that points at an existing asset, the way a purchase will:
// only asset_id is written, never model_path.
func (s *seed) grant(owner authdomain.User, asset uuid.UUID) uuid.UUID {
	s.t.Helper()
	var inventory uuid.UUID
	if err := s.pool.QueryRow(s.ctx, `INSERT INTO public.inventories (user_id, slot) VALUES ($1, 3)
		ON CONFLICT (user_id) DO UPDATE SET slot = public.inventories.slot RETURNING id`, owner.ID).Scan(&inventory); err != nil {
		s.t.Fatal(err)
	}
	var avatar uuid.UUID
	if err := s.pool.QueryRow(s.ctx, `INSERT INTO public.avatars (inventory_id, name, asset_id, personality) VALUES ($1, 'Granted', $2, 'calm') RETURNING id`, inventory, asset).Scan(&avatar); err != nil {
		s.t.Fatal(err)
	}
	return avatar
}

func TestCreateWritesTheAssetAndTheAvatarTogether(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	s.cleanupAvatars()
	owner := s.candidate(nil)
	key := newKey()
	s.registerKey(key)

	avatar, err := repository.NewRepository(pool).Create(ctx, owner.ID, "Alex", "calm", key)
	if err != nil {
		t.Fatal(err)
	}
	asset, ok := s.assetByKey(key)
	if !ok || asset.CreatedBy == nil || *asset.CreatedBy != owner.ID || asset.Status != "ready" {
		t.Fatalf("asset = %+v, found = %v, want one ready asset created by the owner", asset, ok)
	}
	var assetID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT asset_id FROM public.avatars WHERE id = $1`, avatar.ID).Scan(&assetID); err != nil {
		t.Fatal(err)
	}
	if assetID != asset.ID {
		t.Fatalf("avatar points at %s, want asset %s", assetID, asset.ID)
	}
}

func TestCreateOnAFullInventoryLeavesNoAsset(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	s.cleanupAvatars()
	owner := s.candidate(nil)
	repo := repository.NewRepository(pool)
	for range 3 {
		key := newKey()
		s.registerKey(key)
		if _, err := repo.Create(ctx, owner.ID, "Fill", "calm", key); err != nil {
			t.Fatal(err)
		}
	}
	key := newKey()
	s.registerKey(key)
	_, err := repo.Create(ctx, owner.ID, "Overflow", "calm", key)
	wantCode(t, err, apperror.CodeInventoryFull)
	if n := s.countAssets(key); n != 0 {
		t.Fatalf("a refused creation left %d assets", n)
	}
}

func TestConcurrentCreatesNeverPassTheSlots(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	s.cleanupAvatars()
	owner := s.candidate(nil)
	repo := repository.NewRepository(pool)
	const attempts = 8
	keys := make([]string, attempts)
	for i := range keys {
		keys[i] = newKey()
		s.registerKey(keys[i])
	}
	errs := make([]error, attempts)
	var wg sync.WaitGroup
	for i := range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = repo.Create(ctx, owner.ID, "Race", "calm", keys[i])
		}()
	}
	wg.Wait()
	created, full := 0, 0
	for i, err := range errs {
		switch {
		case err == nil:
			created++
			if s.countAssets(keys[i]) != 1 {
				t.Fatalf("created avatar %d has no asset", i)
			}
		case apperror.IsCode(err, apperror.CodeInventoryFull):
			full++
			if s.countAssets(keys[i]) != 0 {
				t.Fatalf("refused creation %d left an asset", i)
			}
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if created != 3 || full != attempts-3 {
		t.Fatalf("created %d, refused %d, want 3 and %d", created, full, attempts-3)
	}
}

func TestModelAccessGoesThroughTheAssetAndOwnership(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	s.cleanupAvatars()
	seller, buyer, stranger := s.candidate(nil), s.candidate(nil), s.candidate(nil)
	key := newKey()
	s.registerKey(key)
	created, err := repository.NewRepository(pool).Create(ctx, seller.ID, "Alex", "calm", key)
	if err != nil {
		t.Fatal(err)
	}
	asset, _ := s.assetByKey(key)
	// The buyer's row shares the asset; it names no creator and no listing.
	bought := s.grant(buyer, asset.ID)
	store := &signingStore{}
	svc := service.NewService(repository.NewRepository(pool), store)

	if _, err := svc.Get(ctx, seller, created.ID); err != nil {
		t.Fatalf("seller reads its own avatar: %v", err)
	}
	detail, err := svc.Get(ctx, buyer, bought)
	if err != nil {
		t.Fatalf("a granted avatar must read like a created one: %v", err)
	}
	if len(store.signed) != 2 || store.signed[0] != key || store.signed[1] != key {
		t.Fatalf("signed keys = %v, want the asset key twice", store.signed)
	}
	raw, _ := json.Marshal(detail)
	if strings.Contains(string(raw), key) || strings.Contains(string(raw), "model_path") {
		t.Fatalf("the response leaks the storage key: %s", raw)
	}
	// Nobody reads an avatar through another inventory: not the buyer's row by the seller, not the seller's by the buyer.
	for _, c := range []struct {
		who    authdomain.User
		avatar uuid.UUID
	}{{seller, bought}, {buyer, created.ID}, {stranger, bought}, {stranger, created.ID}} {
		_, err := svc.Get(ctx, c.who, c.avatar)
		wantCode(t, err, apperror.CodeAvatarNotFound)
	}
	if len(store.signed) != 2 {
		t.Fatalf("a foreign read signed a key: %v", store.signed)
	}
}

func TestDeletingAnAvatarKeepsAnAssetThatIsStillUsed(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	s.cleanupAvatars()
	seller, buyer := s.candidate(nil), s.candidate(nil)
	key := newKey()
	s.registerKey(key)
	created, err := repository.NewRepository(pool).Create(ctx, seller.ID, "Alex", "calm", key)
	if err != nil {
		t.Fatal(err)
	}
	asset, _ := s.assetByKey(key)
	bought := s.grant(buyer, asset.ID)
	store := &signingStore{}
	svc := service.NewService(repository.NewRepository(pool), store)

	if err := svc.Delete(ctx, seller, created.ID); err != nil {
		t.Fatal(err)
	}
	if len(store.deleted) != 0 || s.countAssets(key) != 1 {
		t.Fatalf("the seller's delete removed a shared model: deleted %v, assets %d", store.deleted, s.countAssets(key))
	}
	if _, err := svc.Get(ctx, buyer, bought); err != nil {
		t.Fatalf("the buyer lost the model: %v", err)
	}
	if err := svc.Delete(ctx, buyer, bought); err != nil {
		t.Fatal(err)
	}
	if len(store.deleted) != 1 || store.deleted[0] != key || s.countAssets(key) != 0 {
		t.Fatalf("the last avatar must remove the asset and the object: deleted %v, assets %d", store.deleted, s.countAssets(key))
	}
}

func TestDeletingTheLastAvatarKeepsAListedAsset(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	s.cleanupAvatars()
	seller := s.candidate(nil)
	key := newKey()
	s.registerKey(key)
	created, err := repository.NewRepository(pool).Create(ctx, seller.ID, "Alex", "calm", key)
	if err != nil {
		t.Fatal(err)
	}
	asset, _ := s.assetByKey(key)
	s.exec(`INSERT INTO public.avatar_listings (seller_id, asset_id, title, default_personality, price_coins, rights_note)
		VALUES ($1, $2, 'Alex', 'calm', 5, 'mine')`, seller.ID, asset.ID)
	store := &signingStore{}

	if err := service.NewService(repository.NewRepository(pool), store).Delete(ctx, seller, created.ID); err != nil {
		t.Fatal(err)
	}
	if len(store.deleted) != 0 || s.countAssets(key) != 1 {
		t.Fatalf("a listed model was removed: deleted %v, assets %d", store.deleted, s.countAssets(key))
	}
}

func TestDeletingAnAvatarKeepsAnObjectAnotherAssetStillStores(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	s.cleanupAvatars()
	owner := s.candidate(nil)
	key := newKey()
	s.registerKey(key)
	// Legacy rows may share one key across assets: the seeded demo avatars do.
	first := s.avatar(owner, key)
	s.avatar(owner, key)
	if s.countAssets(key) != 2 {
		t.Fatalf("legacy inserts made %d assets, want 2", s.countAssets(key))
	}
	store := &signingStore{}
	if err := service.NewService(repository.NewRepository(pool), store).Delete(ctx, owner, first); err != nil {
		t.Fatal(err)
	}
	if len(store.deleted) != 0 || s.countAssets(key) != 1 {
		t.Fatalf("the shared key was deleted: deleted %v, assets %d", store.deleted, s.countAssets(key))
	}
}
