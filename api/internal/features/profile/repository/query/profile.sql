-- name: GetProfile :one
SELECT id, name, email, image, coalesce(role, 'candidate')::text AS role,
       email_verified, company_name, company_website, created_at, updated_at,
       (coalesce(role, 'candidate') = 'admin' OR onboarding_status IN ('role_selected', 'completed'))::boolean AS onboarding_role_selected,
       (coalesce(role, 'candidate') = 'admin' OR onboarding_status = 'completed')::boolean AS onboarding_completed
FROM public.users
WHERE id = $1;

-- name: UpdateProfile :one
UPDATE public.users SET
    name = CASE WHEN sqlc.arg(set_name)::boolean THEN sqlc.narg(name)::text ELSE name END,
    image = CASE WHEN sqlc.arg(set_image)::boolean THEN sqlc.narg(image)::text ELSE image END,
    company_name = CASE WHEN sqlc.arg(set_company_name)::boolean THEN sqlc.narg(company_name)::text ELSE company_name END,
    company_website = CASE WHEN sqlc.arg(set_company_website)::boolean THEN sqlc.narg(company_website)::text ELSE company_website END,
    onboarding_status = CASE WHEN role = 'admin' THEN onboarding_status ELSE 'completed'::public.onboarding_status END,
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id)
    AND coalesce(role, 'candidate') = sqlc.arg(role)::text
    AND NOT coalesce(is_locked, false) AND email_verified
    AND (coalesce(role, 'candidate') = 'admin' OR onboarding_status = 'completed' OR (
        onboarding_status = 'role_selected' AND sqlc.arg(set_name)::boolean
        AND (role = 'candidate' OR (sqlc.arg(set_company_name)::boolean AND sqlc.arg(set_company_website)::boolean))
    ))
RETURNING id, name, email, image, coalesce(role, 'candidate')::text AS role,
          email_verified, company_name, company_website, created_at, updated_at,
       (coalesce(role, 'candidate') = 'admin' OR onboarding_status IN ('role_selected', 'completed'))::boolean AS onboarding_role_selected,
       (coalesce(role, 'candidate') = 'admin' OR onboarding_status = 'completed')::boolean AS onboarding_completed;

-- name: ProfileExists :one
SELECT EXISTS(SELECT 1 FROM public.users WHERE id = $1);
