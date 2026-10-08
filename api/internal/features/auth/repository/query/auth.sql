-- name: GetAuthUser :one
SELECT id, email, name, coalesce(role, 'candidate')::text AS role,
       email_verified, coalesce(is_locked, false)::boolean AS is_locked, image
FROM public.users
WHERE id = $1;
