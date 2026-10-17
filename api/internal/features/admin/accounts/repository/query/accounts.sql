-- name: ListManagedAccounts :one
WITH matching AS (
 SELECT id, name, email, image, coalesce(role, 'candidate')::text AS role,
   email_verified, coalesce(is_locked, false)::boolean AS is_locked,
   lock_reason, lock_expires_at, created_at, updated_at
 FROM public.users
 WHERE (sqlc.arg(search)::text = '' OR name ILIKE sqlc.arg(pattern)::text OR email ILIKE sqlc.arg(pattern)::text)
   AND (sqlc.arg(role_filter)::text = '' OR role = sqlc.arg(role_filter)::text)
   AND (NOT sqlc.arg(locked_only)::boolean OR
     (is_locked = true AND (lock_expires_at IS NULL OR lock_expires_at > sqlc.arg(now)::timestamptz)))
), page AS (
 SELECT * FROM matching ORDER BY created_at DESC, id DESC LIMIT sqlc.arg(page_limit)::int OFFSET sqlc.arg(page_offset)::bigint
)
SELECT (SELECT count(*) FROM matching) AS total,
 coalesce((SELECT jsonb_agg(to_jsonb(page) ORDER BY created_at DESC, id DESC) FROM page), '[]'::jsonb)::jsonb AS users;

-- name: GetManagedAccount :one
SELECT to_jsonb(account) FROM (
 SELECT id, name, email, image, coalesce(role, 'candidate')::text AS role,
   email_verified, coalesce(is_locked, false)::boolean AS is_locked,
   lock_reason, lock_expires_at, created_at, updated_at
 FROM public.users WHERE id = @id
) AS account;

-- name: AcquireAccountRequestLock :exec
SELECT pg_advisory_xact_lock(hashtextextended(sqlc.arg(user_id)::text, 0));
