-- Avatar Marketplace listings (KAN-363 seller, KAN-365 admin, KAN-367 catalog). Schema: migration 000036.
-- Every state change is one guarded UPDATE: the status and the caller's updated_at are part of the WHERE
-- clause (compare-and-set), so a stale request matches no row and nothing is written.

-- name: LockMarketplaceUser :one
SELECT id FROM public.users
WHERE id = $1 AND coalesce(role, 'candidate') IN ('candidate', 'recruiter')
  AND email_verified AND NOT coalesce(is_locked, false)
FOR SHARE;

-- name: GetOwnedAvatarAsset :one
SELECT a.id AS asset_id, a.created_by, a.status
FROM public.avatars av
JOIN public.inventories inv ON inv.id = av.inventory_id
JOIN public.avatar_assets a ON a.id = av.asset_id
WHERE av.id = $1 AND inv.user_id = $2;

-- name: CreateListing :one
INSERT INTO public.avatar_listings (seller_id, asset_id, title, description, default_personality, price_coins, rights_note)
SELECT sqlc.arg(seller_id)::uuid, a.id, sqlc.arg(title), sqlc.narg(description), sqlc.arg(default_personality), sqlc.arg(price_coins), sqlc.arg(rights_note)
FROM public.avatars av
JOIN public.inventories inv ON inv.id = av.inventory_id
JOIN public.avatar_assets a ON a.id = av.asset_id
WHERE av.id = sqlc.arg(avatar_id) AND inv.user_id = sqlc.arg(seller_id)
  AND a.created_by = sqlc.arg(seller_id) AND a.status = 'ready'
RETURNING id;

-- name: ListEligibleAvatars :many
SELECT av.id AS avatar_id, a.id AS asset_id, av.name
FROM public.avatars av
JOIN public.inventories inv ON inv.id = av.inventory_id
JOIN public.avatar_assets a ON a.id = av.asset_id
WHERE inv.user_id = $1 AND a.created_by = $1 AND a.status = 'ready'
  AND NOT EXISTS (SELECT 1 FROM public.avatar_listings l WHERE l.asset_id = a.id)
ORDER BY av.created_at DESC, av.id;

-- name: GetListing :one
SELECT sqlc.embed(l), a.status AS asset_status, u.name AS seller_name
FROM public.avatar_listings l
JOIN public.avatar_assets a ON a.id = l.asset_id
JOIN public.users u ON u.id = l.seller_id
WHERE l.id = $1;

-- name: ListSellerListings :many
SELECT sqlc.embed(l), a.status AS asset_status, u.name AS seller_name
FROM public.avatar_listings l
JOIN public.avatar_assets a ON a.id = l.asset_id
JOIN public.users u ON u.id = l.seller_id
WHERE l.seller_id = $1 AND (sqlc.narg(status)::text IS NULL OR l.status = sqlc.narg(status)::text)
ORDER BY l.updated_at DESC, l.id
LIMIT $2 OFFSET $3;

-- name: CountSellerListings :one
SELECT count(*) FROM public.avatar_listings l
WHERE l.seller_id = $1 AND (sqlc.narg(status)::text IS NULL OR l.status = sqlc.narg(status)::text);

-- name: UpdateListingPayload :one
UPDATE public.avatar_listings
SET title = sqlc.arg(title), description = sqlc.narg(description), default_personality = sqlc.arg(default_personality),
    price_coins = sqlc.arg(price_coins), rights_note = sqlc.arg(rights_note), updated_at = now()
WHERE id = sqlc.arg(id) AND seller_id = sqlc.arg(seller_id) AND status IN ('draft', 'rejected')
  AND updated_at = sqlc.arg(expected_updated_at)
RETURNING id;

-- A rejected listing may only be resubmitted after an edit: rejection stamps updated_at = reviewed_at, an
-- edit moves updated_at past it. The seller must still own an avatar of the asset and the asset must be ready.
-- name: SubmitListing :one
UPDATE public.avatar_listings l
SET status = 'pending', updated_at = now()
WHERE l.id = sqlc.arg(id) AND l.seller_id = sqlc.arg(seller_id) AND l.status IN ('draft', 'rejected')
  AND l.updated_at = sqlc.arg(expected_updated_at)
  AND (l.status = 'draft' OR l.updated_at > l.reviewed_at)
  AND EXISTS (SELECT 1 FROM public.avatar_assets a WHERE a.id = l.asset_id AND a.status = 'ready' AND a.created_by = l.seller_id)
  AND EXISTS (SELECT 1 FROM public.avatars av JOIN public.inventories inv ON inv.id = av.inventory_id
              WHERE av.asset_id = l.asset_id AND inv.user_id = l.seller_id)
RETURNING id;

-- name: DelistListing :one
UPDATE public.avatar_listings
SET status = 'delisted', updated_at = now()
WHERE id = sqlc.arg(id) AND seller_id = sqlc.arg(seller_id) AND status = 'active' AND updated_at = sqlc.arg(expected_updated_at)
RETURNING id;

-- name: ListAdminListings :many
SELECT sqlc.embed(l), a.status AS asset_status, u.name AS seller_name
FROM public.avatar_listings l
JOIN public.avatar_assets a ON a.id = l.asset_id
JOIN public.users u ON u.id = l.seller_id
WHERE (sqlc.narg(status)::text IS NULL OR l.status = sqlc.narg(status)::text)
ORDER BY l.updated_at, l.id
LIMIT $1 OFFSET $2;

-- name: CountAdminListings :one
SELECT count(*) FROM public.avatar_listings l
WHERE (sqlc.narg(status)::text IS NULL OR l.status = sqlc.narg(status)::text);

-- Review is by exact payload: only the pending listing whose updated_at the Admin loaded can be decided.
-- name: ApproveListing :one
UPDATE public.avatar_listings l
SET status = 'active', reviewed_by = sqlc.arg(admin_id), reviewed_at = now(), review_note = NULL, updated_at = now()
WHERE l.id = sqlc.arg(id) AND l.status = 'pending' AND l.updated_at = sqlc.arg(expected_updated_at)
  AND EXISTS (SELECT 1 FROM public.avatar_assets a WHERE a.id = l.asset_id AND a.status = 'ready')
RETURNING id;

-- name: RejectListing :one
UPDATE public.avatar_listings
SET status = 'rejected', reviewed_by = sqlc.arg(admin_id), reviewed_at = now(), review_note = sqlc.arg(reason), updated_at = now()
WHERE id = sqlc.arg(id) AND status = 'pending' AND updated_at = sqlc.arg(expected_updated_at)
RETURNING id;

-- name: SuspendListing :one
UPDATE public.avatar_listings
SET status = 'suspended', reviewed_by = sqlc.arg(admin_id), reviewed_at = now(), review_note = sqlc.arg(reason), updated_at = now()
WHERE id = sqlc.arg(id) AND status = 'active'
RETURNING id;

-- Catalog: active listings of a ready asset. already_owned reads the caller's inventory rows, never a
-- transaction description.
-- name: ListCatalogNewest :many
SELECT l.id, l.asset_id, l.title, l.description, l.default_personality, l.price_coins, l.created_at, u.name AS seller_name,
       (l.seller_id = sqlc.arg(caller_id)::uuid) AS is_own_listing,
       EXISTS (SELECT 1 FROM public.avatars av JOIN public.inventories inv ON inv.id = av.inventory_id
               WHERE av.asset_id = l.asset_id AND inv.user_id = sqlc.arg(caller_id)::uuid) AS already_owned
FROM public.avatar_listings l
JOIN public.avatar_assets a ON a.id = l.asset_id AND a.status = 'ready'
JOIN public.users u ON u.id = l.seller_id
WHERE l.status = 'active' AND (sqlc.narg(pattern)::text IS NULL OR l.title ILIKE sqlc.narg(pattern)::text ESCAPE '\')
ORDER BY l.created_at DESC, l.id DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: ListCatalogPriceAsc :many
SELECT l.id, l.asset_id, l.title, l.description, l.default_personality, l.price_coins, l.created_at, u.name AS seller_name,
       (l.seller_id = sqlc.arg(caller_id)::uuid) AS is_own_listing,
       EXISTS (SELECT 1 FROM public.avatars av JOIN public.inventories inv ON inv.id = av.inventory_id
               WHERE av.asset_id = l.asset_id AND inv.user_id = sqlc.arg(caller_id)::uuid) AS already_owned
FROM public.avatar_listings l
JOIN public.avatar_assets a ON a.id = l.asset_id AND a.status = 'ready'
JOIN public.users u ON u.id = l.seller_id
WHERE l.status = 'active' AND (sqlc.narg(pattern)::text IS NULL OR l.title ILIKE sqlc.narg(pattern)::text ESCAPE '\')
ORDER BY l.price_coins, l.id
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: ListCatalogPriceDesc :many
SELECT l.id, l.asset_id, l.title, l.description, l.default_personality, l.price_coins, l.created_at, u.name AS seller_name,
       (l.seller_id = sqlc.arg(caller_id)::uuid) AS is_own_listing,
       EXISTS (SELECT 1 FROM public.avatars av JOIN public.inventories inv ON inv.id = av.inventory_id
               WHERE av.asset_id = l.asset_id AND inv.user_id = sqlc.arg(caller_id)::uuid) AS already_owned
FROM public.avatar_listings l
JOIN public.avatar_assets a ON a.id = l.asset_id AND a.status = 'ready'
JOIN public.users u ON u.id = l.seller_id
WHERE l.status = 'active' AND (sqlc.narg(pattern)::text IS NULL OR l.title ILIKE sqlc.narg(pattern)::text ESCAPE '\')
ORDER BY l.price_coins DESC, l.id DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountCatalog :one
SELECT count(*) FROM public.avatar_listings l
JOIN public.avatar_assets a ON a.id = l.asset_id AND a.status = 'ready'
WHERE l.status = 'active' AND (sqlc.narg(pattern)::text IS NULL OR l.title ILIKE sqlc.narg(pattern)::text ESCAPE '\');

-- name: GetCatalogListing :one
SELECT l.id, l.asset_id, l.title, l.description, l.default_personality, l.price_coins, l.created_at, u.name AS seller_name,
       (l.seller_id = sqlc.arg(caller_id)::uuid) AS is_own_listing,
       EXISTS (SELECT 1 FROM public.avatars av JOIN public.inventories inv ON inv.id = av.inventory_id
               WHERE av.asset_id = l.asset_id AND inv.user_id = sqlc.arg(caller_id)::uuid) AS already_owned
FROM public.avatar_listings l
JOIN public.avatar_assets a ON a.id = l.asset_id AND a.status = 'ready'
JOIN public.users u ON u.id = l.seller_id
WHERE l.id = sqlc.arg(id) AND l.status = 'active';
