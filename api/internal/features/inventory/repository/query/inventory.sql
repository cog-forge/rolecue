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

-- name: InsertInventoryAvatar :one
INSERT INTO public.avatars (inventory_id, name, model_path, personality)
SELECT i.id, sqlc.arg(name)::text, sqlc.arg(model_path)::text, sqlc.arg(personality)::text
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

-- name: ListInventoryAvatars :many
SELECT a.id, a.name, a.personality, a.created_at, a.updated_at
FROM public.avatars a JOIN public.inventories i ON i.id = a.inventory_id
WHERE i.user_id = $1
ORDER BY a.created_at DESC, a.id DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: GetInventoryAvatar :one
SELECT a.id, a.name, a.personality, a.model_path, a.created_at, a.updated_at
FROM public.avatars a JOIN public.inventories i ON i.id = a.inventory_id
WHERE i.user_id = sqlc.arg(owner_id) AND a.id = sqlc.arg(avatar_id);

-- name: DeleteInventoryAvatar :execrows
DELETE FROM public.avatars a USING public.inventories i
WHERE a.inventory_id = i.id AND i.user_id = sqlc.arg(owner_id)
  AND a.id = sqlc.arg(avatar_id);

-- name: UpdateInventorySlot :exec
UPDATE public.inventories
SET slot = slot + @amount
WHERE user_id = @user_id;
