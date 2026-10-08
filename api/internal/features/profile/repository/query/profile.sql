-- name: GetProfile :one
SELECT id, name, email, image, coalesce(role, 'candidate')::text AS role,
       email_verified, company_name, company_website, created_at, updated_at
FROM public.users
WHERE id = $1;

-- name: UpdateProfile :one
UPDATE public.users SET
    name = CASE WHEN sqlc.arg(set_name)::boolean THEN sqlc.narg(name)::text ELSE name END,
    image = CASE WHEN sqlc.arg(set_image)::boolean THEN sqlc.narg(image)::text ELSE image END,
    company_name = CASE WHEN sqlc.arg(set_company_name)::boolean THEN sqlc.narg(company_name)::text ELSE company_name END,
    company_website = CASE WHEN sqlc.arg(set_company_website)::boolean THEN sqlc.narg(company_website)::text ELSE company_website END,
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id)
    AND coalesce(role, 'candidate') = sqlc.arg(role)::text
    AND NOT coalesce(is_locked, false) AND email_verified
RETURNING id, name, email, image, coalesce(role, 'candidate')::text AS role,
          email_verified, company_name, company_website, created_at, updated_at;

-- name: ProfileExists :one
SELECT EXISTS(SELECT 1 FROM public.users WHERE id = $1);
