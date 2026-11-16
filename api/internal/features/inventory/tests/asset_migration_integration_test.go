//go:build integration

package tests

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// scratchDatabase creates an empty database next to the one DATABASE_URL names, migrates it to version 000035 from
// the migration files and returns a connection to it. The database is dropped when the test ends, so the legacy
// data the test writes never touches the shared clone.
func scratchDatabase(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is not set (load ../.env)")
	}
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	if err := admin.Ping(ctx); err != nil {
		t.Skipf("database not reachable: %v", err)
	}
	name := "kan360_mig_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Skipf("cannot create a scratch database: %v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
	})
	scratch := cfg.Copy()
	scratch.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, scratch)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	files, err := filepath.Glob("../../../../migrations/*.up.sql")
	if err != nil || len(files) == 0 {
		t.Fatalf("migration files: %v, %v", files, err)
	}
	sort.Strings(files)
	for _, file := range files {
		if strings.Contains(filepath.Base(file), "000036_") {
			break
		}
		applyFile(t, ctx, pool, file)
	}
	return pool
}

func applyFile(t *testing.T, ctx context.Context, pool *pgxpool.Pool, file string) {
	t.Helper()
	sql, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	// The file carries its own BEGIN/COMMIT; the simple protocol runs it as one multi-statement string.
	if _, err := pool.Exec(ctx, string(sql), pgx.QueryExecModeSimpleProtocol); err != nil {
		t.Fatalf("%s: %v", filepath.Base(file), err)
	}
}

func scalar[T any](t *testing.T, ctx context.Context, pool *pgxpool.Pool, sql string, args ...any) T {
	t.Helper()
	var v T
	if err := pool.QueryRow(ctx, sql, args...).Scan(&v); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return v
}

func expectCode(t *testing.T, err error, code, what string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != code {
		t.Fatalf("%s: err = %v, want SQLSTATE %s", what, err, code)
	}
}

// TestAvatarAssetMigrationBackfillsLegacyAvatarsAndReverts runs 000036 on a database holding legacy avatars (one
// key shared by two of them, like the seeded demo data) and checks the backfill, the schema contract and the rollback.
func TestAvatarAssetMigrationBackfillsLegacyAvatarsAndReverts(t *testing.T) {
	ctx := context.Background()
	pool := scratchDatabase(t, ctx)
	owner, other := uuid.New(), uuid.New()
	for _, u := range []uuid.UUID{owner, other} {
		if _, err := pool.Exec(ctx, `INSERT INTO public.users (id, name, email, role, email_verified) VALUES ($1, 'legacy', $2, 'candidate', true)`, u, fmt.Sprintf("%s@example.test", u)); err != nil {
			t.Fatal(err)
		}
	}
	invOwner, invOther := uuid.New(), uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO public.inventories (id, user_id, slot) VALUES ($1, $2, 5), ($3, $4, 5)`, invOwner, owner, invOther, other); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.avatars (inventory_id, name, model_path, personality) VALUES
		($1, 'A', 'seed/shared.vrm', 'p'), ($1, 'B', 'seed/shared.vrm', 'p'), ($1, 'C', 'private/c.vrm', 'p'), ($2, 'D', 'private/d.vrm', 'p')`, invOwner, invOther); err != nil {
		t.Fatal(err)
	}
	before := scalar[string](t, ctx, pool, `SELECT string_agg(name || ':' || model_path, ',' ORDER BY name) FROM public.avatars`)

	applyFile(t, ctx, pool, "../../../../migrations/000036_avatar_assets.up.sql")

	// Backfill: one asset per avatar, creator = inventory owner, key copied, nothing merged by path.
	if n := scalar[int](t, ctx, pool, `SELECT count(*) FROM public.avatar_assets`); n != 4 {
		t.Fatalf("assets = %d, want one per avatar (4)", n)
	}
	if n := scalar[int](t, ctx, pool, `SELECT count(DISTINCT asset_id) FROM public.avatars`); n != 4 {
		t.Fatalf("distinct asset ids = %d, want 4", n)
	}
	if got := scalar[string](t, ctx, pool, `SELECT string_agg(a.name || ':' || s.model_path, ',' ORDER BY a.name) FROM public.avatars a JOIN public.avatar_assets s ON s.id = a.asset_id
		JOIN public.inventories i ON i.id = a.inventory_id WHERE s.created_by = i.user_id AND s.status = 'ready' AND s.created_at = a.created_at`); got != before {
		t.Fatalf("avatars through their assets = %s, want %s", got, before)
	}
	if nullable := scalar[string](t, ctx, pool, `SELECT is_nullable FROM information_schema.columns WHERE table_name = 'avatars' AND column_name = 'asset_id'`); nullable != "NO" {
		t.Fatal("avatars.asset_id must be NOT NULL")
	}
	if n := scalar[int](t, ctx, pool, `SELECT count(*) FROM information_schema.columns WHERE table_name = 'avatars' AND column_name = 'model_path'`); n != 0 {
		t.Fatal("avatars.model_path must be gone")
	}

	// Indexes of the contract.
	for _, idx := range []string{"avatar_listings_asset_key", "avatar_listings_catalog_newest_idx", "avatar_listings_catalog_price_idx",
		"avatar_listings_seller_idx", "avatar_listings_status_idx", "avatars_asset_id_idx", "avatar_assets_model_path_idx"} {
		if scalar[int](t, ctx, pool, `SELECT count(*) FROM pg_indexes WHERE indexname = $1`, idx) != 1 {
			t.Fatalf("index %s is missing", idx)
		}
	}
	// No Marketplace column on transactions, no purchase table, no preview_path/source.
	if n := scalar[int](t, ctx, pool, `SELECT count(*) FROM information_schema.columns WHERE table_name = 'transactions'
		AND column_name IN ('purpose', 'access_status', 'avatar_listing_id')`); n != 0 {
		t.Fatalf("transactions gained %d Marketplace columns", n)
	}
	if n := scalar[int](t, ctx, pool, `SELECT count(*) FROM information_schema.columns WHERE table_name IN ('avatar_assets', 'avatar_listings')
		AND column_name IN ('preview_path', 'source', 'purchase_transaction_id')`); n != 0 {
		t.Fatalf("out-of-scope columns exist: %d", n)
	}
	if scalar[*string](t, ctx, pool, `SELECT to_regclass('public.avatar_purchases')::text`) != nil {
		t.Fatal("a purchase table exists")
	}

	// Listing constraints.
	asset := scalar[uuid.UUID](t, ctx, pool, `SELECT asset_id FROM public.avatars WHERE name = 'C'`)
	insert := func(price int, status, title string, assetID uuid.UUID) error {
		_, err := pool.Exec(ctx, `INSERT INTO public.avatar_listings (seller_id, asset_id, title, default_personality, price_coins, status, rights_note)
			VALUES ($1, $2, $3, 'calm', $4, $5, 'mine')`, owner, assetID, title, price, status)
		return err
	}
	expectCode(t, insert(0, "draft", "Free", asset), "23514", "price 0")
	expectCode(t, insert(-3, "draft", "Negative", asset), "23514", "negative price")
	expectCode(t, insert(1000001, "draft", "Huge", asset), "23514", "price over the bound")
	expectCode(t, insert(5, "sold", "Odd", asset), "23514", "unknown status")
	expectCode(t, insert(5, "draft", "   ", asset), "23514", "blank title")
	expectCode(t, insert(5, "draft", "Ghost", uuid.New()), "23503", "unknown asset")
	if err := insert(5, "draft", "Fine", asset); err != nil {
		t.Fatal(err)
	}
	expectCode(t, insert(7, "draft", "Twice", asset), "23505", "second listing of one asset")
	// An asset in use cannot be deleted from under an avatar or a listing.
	_, err := pool.Exec(ctx, `DELETE FROM public.avatar_assets WHERE id = $1`, asset)
	expectCode(t, err, "23001", "deleting an asset in use (RESTRICT)")

	// Erasing a user: creator anonymised, listings of the seller removed, buyers' rows untouched.
	if _, err := pool.Exec(ctx, `DELETE FROM public.avatars WHERE inventory_id = $1`, invOwner); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM public.inventories WHERE id = $1`, invOwner); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM public.users WHERE id = $1`, owner); err != nil {
		t.Fatalf("a user with listings and assets must stay deletable: %v", err)
	}
	if scalar[int](t, ctx, pool, `SELECT count(*) FROM public.avatar_listings`) != 0 || scalar[int](t, ctx, pool, `SELECT count(*) FROM public.avatar_assets WHERE created_by IS NULL`) != 3 {
		t.Fatal("erasing the seller must drop its listings and anonymise its assets")
	}

	// The rollback gives the surviving avatar its model_path back from the asset, and up again backfills it.
	applyFile(t, ctx, pool, "../../../../migrations/000036_avatar_assets.down.sql")
	if scalar[*string](t, ctx, pool, `SELECT to_regclass('public.avatar_assets')::text`) != nil || scalar[*string](t, ctx, pool, `SELECT to_regclass('public.avatar_listings')::text`) != nil {
		t.Fatal("the rollback left a table behind")
	}
	if got := scalar[string](t, ctx, pool, `SELECT name || ':' || model_path FROM public.avatars`); got != "D:private/d.vrm" {
		t.Fatalf("avatar after the rollback = %s, want D:private/d.vrm", got)
	}
	applyFile(t, ctx, pool, "../../../../migrations/000036_avatar_assets.up.sql")
	if scalar[int](t, ctx, pool, `SELECT count(*) FROM public.avatar_assets`) != 1 {
		t.Fatal("up after down must backfill the surviving avatar again")
	}
}
