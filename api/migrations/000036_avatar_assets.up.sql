-- Avatar Marketplace asset foundation (KAN-360, AM-01; Avatar-Marketplace-Spec sections 5 and 10).
--
-- Two new tables and one new column, nothing on transactions:
--
--   avatar_assets     one stored model (VRM object key) shared by every avatar row that uses it
--   avatar_listings   a Marketplace listing of one asset (draft -> pending -> active | rejected, delisted, suspended)
--   avatars.asset_id  the avatar's model; the owner stays inventories.user_id, the creator is avatar_assets.created_by
--
-- avatars.model_path moves to avatar_assets.model_path and is dropped here (the data is development/seed data, so
-- there is no staged transition). Each existing avatar gets its own asset (no merge by path: several seeded avatars
-- share a key on purpose), created by the inventory owner. An asset's model_path is never updated; a new model is a
-- new asset id.
--
-- Delete actions:
--   avatars.asset_id            RESTRICT  an asset in use is never deleted from under an avatar
--   avatar_listings.asset_id    RESTRICT  nor from under a listing (cleanup deletes orphans only)
--   avatar_assets.created_by    SET NULL  erasure anonymises the creator; the asset stays while others use it
--   avatar_listings.seller_id   CASCADE   erasing a seller removes the listings, never the buyers' avatars or the asset
--   avatar_listings.reviewed_by SET NULL
BEGIN;

CREATE TABLE public.avatar_assets (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    created_by uuid REFERENCES public.users(id) ON DELETE SET NULL,
    model_path text NOT NULL,
    status text NOT NULL DEFAULT 'ready',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT avatar_assets_model_path_chk CHECK (btrim(model_path) <> ''),
    CONSTRAINT avatar_assets_status_chk CHECK (status IN ('ready', 'blocked'))
);
-- model_path is not unique: legacy rows may share a key. Object cleanup compares by path.
CREATE INDEX avatar_assets_model_path_idx ON public.avatar_assets(model_path);
CREATE INDEX avatar_assets_created_by_idx ON public.avatar_assets(created_by) WHERE created_by IS NOT NULL;

CREATE TABLE public.avatar_listings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    seller_id uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    asset_id uuid NOT NULL REFERENCES public.avatar_assets(id) ON DELETE RESTRICT,
    title text NOT NULL,
    description text,
    default_personality text NOT NULL,
    price_coins integer NOT NULL,
    status text NOT NULL DEFAULT 'draft',
    rights_note text NOT NULL,
    reviewed_by uuid REFERENCES public.users(id) ON DELETE SET NULL,
    reviewed_at timestamptz,
    review_note text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    -- One listing per asset in the MVP: the same model is not sold twice.
    CONSTRAINT avatar_listings_asset_key UNIQUE (asset_id),
    CONSTRAINT avatar_listings_status_chk CHECK (status IN ('draft', 'pending', 'active', 'rejected', 'delisted', 'suspended')),
    -- Wallet balances and transaction amounts are 32-bit integers; one million coins is a generous ceiling.
    CONSTRAINT avatar_listings_price_chk CHECK (price_coins > 0 AND price_coins <= 1000000),
    CONSTRAINT avatar_listings_title_chk CHECK (btrim(title) <> '' AND char_length(title) <= 100),
    CONSTRAINT avatar_listings_description_chk CHECK (description IS NULL OR char_length(description) <= 2000),
    CONSTRAINT avatar_listings_personality_chk CHECK (btrim(default_personality) <> '' AND char_length(default_personality) <= 2000),
    CONSTRAINT avatar_listings_rights_note_chk CHECK (btrim(rights_note) <> '' AND char_length(rights_note) <= 2000),
    CONSTRAINT avatar_listings_review_note_chk CHECK (review_note IS NULL OR char_length(review_note) <= 2000)
);
-- Catalog (active only): newest first and cheapest first; id breaks ties so keyset pagination is stable.
CREATE INDEX avatar_listings_catalog_newest_idx ON public.avatar_listings(created_at DESC, id DESC) WHERE status = 'active';
CREATE INDEX avatar_listings_catalog_price_idx ON public.avatar_listings(price_coins, id) WHERE status = 'active';
-- A seller's own listings, and the admin queue by status (oldest change first).
CREATE INDEX avatar_listings_seller_idx ON public.avatar_listings(seller_id, updated_at DESC, id);
CREATE INDEX avatar_listings_status_idx ON public.avatar_listings(status, updated_at, id);
CREATE INDEX avatar_listings_reviewed_by_idx ON public.avatar_listings(reviewed_by) WHERE reviewed_by IS NOT NULL;

-- Backfill: one asset per avatar, created by the inventory owner and dated like the avatar.
ALTER TABLE public.avatars ADD COLUMN asset_id uuid;
WITH src AS (
    SELECT a.id AS avatar_id, gen_random_uuid() AS asset_id, inv.user_id, a.model_path, a.created_at
    FROM public.avatars a
    JOIN public.inventories inv ON inv.id = a.inventory_id
), ins AS (
    INSERT INTO public.avatar_assets (id, created_by, model_path, status, created_at, updated_at)
    SELECT asset_id, user_id, model_path, 'ready', created_at, created_at FROM src
    RETURNING id
)
UPDATE public.avatars a SET asset_id = src.asset_id
FROM src
WHERE a.id = src.avatar_id AND EXISTS (SELECT 1 FROM ins WHERE ins.id = src.asset_id);

ALTER TABLE public.avatars
    ALTER COLUMN asset_id SET NOT NULL,
    ADD CONSTRAINT avatars_asset_id_fkey FOREIGN KEY (asset_id) REFERENCES public.avatar_assets(id) ON DELETE RESTRICT,
    DROP COLUMN model_path;
CREATE INDEX avatars_asset_id_idx ON public.avatars(asset_id);

COMMIT;
