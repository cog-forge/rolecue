//go:build integration

package tests

import (
	"context"
	"sync"
	"testing"
	"time"

	invrepo "github.com/cog-forge/rolecue/api/internal/features/inventory/repository"
	"github.com/cog-forge/rolecue/api/internal/features/marketplace/domain"
	"github.com/cog-forge/rolecue/api/internal/features/marketplace/repository"
	"github.com/cog-forge/rolecue/api/internal/pkg/ledger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// ---- helpers ----

func (w *world) fund(user uuid.UUID, balance int32) {
	w.t.Helper()
	w.exec(`INSERT INTO public.wallets (user_id, balance) VALUES ($1, $2) ON CONFLICT (user_id) DO UPDATE SET balance = $2`, user, balance)
}

// balance is -1 when the user has no wallet.
func (w *world) balance(user uuid.UUID) int32 {
	w.t.Helper()
	var b int32 = -1
	rows, err := w.pool.Query(w.ctx, `SELECT balance FROM public.wallets WHERE user_id = $1`, user)
	if err != nil {
		w.t.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		if err := rows.Scan(&b); err != nil {
			w.t.Fatal(err)
		}
	}
	return b
}

func (w *world) count(sql string, args ...any) int {
	w.t.Helper()
	var n int
	if err := w.pool.QueryRow(w.ctx, sql, args...).Scan(&n); err != nil {
		w.t.Fatal(err)
	}
	return n
}

func (w *world) txRows(buyer uuid.UUID) int {
	return w.count(`SELECT count(*) FROM public.transactions WHERE "from" = $1`, buyer.String())
}

func (w *world) owned(buyer, asset uuid.UUID) int {
	return w.count(`SELECT count(*) FROM public.avatars a JOIN public.inventories i ON i.id = a.inventory_id WHERE i.user_id = $1 AND a.asset_id = $2`, buyer, asset)
}

type shop struct {
	seller, admin uuid.UUID
	listing       domain.Listing
	avatar        uuid.UUID
}

// shop seeds a seller with an avatar and an active listing at price. The seller has a wallet of 10.
func (w *world) shop(price int32) shop {
	w.t.Helper()
	s := shop{seller: w.user("candidate"), admin: w.user("admin")}
	var asset uuid.UUID
	s.avatar, asset = w.avatar(s.seller)
	_ = asset
	s.listing = w.active(s.seller, s.avatar, s.admin, "Alex the interviewer", price)
	w.fund(s.seller, 10)
	return s
}

func (w *world) buyer(balance int32) uuid.UUID {
	w.t.Helper()
	b := w.user("candidate")
	w.inventory(b)
	if balance >= 0 {
		w.fund(b, balance)
	}
	return b
}

func newKey(t *testing.T, listing uuid.UUID, price int32) string {
	t.Helper()
	key, err := ledger.NewMarketplaceKey(listing, price, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func (w *world) buy(buyer uuid.UUID, s shop, price int32, key string) (domain.PurchaseResult, error) {
	return w.svc.Purchase(w.ctx, buyer, s.listing.ID, price, key)
}

type snapshot struct {
	buyer, seller int32
	tx, avatars   int
}

func (w *world) snap(buyer uuid.UUID, s shop) snapshot {
	return snapshot{w.balance(buyer), w.balance(s.seller), w.txRows(buyer),
		w.count(`SELECT count(*) FROM public.avatars a JOIN public.inventories i ON i.id = a.inventory_id WHERE i.user_id = $1`, buyer)}
}

// ---- happy path and refusals ----

func TestCheckoutMovesCoinsAndGrantsOneAvatar(t *testing.T) {
	w := newWorld(t)
	s := w.shop(20)
	buyer := w.buyer(50)
	key := newKey(t, s.listing.ID, 20)
	res, err := w.buy(buyer, s, 20, key)
	if err != nil {
		t.Fatal(err)
	}
	if res.Replayed || res.Avatar == nil || res.Balance != 30 || res.Receipt.Amount != 20 || res.Receipt.Currency != "COIN" {
		t.Fatalf("result %+v", res)
	}
	if w.balance(buyer) != 30 || w.balance(s.seller) != 30 {
		t.Fatalf("balances buyer=%d seller=%d", w.balance(buyer), w.balance(s.seller))
	}
	if n := w.count(`SELECT count(*) FROM public.transactions WHERE "from" = $1 AND "to" = $2 AND amount = 20 AND currency = 'COIN' AND status = 'success' AND idempotency_key = $3`,
		buyer.String(), s.seller.String(), key); n != 1 {
		t.Fatalf("%d coin rows", n)
	}
	if w.txRows(buyer) != 1 || w.owned(buyer, s.listing.AssetID) != 1 {
		t.Fatalf("rows tx=%d owned=%d", w.txRows(buyer), w.owned(buyer, s.listing.AssetID))
	}
	var name, personality string
	var asset uuid.UUID
	if err := w.pool.QueryRow(w.ctx, `SELECT name, personality, asset_id FROM public.avatars WHERE id = $1`, res.Avatar.ID).Scan(&name, &personality, &asset); err != nil {
		t.Fatal(err)
	}
	if name != "Alex the interviewer" || personality != "calm and precise" || asset != s.listing.AssetID {
		t.Errorf("avatar %q %q asset=%v", name, personality, asset)
	}
	// The seller keeps the original avatar and the asset is shared, never copied or changed.
	if w.owned(s.seller, s.listing.AssetID) != 1 || w.count(`SELECT count(*) FROM public.avatar_assets WHERE id = $1 AND created_by = $2`, s.listing.AssetID, s.seller) != 1 {
		t.Error("seller avatar or asset touched")
	}
}

func TestCheckoutCreatesTheSellerWalletAndRollsItBackOnRefusal(t *testing.T) {
	w := newWorld(t)
	s := w.shop(20)
	if _, err := w.pool.Exec(w.ctx, `DELETE FROM public.wallets WHERE user_id = $1`, s.seller); err != nil {
		t.Fatal(err)
	}
	poor := w.buyer(5)
	_, err := w.buy(poor, s, 20, newKey(t, s.listing.ID, 20))
	wantCode(t, err, apperror.CodeInsufficientBalance)
	if w.balance(s.seller) != -1 || w.balance(poor) != 5 || w.txRows(poor) != 0 || w.owned(poor, s.listing.AssetID) != 0 {
		t.Fatal("a refused purchase left a seller wallet, a debit, a row or an avatar")
	}
	rich := w.buyer(50)
	if _, err := w.buy(rich, s, 20, newKey(t, s.listing.ID, 20)); err != nil {
		t.Fatal(err)
	}
	if w.balance(s.seller) != 20 {
		t.Fatalf("seller wallet %d", w.balance(s.seller))
	}
}

func TestCheckoutRefusalsWriteNothing(t *testing.T) {
	w := newWorld(t)
	s := w.shop(20)
	setStatus := func(status string) {
		w.exec(`UPDATE public.avatar_listings SET status = $2 WHERE id = $1`, s.listing.ID, status)
	}
	for name, c := range map[string]struct {
		prep  func(buyer uuid.UUID) uuid.UUID // returns the buyer to use
		price int32
		want  apperror.Code
	}{
		"insufficient": {func(b uuid.UUID) uuid.UUID { w.fund(b, 19); return b }, 20, apperror.CodeInsufficientBalance},
		"no wallet":    {func(b uuid.UUID) uuid.UUID { w.exec(`DELETE FROM public.wallets WHERE user_id = $1`, b); return b }, 20, apperror.CodeInsufficientBalance},
		"self":         {func(uuid.UUID) uuid.UUID { w.fund(s.seller, 100); return s.seller }, 20, apperror.CodeMarketplaceOwnListing},
		"already owns": {func(b uuid.UUID) uuid.UUID { w.copyOf(b, s.listing.AssetID); return b }, 20, apperror.CodeMarketplaceAlreadyOwned},
		"price":        {func(b uuid.UUID) uuid.UUID { return b }, 19, apperror.CodeMarketplacePriceChanged},
		"draft":        {func(b uuid.UUID) uuid.UUID { setStatus("draft"); return b }, 20, apperror.CodeMarketplaceListingNotFound},
		"pending":      {func(b uuid.UUID) uuid.UUID { setStatus("pending"); return b }, 20, apperror.CodeMarketplaceListingNotFound},
		"rejected":     {func(b uuid.UUID) uuid.UUID { setStatus("rejected"); return b }, 20, apperror.CodeMarketplaceListingNotFound},
		"delisted":     {func(b uuid.UUID) uuid.UUID { setStatus("delisted"); return b }, 20, apperror.CodeMarketplaceListingUnavailable},
		"suspended":    {func(b uuid.UUID) uuid.UUID { setStatus("suspended"); return b }, 20, apperror.CodeMarketplaceListingUnavailable},
		"blocked asset": {func(b uuid.UUID) uuid.UUID {
			w.exec(`UPDATE public.avatar_assets SET status = 'blocked' WHERE id = $1`, s.listing.AssetID)
			return b
		}, 20, apperror.CodeMarketplaceAssetNotReady},
		"slot full": {func(b uuid.UUID) uuid.UUID {
			w.avatar(b)
			w.exec(`UPDATE public.inventories SET slot = 1 WHERE user_id = $1`, b)
			return b
		}, 20, apperror.CodeInventoryFull},
		"admin account": {func(uuid.UUID) uuid.UUID { return s.admin }, 20, apperror.CodeForbidden},
		"locked account": {func(b uuid.UUID) uuid.UUID {
			w.exec(`UPDATE public.users SET is_locked = true WHERE id = $1`, b)
			return b
		}, 20, apperror.CodeForbidden},
		"unverified": {func(b uuid.UUID) uuid.UUID {
			w.exec(`UPDATE public.users SET email_verified = false WHERE id = $1`, b)
			return b
		}, 20, apperror.CodeForbidden},
	} {
		t.Run(name, func(t *testing.T) {
			// A fresh listing state per case: reset what earlier cases changed.
			w.exec(`UPDATE public.avatar_listings SET status = 'active' WHERE id = $1`, s.listing.ID)
			w.exec(`UPDATE public.avatar_assets SET status = 'ready' WHERE id = $1`, s.listing.AssetID)
			w.t = t
			buyer := c.prep(w.buyer(50))
			before := w.snap(buyer, s)
			_, err := w.buy(buyer, s, c.price, newKey(t, s.listing.ID, c.price))
			wantCode(t, err, c.want)
			if after := w.snap(buyer, s); after != before {
				t.Fatalf("a refusal wrote something: %+v -> %+v", before, after)
			}
		})
	}
	w.t = t
	_, err := w.svc.Purchase(w.ctx, w.buyer(50), uuid.New(), 20, newKey(t, uuid.New(), 20))
	if err == nil {
		t.Fatal("a key for another listing must be refused")
	}
	missing := uuid.New()
	_, err = w.svc.Purchase(w.ctx, w.buyer(50), missing, 20, newKey(t, missing, 20))
	wantCode(t, err, apperror.CodeMarketplaceListingNotFound)
}

func TestCheckoutRollsBackTheGrantWhenTheTransferFails(t *testing.T) {
	w := newWorld(t)
	s := w.shop(20)
	w.fund(s.seller, 2147483647-5) // the seller cannot hold 20 more coins
	buyer := w.buyer(50)
	before := w.snap(buyer, s)
	_, err := w.buy(buyer, s, 20, newKey(t, s.listing.ID, 20))
	wantCode(t, err, apperror.CodeCoinRecipientBalanceLimit)
	if after := w.snap(buyer, s); after != before {
		t.Fatalf("avatar grant or coins survived a failed transfer: %+v -> %+v", before, after)
	}
}

// ---- idempotency ----

func TestCheckoutReplayChargesAndGrantsOnce(t *testing.T) {
	w := newWorld(t)
	s := w.shop(20)
	buyer := w.buyer(50)
	key := newKey(t, s.listing.ID, 20)
	first, err := w.buy(buyer, s, 20, key)
	if err != nil {
		t.Fatal(err)
	}
	again, err := w.buy(buyer, s, 20, key)
	if err != nil {
		t.Fatal(err)
	}
	if !again.Replayed || again.Receipt.TransactionID != first.Receipt.TransactionID || again.Balance != 30 ||
		again.Avatar == nil || again.Avatar.ID != first.Avatar.ID {
		t.Fatalf("replay %+v vs %+v", again, first)
	}
	if w.balance(buyer) != 30 || w.balance(s.seller) != 30 || w.txRows(buyer) != 1 || w.owned(buyer, s.listing.AssetID) != 1 {
		t.Fatal("replay changed state")
	}
}

func TestCheckoutKeyConflicts(t *testing.T) {
	w := newWorld(t)
	s, other := w.shop(20), w.shop(20)
	buyer := w.buyer(100)
	key := newKey(t, s.listing.ID, 20)
	if _, err := w.buy(buyer, s, 20, key); err != nil {
		t.Fatal(err)
	}
	// The same key text with another listing or another price is never a replay.
	_, err := w.svc.Purchase(w.ctx, buyer, other.listing.ID, 20, key)
	wantCode(t, err, apperror.CodeCoinTransferKeyConflict)
	_, err = w.buy(buyer, s, 21, key)
	wantCode(t, err, apperror.CodeCoinTransferKeyConflict)
	// A row of another operation under a Marketplace-shaped key (a legacy slot row) is a conflict, not a replay.
	slotKey := newKey(t, other.listing.ID, 20)
	w.exec(`INSERT INTO public.transactions (amount, "to", currency, status, "from", idempotency_key) VALUES (20, 'system', 'COIN', 'success', $1, $2)`, buyer.String(), slotKey)
	before := w.snap(buyer, other)
	_, err = w.svc.Purchase(w.ctx, buyer, other.listing.ID, 20, slotKey)
	wantCode(t, err, apperror.CodeCoinTransferKeyConflict)
	if w.snap(buyer, other) != before {
		t.Fatal("a conflicting key wrote something")
	}
	for _, bad := range []string{"", "slot-0001", "mkt1:nope"} {
		_, err = w.buy(buyer, s, 20, bad)
		wantCode(t, err, apperror.CodeCoinTransferKeyInvalid)
	}
}

func TestCheckoutReplayAfterSuspendAndAfterDeleteNeverRestores(t *testing.T) {
	w := newWorld(t)
	s := w.shop(20)
	buyer := w.buyer(100)
	key := newKey(t, s.listing.ID, 20)
	first, err := w.buy(buyer, s, 20, key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.svc.Suspend(w.ctx, s.admin, "admin", s.listing.ID, "policy"); err != nil {
		t.Fatal(err)
	}
	if res, err := w.buy(buyer, s, 20, key); err != nil || !res.Replayed || res.Avatar == nil {
		t.Fatalf("replay after suspend: %+v %v", res, err)
	}
	w.exec(`DELETE FROM public.avatars WHERE id = $1`, first.Avatar.ID) // the buyer deletes the avatar
	res, err := w.buy(buyer, s, 20, key)
	if err != nil || !res.Replayed || res.Avatar != nil || res.Receipt.TransactionID != first.Receipt.TransactionID {
		t.Fatalf("replay after delete: %+v %v", res, err)
	}
	if w.owned(buyer, s.listing.AssetID) != 0 || w.balance(buyer) != 80 || w.txRows(buyer) != 1 {
		t.Fatal("a replay restored the avatar or charged again")
	}
	// A new key buys again while the listing is active and the asset exists; here the listing is suspended.
	_, err = w.buy(buyer, s, 20, newKey(t, s.listing.ID, 20))
	wantCode(t, err, apperror.CodeMarketplaceListingUnavailable)
}

func TestCheckoutNewKeyAfterDeleteBuysAgain(t *testing.T) {
	w := newWorld(t)
	s := w.shop(20)
	buyer := w.buyer(100)
	first, err := w.buy(buyer, s, 20, newKey(t, s.listing.ID, 20))
	if err != nil {
		t.Fatal(err)
	}
	w.exec(`DELETE FROM public.avatars WHERE id = $1`, first.Avatar.ID)
	second, err := w.buy(buyer, s, 20, newKey(t, s.listing.ID, 20))
	if err != nil || second.Replayed || second.Avatar == nil || second.Avatar.ID == first.Avatar.ID {
		t.Fatalf("%+v %v", second, err)
	}
	if w.balance(buyer) != 60 || w.balance(s.seller) != 50 || w.txRows(buyer) != 2 || w.owned(buyer, s.listing.AssetID) != 1 {
		t.Fatal("second purchase state")
	}
}

func TestCheckoutReplayWhenTheListingIsGone(t *testing.T) {
	w := newWorld(t) // what the CASCADE of an erased seller does to the listing
	s := w.shop(20)
	buyer := w.buyer(50)
	key := newKey(t, s.listing.ID, 20)
	first, err := w.buy(buyer, s, 20, key)
	if err != nil {
		t.Fatal(err)
	}
	w.exec(`DELETE FROM public.avatar_listings WHERE id = $1`, s.listing.ID)
	res, err := w.buy(buyer, s, 20, key)
	if err != nil || !res.Replayed || res.Receipt.TransactionID != first.Receipt.TransactionID || res.Avatar != nil {
		t.Fatalf("%+v %v", res, err)
	}
	_, err = w.buy(buyer, s, 20, newKey(t, s.listing.ID, 20))
	wantCode(t, err, apperror.CodeMarketplaceListingNotFound)
}

// ---- concurrency ----

func TestCheckoutConcurrentSameKeyDebitsOnce(t *testing.T) {
	w := newWorld(t)
	s := w.shop(20)
	buyer := w.buyer(50)
	key := newKey(t, s.listing.ID, 20)
	var wg sync.WaitGroup
	results := make([]domain.PurchaseResult, 8)
	errs := make([]error, 8)
	for i := range results {
		wg.Add(1)
		go func() { defer wg.Done(); results[i], errs[i] = w.buy(buyer, s, 20, key) }()
	}
	wg.Wait()
	fresh := 0
	for i := range results {
		if errs[i] != nil {
			t.Fatalf("request %d: %v", i, errs[i])
		}
		if !results[i].Replayed {
			fresh++
		}
	}
	if fresh != 1 || w.balance(buyer) != 30 || w.balance(s.seller) != 30 || w.txRows(buyer) != 1 || w.owned(buyer, s.listing.AssetID) != 1 {
		t.Fatalf("fresh=%d buyer=%d seller=%d tx=%d owned=%d", fresh, w.balance(buyer), w.balance(s.seller), w.txRows(buyer), w.owned(buyer, s.listing.AssetID))
	}
}

func TestCheckoutConcurrentDifferentKeysSameBuyerBuyOnce(t *testing.T) {
	w := newWorld(t)
	s := w.shop(20)
	buyer := w.buyer(100)
	var wg sync.WaitGroup
	errs := make([]error, 5)
	for i := range errs {
		wg.Add(1)
		key := newKey(t, s.listing.ID, 20)
		go func() { defer wg.Done(); _, errs[i] = w.buy(buyer, s, 20, key) }()
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case apperror.IsCode(err, apperror.CodeMarketplaceAlreadyOwned):
		default:
			t.Fatalf("unexpected %v", err)
		}
	}
	if ok != 1 || w.balance(buyer) != 80 || w.txRows(buyer) != 1 || w.owned(buyer, s.listing.AssetID) != 1 {
		t.Fatalf("ok=%d buyer=%d tx=%d owned=%d", ok, w.balance(buyer), w.txRows(buyer), w.owned(buyer, s.listing.AssetID))
	}
}

func TestCheckoutTwoBuyersBothGetACopy(t *testing.T) {
	w := newWorld(t)
	s := w.shop(20)
	a, b := w.buyer(50), w.buyer(50)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, buyer := range []uuid.UUID{a, b} {
		wg.Add(1)
		key := newKey(t, s.listing.ID, 20)
		go func() { defer wg.Done(); _, errs[i] = w.buy(buyer, s, 20, key) }()
	}
	wg.Wait()
	if errs[0] != nil || errs[1] != nil {
		t.Fatal(errs)
	}
	if w.balance(s.seller) != 50 || w.balance(a) != 30 || w.balance(b) != 30 || w.count(`SELECT count(*) FROM public.avatars WHERE asset_id = $1`, s.listing.AssetID) != 3 {
		t.Fatal("two buyers: balances or copies wrong")
	}
}

// A purchase racing a suspend or a delist either completes in full or fails with a clear refusal and writes nothing.
func TestCheckoutRacingSuspendAndDelist(t *testing.T) {
	w := newWorld(t)
	for i := 0; i < 20; i++ {
		s := w.shop(20)
		buyer := w.buyer(50)
		var wg sync.WaitGroup
		var buyErr, stopErr error
		wg.Add(2)
		go func() { defer wg.Done(); _, buyErr = w.buy(buyer, s, 20, newKey(t, s.listing.ID, 20)) }()
		go func() {
			defer wg.Done()
			if i%2 == 0 {
				_, stopErr = w.svc.Suspend(w.ctx, s.admin, "admin", s.listing.ID, "race")
			} else {
				_, stopErr = w.svc.Delist(w.ctx, s.seller, s.listing.ID, s.listing.UpdatedAt)
			}
		}()
		wg.Wait()
		if stopErr != nil {
			t.Fatalf("iteration %d: stop failed: %v", i, stopErr)
		}
		switch {
		case buyErr == nil:
			if w.balance(buyer) != 30 || w.balance(s.seller) != 30 || w.txRows(buyer) != 1 || w.owned(buyer, s.listing.AssetID) != 1 {
				t.Fatalf("iteration %d: half purchase", i)
			}
		case apperror.IsCode(buyErr, apperror.CodeMarketplaceListingUnavailable):
			if w.balance(buyer) != 50 || w.balance(s.seller) != 10 || w.txRows(buyer) != 0 || w.owned(buyer, s.listing.AssetID) != 0 {
				t.Fatalf("iteration %d: refused purchase wrote something", i)
			}
		default:
			t.Fatalf("iteration %d: %v", i, buyErr)
		}
	}
}

// Deterministic: a suspend that holds the listing row first makes the purchase wait, then refuse.
func TestCheckoutWaitsForASuspendInFlight(t *testing.T) {
	w := newWorld(t)
	s := w.shop(20)
	buyer := w.buyer(50)
	tx, err := w.pool.Begin(w.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(w.ctx, `UPDATE public.avatar_listings SET status = 'suspended', updated_at = now() WHERE id = $1`, s.listing.ID); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := w.buy(buyer, s, 20, newKey(t, s.listing.ID, 20)); done <- err }()
	select {
	case err := <-done:
		t.Fatalf("purchase did not wait for the suspend: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	if err := tx.Commit(w.ctx); err != nil {
		t.Fatal(err)
	}
	wantCode(t, <-done, apperror.CodeMarketplaceListingUnavailable)
	if w.balance(buyer) != 50 || w.txRows(buyer) != 0 {
		t.Fatal("refused purchase wrote something")
	}
}

// Opposite purchases (A buys B's avatar while B buys A's) lock the two wallets in the same order. One attempt each,
// no retry: any deadlock would show as a retryable error.
func TestCheckoutOppositePurchasesDoNotDeadlock(t *testing.T) {
	w := newWorld(t)
	repo := repository.NewRepository(w.pool)
	for i := 0; i < 15; i++ {
		a, b := w.shop(7), w.shop(9)
		w.fund(a.seller, 100)
		w.fund(b.seller, 100)
		total := w.balance(a.seller) + w.balance(b.seller)
		var wg sync.WaitGroup
		errs := make([]error, 2)
		for n, p := range []struct {
			buyer uuid.UUID
			shop  shop
			price int32
		}{{a.seller, b, 9}, {b.seller, a, 7}} {
			wg.Add(1)
			req, err := domain.NewPurchaseRequest(p.buyer, p.shop.listing.ID, p.price, newKey(t, p.shop.listing.ID, p.price))
			if err != nil {
				t.Fatal(err)
			}
			go func() { defer wg.Done(); _, errs[n] = repo.Checkout(w.ctx, req) }()
		}
		wg.Wait()
		for _, err := range errs {
			if err != nil {
				t.Fatalf("iteration %d: %v (retryable=%v)", i, err, ledger.IsRetryable(err))
			}
		}
		if got := w.balance(a.seller) + w.balance(b.seller); got != total {
			t.Fatalf("iteration %d: coins not conserved %d != %d", i, got, total)
		}
	}
}

// ---- hardening items ----

func TestSlotPurchaseCannotUseAMarketplaceKey(t *testing.T) {
	w := newWorld(t)
	s := w.shop(3) // 3 coins = 3 slots at 1 coin each, the case where a loose lookup would replay
	buyer := w.buyer(50)
	key := newKey(t, s.listing.ID, 3)
	if _, err := w.buy(buyer, s, 3, key); err != nil {
		t.Fatal(err)
	}
	repo := invrepo.NewRepository(w.pool)
	slotsBefore := w.count(`SELECT slot FROM public.inventories WHERE user_id = $1`, buyer)
	if _, err := repo.PurchaseSlot(w.ctx, buyer, 3, key); err == nil {
		t.Fatal("a slot purchase accepted a Marketplace row as its own replay")
	}
	if w.count(`SELECT slot FROM public.inventories WHERE user_id = $1`, buyer) != slotsBefore || w.balance(buyer) != 47 || w.txRows(buyer) != 1 {
		t.Fatal("the refused slot purchase changed state")
	}
}

func TestPurchasedAvatarsDoNotUseTheConversionQuota(t *testing.T) {
	w := newWorld(t)
	s := w.shop(20)
	buyer := w.buyer(50)
	repo := invrepo.NewRepository(w.pool)
	since := time.Now().Add(-24 * time.Hour)
	count := func(u uuid.UUID) int64 {
		n, err := repo.CountRecentAvatars(w.ctx, u, since)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	if count(buyer) != 0 || count(s.seller) != 1 {
		t.Fatalf("before: buyer=%d seller=%d", count(buyer), count(s.seller))
	}
	if _, err := repo.Create(w.ctx, buyer, "Mine", "calm", "mp-test/"+uuid.NewString()+".vrm"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.buy(buyer, s, 20, newKey(t, s.listing.ID, 20)); err != nil {
		t.Fatal(err)
	}
	if count(buyer) != 1 || count(s.seller) != 1 {
		t.Fatalf("after purchase: buyer=%d seller=%d (a bought avatar must not count, a self-created one must)", count(buyer), count(s.seller))
	}
}

// Two purchases of different listings race for the buyer's last free slot: exactly one wins, the other writes nothing.
func TestCheckoutSlotRace(t *testing.T) {
	w := newWorld(t)
	a, b := w.shop(20), w.shop(20)
	buyer := w.buyer(100)
	w.avatar(buyer)
	w.exec(`UPDATE public.inventories SET slot = 2 WHERE user_id = $1`, buyer) // one avatar, one free slot
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, s := range []shop{a, b} {
		wg.Add(1)
		key := newKey(t, s.listing.ID, 20)
		go func() { defer wg.Done(); _, errs[i] = w.buy(buyer, s, 20, key) }()
	}
	wg.Wait()
	won := 0
	for _, err := range errs {
		switch {
		case err == nil:
			won++
		case apperror.IsCode(err, apperror.CodeInventoryFull):
		default:
			t.Fatalf("unexpected %v", err)
		}
	}
	if won != 1 || w.balance(buyer) != 80 || w.txRows(buyer) != 1 || w.count(`SELECT count(*) FROM public.avatars a JOIN public.inventories i ON i.id = a.inventory_id WHERE i.user_id = $1`, buyer) != 2 {
		t.Fatalf("won=%d buyer=%d tx=%d", won, w.balance(buyer), w.txRows(buyer))
	}
}
