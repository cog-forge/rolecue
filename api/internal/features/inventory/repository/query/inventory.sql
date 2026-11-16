-- name: LockInventoryOwner :one
SELECT id FROM public.users
WHERE id = $1 AND coalesce(role, 'candidate') IN ('candidate', 'recruiter')
  AND email_verified AND NOT coalesce(is_locked, false)
FOR SHARE;

-- name: EnsureInventory :exec
INSERT INTO public.inventories (user_id, slot) VALUES ($1, sqlc.arg(initial_capacity)::integer)
ON CONFLICT (user_id) DO NOTHING;

-- name: LockAvatarInventory :one
SELECT id FROM public.inventories WHERE user_id = $1 FOR UPDATE;

-- The shared model of a new avatar: one asset per conversion, created by the owner. model_path is the object key
-- the upload returned; the asset's payload never changes afterwards.
-- name: InsertAvatarAsset :one
INSERT INTO public.avatar_assets (created_by, model_path) VALUES (sqlc.arg(created_by)::uuid, sqlc.arg(model_path)::text)
RETURNING id;

-- The avatar points at its asset. Zero rows means the inventory is full (the caller rolls the asset back).
-- name: InsertInventoryAvatar :one
INSERT INTO public.avatars (inventory_id, name, asset_id, personality)
SELECT i.id, sqlc.arg(name)::text, sqlc.arg(asset_id)::uuid, sqlc.arg(personality)::text
FROM public.inventories i
WHERE i.user_id = sqlc.arg(user_id)
  AND (SELECT count(*) FROM public.avatars a WHERE a.inventory_id = i.id) < i.slot
RETURNING id, name, personality, created_at, updated_at;

-- name: GetInventoryCapacity :one
SELECT i.slot, count(a.id)::bigint AS used
FROM public.inventories i LEFT JOIN public.avatars a ON a.inventory_id = i.id
WHERE i.user_id = $1
GROUP BY i.id;

-- name: CountInventoryAvatars :one
SELECT count(*)::bigint FROM public.avatars a
JOIN public.inventories i ON i.id = a.inventory_id
WHERE i.user_id = $1;

-- Avatar models the owner created since a point in time: the persisted side of the daily conversion quota.
-- Counted from avatar_assets.created_by, so an avatar bought on the Marketplace (someone else's asset) does not use
-- the quota and a model the owner converted keeps counting while it exists.
-- name: CountRecentInventoryAvatars :one
SELECT count(*)::bigint FROM public.avatar_assets s
WHERE s.created_by = sqlc.arg(owner_id)::uuid AND s.created_at >= sqlc.arg(since)::timestamptz;

-- name: ListInventoryAvatars :many
SELECT a.id, a.name, a.personality, a.created_at, a.updated_at
FROM public.avatars a JOIN public.inventories i ON i.id = a.inventory_id
WHERE i.user_id = $1
ORDER BY a.created_at DESC, a.id DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- The model key comes from the asset. Access is decided by inventory ownership alone: the creator of the asset,
-- its listings and any transaction are irrelevant.
-- name: GetInventoryAvatar :one
SELECT a.id, a.name, a.personality, s.model_path, a.created_at, a.updated_at
FROM public.avatars a
JOIN public.inventories i ON i.id = a.inventory_id
JOIN public.avatar_assets s ON s.id = a.asset_id
WHERE i.user_id = sqlc.arg(owner_id) AND a.id = sqlc.arg(avatar_id);

-- Only the owner's avatar row goes; the asset it pointed at is returned so the caller can clean it up when nothing
-- else uses it.
-- name: DeleteInventoryAvatar :one
DELETE FROM public.avatars a USING public.inventories i
WHERE a.inventory_id = i.id AND i.user_id = sqlc.arg(owner_id)
  AND a.id = sqlc.arg(avatar_id)
RETURNING a.asset_id;

-- Row lock on the asset: a concurrent grant or listing of it holds a key-share lock, so this waits for it and the
-- reference check below sees its row.
-- name: LockAvatarAsset :one
SELECT id FROM public.avatar_assets WHERE id = $1 FOR UPDATE;

-- Deletes the asset only when no avatar and no listing uses it; the stored key comes back so the service can
-- decide about the object. Zero rows: the asset is still in use (or already gone).
-- name: DeleteUnreferencedAsset :one
DELETE FROM public.avatar_assets s
WHERE s.id = $1
  AND NOT EXISTS (SELECT 1 FROM public.avatars a WHERE a.asset_id = s.id)
  AND NOT EXISTS (SELECT 1 FROM public.avatar_listings l WHERE l.asset_id = s.id)
RETURNING s.model_path;

-- Whether any asset still stores this object key. Legacy rows may share one key, so the object is deleted only
-- when this is false.
-- name: ModelPathInUse :one
SELECT EXISTS (SELECT 1 FROM public.avatar_assets WHERE model_path = $1);

-- Zero rows means the purchase would pass max_slots.
-- name: UpdateInventorySlot :execrows
UPDATE public.inventories
SET slot = slot + @amount
WHERE user_id = @user_id AND slot + @amount <= @max_slots::integer;

-- The ledger row of an earlier slot purchase with this Idempotency-Key, read under the inventory row lock. Only
-- rows paid to the system match: a Marketplace purchase row under the same key text is never a slot purchase.
-- name: GetSlotPurchaseByKey :one
SELECT amount FROM public.transactions
WHERE "from" = @payer AND idempotency_key = @idempotency_key AND "to" = 'system';
