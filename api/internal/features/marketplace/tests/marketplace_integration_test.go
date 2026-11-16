//go:build integration

// Package tests: Avatar Marketplace seller, admin and catalog integration tests against a real Postgres. The
// repository and the service run unmodified; every test seeds its own users and removes exactly its rows.
package tests

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/marketplace/domain"
	"github.com/cog-forge/rolecue/api/internal/features/marketplace/repository"
	"github.com/cog-forge/rolecue/api/internal/features/marketplace/service"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type world struct {
	t      *testing.T
	ctx    context.Context
	pool   *pgxpool.Pool
	svc    *service.Service
	signer *recordingSigner
	users  []uuid.UUID
}

func newWorld(t *testing.T) *world {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is not set (load ../.env)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("database not reachable: %v", err)
	}
	w := &world{t: t, ctx: ctx, pool: pool, signer: &recordingSigner{}}
	w.svc = service.New(repository.NewRepository(pool), w.signer)
	_ = w
	t.Cleanup(w.cleanup)
	return w
}

// recordingSigner records the keys it is asked to sign; nothing here talks to a bucket.
type recordingSigner struct {
	mu   sync.Mutex
	keys []string
}

func (r *recordingSigner) PresignGet(_ context.Context, key string, _ time.Duration) (storage.SignedRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.keys = append(r.keys, key)
	return storage.SignedRequest{URL: "https://storage.test/" + key}, nil
}

func (w *world) exec(sql string, args ...any) {
	w.t.Helper()
	if _, err := w.pool.Exec(w.ctx, sql, args...); err != nil {
		w.t.Fatalf("seed %q: %v", sql, err)
	}
}

func (w *world) cleanup() {
	bg := context.Background()
	for _, sql := range []string{
		`DELETE FROM public.avatar_listings WHERE seller_id = ANY($1)`,
		`DELETE FROM public.avatars WHERE inventory_id IN (SELECT id FROM public.inventories WHERE user_id = ANY($1))`,
		`DELETE FROM public.avatar_assets WHERE created_by = ANY($1) OR created_by IS NULL AND model_path LIKE 'mp-test/%'`,
		`DELETE FROM public.inventories WHERE user_id = ANY($1)`,
		`DELETE FROM public.transactions WHERE "from" = ANY($1::uuid[]::text[]) OR "to" = ANY($1::uuid[]::text[])`,
		`DELETE FROM public.wallets WHERE user_id = ANY($1)`,
		`DELETE FROM public.users WHERE id = ANY($1)`,
	} {
		if _, err := w.pool.Exec(bg, sql, w.users); err != nil {
			w.t.Errorf("cleanup: %v", err)
		}
	}
}

func (w *world) user(role string) uuid.UUID {
	w.t.Helper()
	id := uuid.New()
	w.exec(`INSERT INTO public.users (id, name, email, role, email_verified) VALUES ($1, $2, $3, $4, true)`,
		id, "mp "+role+" "+id.String()[:8], fmt.Sprintf("mp-%s@example.test", id), role)
	w.users = append(w.users, id)
	return id
}

func (w *world) inventory(owner uuid.UUID) uuid.UUID {
	w.t.Helper()
	var id uuid.UUID
	if err := w.pool.QueryRow(w.ctx, `INSERT INTO public.inventories (user_id, slot) VALUES ($1, 5)
		ON CONFLICT (user_id) DO UPDATE SET slot = public.inventories.slot RETURNING id`, owner).Scan(&id); err != nil {
		w.t.Fatal(err)
	}
	return id
}

// avatar creates a fresh asset created by owner, then an avatar of it in the owner's inventory.
func (w *world) avatar(owner uuid.UUID) (avatar, asset uuid.UUID) {
	w.t.Helper()
	inv := w.inventory(owner)
	if err := w.pool.QueryRow(w.ctx, `INSERT INTO public.avatar_assets (created_by, model_path) VALUES ($1, $2) RETURNING id`,
		owner, "mp-test/"+uuid.NewString()+".vrm").Scan(&asset); err != nil {
		w.t.Fatal(err)
	}
	if err := w.pool.QueryRow(w.ctx, `INSERT INTO public.avatars (inventory_id, name, asset_id, personality)
		VALUES ($1, 'Mine', $2, 'calm') RETURNING id`, inv, asset).Scan(&avatar); err != nil {
		w.t.Fatal(err)
	}
	return avatar, asset
}

// copyOf gives owner an avatar of an existing asset (what a purchase grants).
func (w *world) copyOf(owner, asset uuid.UUID) uuid.UUID {
	w.t.Helper()
	var id uuid.UUID
	if err := w.pool.QueryRow(w.ctx, `INSERT INTO public.avatars (inventory_id, name, asset_id, personality)
		VALUES ($1, 'Bought', $2, 'calm') RETURNING id`, w.inventory(owner), asset).Scan(&id); err != nil {
		w.t.Fatal(err)
	}
	return id
}

func payload(title string, price int32) domain.Payload {
	d := "A calm interviewer"
	return domain.Payload{Title: title, Description: &d, DefaultPersonality: "calm and precise", RightsNote: "I made this avatar myself.", PriceCoins: price}
}

func (w *world) draft(seller, avatar uuid.UUID, title string, price int32) domain.Listing {
	w.t.Helper()
	l, err := w.svc.Create(w.ctx, seller, avatar, payload(title, price))
	if err != nil {
		w.t.Fatalf("create: %v", err)
	}
	return l
}

func (w *world) pending(seller, avatar uuid.UUID, title string, price int32) domain.Listing {
	w.t.Helper()
	l := w.draft(seller, avatar, title, price)
	l, err := w.svc.Submit(w.ctx, seller, l.ID, l.UpdatedAt)
	if err != nil {
		w.t.Fatalf("submit: %v", err)
	}
	return l
}

func (w *world) active(seller, avatar uuid.UUID, admin uuid.UUID, title string, price int32) domain.Listing {
	w.t.Helper()
	l := w.pending(seller, avatar, title, price)
	l, err := w.svc.Approve(w.ctx, admin, "admin", l.ID, l.UpdatedAt)
	if err != nil {
		w.t.Fatalf("approve: %v", err)
	}
	return l
}

func wantCode(t *testing.T, err error, code apperror.Code) {
	t.Helper()
	if !apperror.IsCode(err, code) {
		t.Fatalf("want %s, got %v", code, err)
	}
}

func TestSellerLifecycle(t *testing.T) {
	w := newWorld(t)
	seller, admin := w.user("candidate"), w.user("admin")
	avatar, asset := w.avatar(seller)

	eligible, err := w.svc.EligibleAvatars(w.ctx, seller)
	if err != nil || len(eligible) != 1 || eligible[0].AvatarID != avatar {
		t.Fatalf("eligible: %v %v", eligible, err)
	}
	l := w.draft(seller, avatar, "Alex", 20)
	if l.Status != domain.StatusDraft || l.SellerID != seller || l.AssetID != asset {
		t.Fatalf("draft: %+v", l)
	}
	if eligible, _ = w.svc.EligibleAvatars(w.ctx, seller); len(eligible) != 0 {
		t.Fatal("a listed avatar must leave the eligible list")
	}

	edited, err := w.svc.Edit(w.ctx, seller, l.ID, l.UpdatedAt, payload("Alex v2", 25))
	if err != nil || edited.Title != "Alex v2" || edited.PriceCoins != 25 || !edited.UpdatedAt.After(l.UpdatedAt) {
		t.Fatalf("edit: %+v %v", edited, err)
	}
	_, err = w.svc.Edit(w.ctx, seller, l.ID, l.UpdatedAt, payload("stale", 1))
	wantCode(t, err, apperror.CodeMarketplaceListingStale)
	_, err = w.svc.Edit(w.ctx, seller, l.ID, edited.UpdatedAt, payload("x", 0))
	wantCode(t, err, apperror.CodeValidation)

	pending, err := w.svc.Submit(w.ctx, seller, l.ID, edited.UpdatedAt)
	if err != nil || pending.Status != domain.StatusPending {
		t.Fatalf("submit: %+v %v", pending, err)
	}
	_, err = w.svc.Submit(w.ctx, seller, l.ID, edited.UpdatedAt)
	wantCode(t, err, apperror.CodeMarketplaceInvalidTransition) // double submit
	_, err = w.svc.Edit(w.ctx, seller, l.ID, pending.UpdatedAt, payload("sneaky", 1))
	wantCode(t, err, apperror.CodeMarketplaceInvalidTransition) // pending payload is frozen
	_, err = w.svc.Delist(w.ctx, seller, l.ID, pending.UpdatedAt)
	wantCode(t, err, apperror.CodeMarketplaceInvalidTransition)

	active, err := w.svc.Approve(w.ctx, admin, "admin", l.ID, pending.UpdatedAt)
	if err != nil || active.Status != domain.StatusActive || active.ReviewedBy == nil || *active.ReviewedBy != admin || active.ReviewedAt == nil {
		t.Fatalf("approve: %+v %v", active, err)
	}
	_, err = w.svc.Edit(w.ctx, seller, l.ID, active.UpdatedAt, payload("sneaky", 1))
	wantCode(t, err, apperror.CodeMarketplaceInvalidTransition) // active payload is never silently edited
	_, err = w.svc.Submit(w.ctx, seller, l.ID, active.UpdatedAt)
	wantCode(t, err, apperror.CodeMarketplaceInvalidTransition)

	// A buyer holds a copy before the delist; the delist must not touch it, the asset or the listing row.
	buyer := w.user("candidate")
	copyID := w.copyOf(buyer, asset)
	_, err = w.svc.Delist(w.ctx, w.user("candidate"), l.ID, active.UpdatedAt)
	wantCode(t, err, apperror.CodeMarketplaceListingNotFound) // owner only
	delisted, err := w.svc.Delist(w.ctx, seller, l.ID, active.UpdatedAt)
	if err != nil || delisted.Status != domain.StatusDelisted {
		t.Fatalf("delist: %+v %v", delisted, err)
	}
	again, err := w.svc.Delist(w.ctx, seller, l.ID, active.UpdatedAt) // stale timestamp, already delisted: idempotent
	if err != nil || again.Status != domain.StatusDelisted || !again.UpdatedAt.Equal(delisted.UpdatedAt) {
		t.Fatalf("repeat delist: %+v %v", again, err)
	}
	var n int
	if err := w.pool.QueryRow(w.ctx, `SELECT count(*) FROM public.avatars WHERE id = $1 AND asset_id = $2`, copyID, asset).Scan(&n); err != nil || n != 1 {
		t.Fatalf("buyer avatar must survive a delist: %d %v", n, err)
	}
	_, err = w.svc.Submit(w.ctx, seller, l.ID, delisted.UpdatedAt)
	wantCode(t, err, apperror.CodeMarketplaceInvalidTransition) // no relist

	mine, total, err := w.svc.MyListings(w.ctx, seller, nil, 20, 0)
	if err != nil || total != 1 || len(mine) != 1 {
		t.Fatalf("my listings: %d %v", total, err)
	}
	if _, err = w.svc.MyListing(w.ctx, buyer, l.ID); err == nil {
		t.Fatal("another user must not read my listing")
	}
}

func TestEligibility(t *testing.T) {
	w := newWorld(t)
	seller, buyer, other := w.user("candidate"), w.user("recruiter"), w.user("candidate")
	avatar, asset := w.avatar(seller)

	_, err := w.svc.Create(w.ctx, other, avatar, payload("Theft", 5)) // someone else's avatar
	wantCode(t, err, apperror.CodeMarketplaceAssetNotEligible)
	_, err = w.svc.Create(w.ctx, seller, uuid.New(), payload("Ghost", 5))
	wantCode(t, err, apperror.CodeMarketplaceAssetNotEligible)
	bought := w.copyOf(buyer, asset) // a buyer's copy of the seller's asset
	_, err = w.svc.Create(w.ctx, buyer, bought, payload("Resale", 5))
	wantCode(t, err, apperror.CodeMarketplaceAssetNotEligible)
	if eligible, _ := w.svc.EligibleAvatars(w.ctx, buyer); len(eligible) != 0 {
		t.Fatal("a bought copy is never eligible")
	}

	first := w.draft(seller, avatar, "First", 5)
	_, err = w.svc.Create(w.ctx, seller, avatar, payload("Second", 5))
	wantCode(t, err, apperror.CodeMarketplaceListingExists)

	// Erasure of the creator (created_by NULL) makes the asset unsellable by anyone.
	w.exec(`UPDATE public.avatar_assets SET created_by = NULL WHERE id = $1`, asset)
	_, err = w.svc.Submit(w.ctx, seller, first.ID, first.UpdatedAt)
	wantCode(t, err, apperror.CodeMarketplaceAssetNotEligible)

	// A blocked asset cannot be listed or submitted.
	avatar2, asset2 := w.avatar(seller)
	w.exec(`UPDATE public.avatar_assets SET status = 'blocked' WHERE id = $1`, asset2)
	_, err = w.svc.Create(w.ctx, seller, avatar2, payload("Blocked", 5))
	wantCode(t, err, apperror.CodeMarketplaceAssetNotReady)

	// A locked or unverified account is refused.
	w.exec(`UPDATE public.users SET is_locked = true WHERE id = $1`, seller)
	_, err = w.svc.Create(w.ctx, seller, avatar2, payload("Locked", 5))
	wantCode(t, err, apperror.CodeForbidden)
}

func TestSellerCannotSellWhenAvatarGone(t *testing.T) {
	w := newWorld(t)
	seller := w.user("candidate")
	avatar, _ := w.avatar(seller)
	l := w.draft(seller, avatar, "Gone", 5)
	w.exec(`DELETE FROM public.avatars WHERE id = $1`, avatar)
	_, err := w.svc.Submit(w.ctx, seller, l.ID, l.UpdatedAt)
	wantCode(t, err, apperror.CodeMarketplaceAssetNotEligible)
}

func TestReviewIsByExactPayload(t *testing.T) {
	w := newWorld(t)
	seller, admin := w.user("candidate"), w.user("admin")
	avatar, _ := w.avatar(seller)
	l := w.pending(seller, avatar, "Reviewed", 10)

	_, err := w.svc.Approve(w.ctx, seller, "candidate", l.ID, l.UpdatedAt)
	wantCode(t, err, apperror.CodeForbidden)
	_, err = w.svc.Reject(w.ctx, seller, "recruiter", l.ID, l.UpdatedAt, "no")
	wantCode(t, err, apperror.CodeForbidden)
	_, err = w.svc.Suspend(w.ctx, seller, "", l.ID, "no")
	wantCode(t, err, apperror.CodeForbidden)
	_, _, err = w.svc.AdminList(w.ctx, "candidate", nil, 20, 0)
	wantCode(t, err, apperror.CodeForbidden)

	_, err = w.svc.Reject(w.ctx, admin, "admin", l.ID, l.UpdatedAt, "  ")
	wantCode(t, err, apperror.CodeValidation) // a reason is required
	_, err = w.svc.Approve(w.ctx, admin, "admin", l.ID, l.UpdatedAt.Add(-time.Second))
	wantCode(t, err, apperror.CodeMarketplaceListingStale)

	rejected, err := w.svc.Reject(w.ctx, admin, "admin", l.ID, l.UpdatedAt, "Rights note is unclear")
	if err != nil || rejected.Status != domain.StatusRejected || rejected.ReviewNote == nil || *rejected.ReviewNote != "Rights note is unclear" {
		t.Fatalf("reject: %+v %v", rejected, err)
	}
	_, err = w.svc.Approve(w.ctx, admin, "admin", l.ID, rejected.UpdatedAt)
	wantCode(t, err, apperror.CodeMarketplaceInvalidTransition)
	_, err = w.svc.Submit(w.ctx, seller, l.ID, rejected.UpdatedAt) // resubmit needs an edit first
	wantCode(t, err, apperror.CodeMarketplaceInvalidTransition)

	edited, err := w.svc.Edit(w.ctx, seller, l.ID, rejected.UpdatedAt, payload("Reviewed again", 10))
	if err != nil || edited.Status != domain.StatusRejected {
		t.Fatalf("edit rejected: %+v %v", edited, err)
	}
	resubmitted, err := w.svc.Submit(w.ctx, seller, l.ID, edited.UpdatedAt)
	if err != nil || resubmitted.Status != domain.StatusPending {
		t.Fatalf("resubmit: %+v %v", resubmitted, err)
	}
	// The Admin who loaded the first pending payload cannot approve the resubmitted one.
	_, err = w.svc.Approve(w.ctx, admin, "admin", l.ID, l.UpdatedAt)
	wantCode(t, err, apperror.CodeMarketplaceListingStale)

	w.exec(`UPDATE public.avatar_assets SET status = 'blocked' WHERE id = $1`, resubmitted.AssetID)
	_, err = w.svc.Approve(w.ctx, admin, "admin", l.ID, resubmitted.UpdatedAt)
	wantCode(t, err, apperror.CodeMarketplaceAssetNotReady)
	detail, err := w.svc.AdminGet(w.ctx, "admin", l.ID)
	if err != nil || detail.AssetStatus != "blocked" || detail.RightsNote == "" || detail.SellerName == "" {
		t.Fatalf("admin detail: %+v %v", detail, err)
	}
	w.exec(`UPDATE public.avatar_assets SET status = 'ready' WHERE id = $1`, resubmitted.AssetID)
	if _, err = w.svc.Approve(w.ctx, admin, "admin", l.ID, resubmitted.UpdatedAt); err != nil {
		t.Fatal(err)
	}
}

func TestQueueFilterAndPagination(t *testing.T) {
	w := newWorld(t)
	seller, admin := w.user("candidate"), w.user("admin")
	var ids []uuid.UUID
	for i := 0; i < 3; i++ {
		avatar, _ := w.avatar(seller)
		ids = append(ids, w.pending(seller, avatar, fmt.Sprintf("Queue %d", i), 10).ID)
	}
	pending := domain.StatusPending
	items, total, err := w.svc.AdminList(w.ctx, "admin", &pending, 2, 0)
	if err != nil || len(items) != 2 || total < 3 {
		t.Fatalf("page 1: %d/%d %v", len(items), total, err)
	}
	for _, bad := range [][2]int32{{0, 0}, {51, 0}, {20, -1}, {20, domain.MaxOffset + 1}} {
		_, _, err := w.svc.AdminList(w.ctx, "admin", nil, bad[0], bad[1])
		wantCode(t, err, apperror.CodeValidation)
	}
	if _, err := w.svc.Reject(w.ctx, admin, "admin", ids[0], mustUpdatedAt(t, w, ids[0]), "no"); err != nil {
		t.Fatal(err)
	}
	rejected := domain.StatusRejected
	items, _, err = w.svc.AdminList(w.ctx, "admin", &rejected, 50, 0)
	found := false
	for _, i := range items {
		found = found || i.ID == ids[0]
	}
	if err != nil || !found {
		t.Fatalf("rejected filter: %v %v", found, err)
	}
}

func mustUpdatedAt(t *testing.T, w *world, id uuid.UUID) time.Time {
	t.Helper()
	l, err := w.svc.AdminGet(w.ctx, "admin", id)
	if err != nil {
		t.Fatal(err)
	}
	return l.UpdatedAt
}

func TestSuspendIsIdempotentAndKeepsBuyers(t *testing.T) {
	w := newWorld(t)
	seller, admin, buyer := w.user("candidate"), w.user("admin"), w.user("candidate")
	avatar, asset := w.avatar(seller)
	l := w.active(seller, avatar, admin, "Suspend me", 10)
	copyID := w.copyOf(buyer, asset)

	_, err := w.svc.Suspend(w.ctx, admin, "admin", l.ID, "")
	wantCode(t, err, apperror.CodeValidation)
	s, err := w.svc.Suspend(w.ctx, admin, "admin", l.ID, "Copyright complaint")
	if err != nil || s.Status != domain.StatusSuspended || s.ReviewNote == nil || *s.ReviewNote != "Copyright complaint" {
		t.Fatalf("suspend: %+v %v", s, err)
	}
	again, err := w.svc.Suspend(w.ctx, admin, "admin", l.ID, "Another reason")
	if err != nil || *again.ReviewNote != "Copyright complaint" || !again.UpdatedAt.Equal(s.UpdatedAt) {
		t.Fatalf("repeat suspend must not rewrite: %+v %v", again, err)
	}
	_, err = w.svc.CatalogItem(w.ctx, buyer, l.ID)
	wantCode(t, err, apperror.CodeMarketplaceListingNotFound) // hidden from the catalog
	_, err = w.svc.Delist(w.ctx, seller, l.ID, s.UpdatedAt)
	wantCode(t, err, apperror.CodeMarketplaceInvalidTransition)
	var n int
	if err := w.pool.QueryRow(w.ctx, `SELECT count(*) FROM public.avatars WHERE id = $1`, copyID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("buyer avatar must survive a suspend: %d %v", n, err)
	}
	var status string
	if err := w.pool.QueryRow(w.ctx, `SELECT status FROM public.avatar_assets WHERE id = $1`, asset).Scan(&status); err != nil || status != "ready" {
		t.Fatalf("a suspend must not block the asset: %q %v", status, err)
	}
	// Only an active listing can be suspended.
	avatar2, _ := w.avatar(seller)
	d := w.draft(seller, avatar2, "Draft", 5)
	_, err = w.svc.Suspend(w.ctx, admin, "admin", d.ID, "why")
	wantCode(t, err, apperror.CodeMarketplaceInvalidTransition)
	_, err = w.svc.Suspend(w.ctx, admin, "admin", uuid.New(), "why")
	wantCode(t, err, apperror.CodeMarketplaceListingNotFound)
}

// Exactly one of many simultaneous decisions on the same pending payload may win, and a seller edit never
// slips into a pending listing.
func TestConcurrentDecisions(t *testing.T) {
	w := newWorld(t)
	seller, admin := w.user("candidate"), w.user("admin")
	avatar, _ := w.avatar(seller)
	l := w.pending(seller, avatar, "Race", 10)

	var wg sync.WaitGroup
	var mu sync.Mutex
	wins, stale := 0, 0
	record := func(err error) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case err == nil:
			wins++
		case apperror.IsCode(err, apperror.CodeMarketplaceListingStale), apperror.IsCode(err, apperror.CodeMarketplaceInvalidTransition):
			stale++
		default:
			t.Errorf("unexpected: %v", err)
		}
	}
	for i := 0; i < 6; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			_, err := w.svc.Approve(w.ctx, admin, "admin", l.ID, l.UpdatedAt)
			record(err)
		}()
		go func() {
			defer wg.Done()
			_, err := w.svc.Reject(w.ctx, admin, "admin", l.ID, l.UpdatedAt, "no")
			record(err)
		}()
		go func() {
			defer wg.Done()
			if _, err := w.svc.Edit(w.ctx, seller, l.ID, l.UpdatedAt, payload("changed while reviewed", 99)); err == nil {
				t.Error("a seller edit of a pending listing must never succeed")
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("exactly one decision must win, got %d (stale %d)", wins, stale)
	}
	final, err := w.svc.AdminGet(w.ctx, "admin", l.ID)
	if err != nil || final.Title != "Race" || final.PriceCoins != 10 {
		t.Fatalf("the reviewed payload must be intact: %+v %v", final, err)
	}
}

func TestCatalog(t *testing.T) {
	w := newWorld(t)
	seller, admin, buyer, stranger := w.user("candidate"), w.user("admin"), w.user("recruiter"), w.user("candidate")
	tag := uuid.New().String()[:8]
	mk := func(title string, price int32) domain.Listing {
		avatar, _ := w.avatar(seller)
		return w.active(seller, avatar, admin, title+" "+tag, price)
	}
	cheap, mid, dear := mk("Cheap", 5), mk("Mid", 50), mk("Dear", 500)

	// One listing per other status must never appear.
	hidden := map[string]uuid.UUID{}
	a, _ := w.avatar(seller)
	hidden["draft"] = w.draft(seller, a, "Hidden draft "+tag, 5).ID
	a, _ = w.avatar(seller)
	hidden["pending"] = w.pending(seller, a, "Hidden pending "+tag, 5).ID
	a, _ = w.avatar(seller)
	p := w.pending(seller, a, "Hidden rejected "+tag, 5)
	if _, err := w.svc.Reject(w.ctx, admin, "admin", p.ID, p.UpdatedAt, "no"); err != nil {
		t.Fatal(err)
	}
	hidden["rejected"] = p.ID
	a, _ = w.avatar(seller)
	d := w.active(seller, a, admin, "Hidden delisted "+tag, 5)
	if _, err := w.svc.Delist(w.ctx, seller, d.ID, d.UpdatedAt); err != nil {
		t.Fatal(err)
	}
	hidden["delisted"] = d.ID
	a, _ = w.avatar(seller)
	s := w.active(seller, a, admin, "Hidden suspended "+tag, 5)
	if _, err := w.svc.Suspend(w.ctx, admin, "admin", s.ID, "bad"); err != nil {
		t.Fatal(err)
	}
	hidden["suspended"] = s.ID
	a, asset := w.avatar(seller)
	b := w.active(seller, a, admin, "Hidden blocked "+tag, 5)
	w.exec(`UPDATE public.avatar_assets SET status = 'blocked' WHERE id = $1`, asset)
	hidden["blocked asset"] = b.ID

	for name, id := range hidden {
		if _, err := w.svc.CatalogItem(w.ctx, buyer, id); !apperror.IsCode(err, apperror.CodeMarketplaceListingNotFound) {
			t.Errorf("%s listing must not be visible, got %v", name, err)
		}
	}
	items, total, err := w.svc.Catalog(w.ctx, buyer, domain.ListFilter{Query: tag, Limit: 50})
	if err != nil || total != 3 || len(items) != 3 {
		t.Fatalf("search by tag: %d/%d %v", len(items), total, err)
	}
	for _, sortCase := range []struct {
		sort  domain.CatalogSort
		first uuid.UUID
	}{{domain.SortPriceAsc, cheap.ID}, {domain.SortPriceDesc, dear.ID}, {domain.SortNewest, dear.ID}} {
		items, _, err := w.svc.Catalog(w.ctx, buyer, domain.ListFilter{Query: tag, Sort: sortCase.sort, Limit: 50})
		if err != nil || items[0].ID != sortCase.first {
			t.Errorf("sort %s: first %v want %v (%v)", sortCase.sort, items[0].ID, sortCase.first, err)
		}
	}
	page2, total, err := w.svc.Catalog(w.ctx, buyer, domain.ListFilter{Query: tag, Sort: domain.SortPriceAsc, Limit: 1, Offset: 1})
	if err != nil || len(page2) != 1 || page2[0].ID != mid.ID || total != 3 {
		t.Fatalf("pagination: %+v %d %v", page2, total, err)
	}
	if items, total, _ = w.svc.Catalog(w.ctx, buyer, domain.ListFilter{Query: "%" + tag, Limit: 50}); total != 0 || len(items) != 0 {
		t.Fatal("LIKE wildcards in the query must be literal")
	}
	for name, f := range map[string]domain.ListFilter{
		"limit 0":    {Limit: 0},
		"limit 51":   {Limit: 51},
		"offset big": {Limit: 10, Offset: domain.MaxOffset + 1},
		"long query": {Limit: 10, Query: strings.Repeat("a", 101)},
		"bad sort":   {Limit: 10, Sort: "oldest"},
	} {
		if _, _, err := w.svc.Catalog(w.ctx, buyer, f); !apperror.IsCode(err, apperror.CodeValidation) {
			t.Errorf("%s: want validation error, got %v", name, err)
		}
	}

	// already_owned comes from the inventory; the seller sees is_own_listing.
	w.copyOf(buyer, cheap.AssetID)
	item, err := w.svc.CatalogItem(w.ctx, buyer, cheap.ID)
	if err != nil || !item.AlreadyOwned || item.IsOwnListing {
		t.Fatalf("buyer with a copy: %+v %v", item, err)
	}
	if item, _ = w.svc.CatalogItem(w.ctx, stranger, cheap.ID); item.AlreadyOwned {
		t.Fatal("a user without a copy does not own it")
	}
	if item, _ = w.svc.CatalogItem(w.ctx, seller, mid.ID); !item.IsOwnListing || !item.AlreadyOwned {
		t.Fatalf("seller view: %+v", item)
	}
	// A bought copy deleted again no longer counts, and a transaction description never does.
	w.exec(`DELETE FROM public.avatars WHERE inventory_id IN (SELECT id FROM public.inventories WHERE user_id = $1) AND asset_id = $2`, buyer, cheap.AssetID)
	if item, _ = w.svc.CatalogItem(w.ctx, buyer, cheap.ID); item.AlreadyOwned {
		t.Fatal("already_owned must follow the inventory")
	}

	// Non Candidate/Recruiter accounts get nothing.
	if _, _, err := w.svc.Catalog(w.ctx, admin, domain.ListFilter{Limit: 10}); !apperror.IsCode(err, apperror.CodeForbidden) {
		t.Fatalf("admin has no catalog capability: %v", err)
	}
	if _, _, err := w.svc.Catalog(w.ctx, uuid.New(), domain.ListFilter{Limit: 10}); !apperror.IsCode(err, apperror.CodeForbidden) {
		t.Fatalf("unknown account: %v", err)
	}
}

// A preview link is signed only for a listing the caller may see, always for previews/<asset_id>.webp.
func TestPreviewSigning(t *testing.T) {
	w := newWorld(t)
	seller, admin, buyer := w.user("candidate"), w.user("admin"), w.user("recruiter")
	avatar, asset := w.avatar(seller)
	l := w.pending(seller, avatar, "Preview", 10)
	want := "https://storage.test/previews/" + asset.String() + ".webp"

	if _, err := w.svc.CatalogPreview(w.ctx, buyer, l.ID); !apperror.IsCode(err, apperror.CodeMarketplaceListingNotFound) {
		t.Fatalf("a pending listing has no public preview: %v", err)
	}
	if _, err := w.svc.MyListingPreview(w.ctx, buyer, l.ID); !apperror.IsCode(err, apperror.CodeMarketplaceListingNotFound) {
		t.Fatalf("another user has no preview of my listing: %v", err)
	}
	if _, err := w.svc.AdminPreview(w.ctx, "candidate", l.ID); !apperror.IsCode(err, apperror.CodeForbidden) {
		t.Fatalf("non-admin: %v", err)
	}
	if len(w.signer.keys) != 0 {
		t.Fatalf("nothing may be signed for refused requests: %v", w.signer.keys)
	}
	if u, err := w.svc.MyListingPreview(w.ctx, seller, l.ID); err != nil || u != want {
		t.Fatalf("seller preview: %q %v", u, err)
	}
	if u, err := w.svc.AdminPreview(w.ctx, "admin", l.ID); err != nil || u != want {
		t.Fatalf("admin preview: %q %v", u, err)
	}
	if _, err := w.svc.Approve(w.ctx, admin, "admin", l.ID, l.UpdatedAt); err != nil {
		t.Fatal(err)
	}
	if u, err := w.svc.CatalogPreview(w.ctx, buyer, l.ID); err != nil || u != want {
		t.Fatalf("catalog preview: %q %v", u, err)
	}
	for _, k := range w.signer.keys {
		if !strings.HasPrefix(k, "previews/") || strings.Contains(k, ".vrm") {
			t.Errorf("only previews/ keys may be signed, got %s", k)
		}
	}
}
