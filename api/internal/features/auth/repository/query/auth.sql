-- name: GetAuthUser :one
SELECT id, email, name, coalesce(role, 'candidate')::text AS role,
       email_verified, coalesce(is_locked, false)::boolean AS is_locked, image,
       (coalesce(role, 'candidate') = 'admin' OR onboarding_status IN ('role_selected', 'completed'))::boolean AS onboarding_role_selected,
       (coalesce(role, 'candidate') = 'admin' OR onboarding_status = 'completed')::boolean AS onboarding_completed
FROM public.users
WHERE id = $1;

-- name: ChooseOnboardingRole :one
UPDATE public.users SET role = sqlc.arg(chosen_role)::text,
    onboarding_status = 'role_selected',
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id) AND coalesce(role, 'candidate') <> 'admin'
    AND NOT coalesce(is_locked, false) AND email_verified
    AND (onboarding_status = 'pending' OR (
        onboarding_status = 'role_selected' AND role = sqlc.arg(chosen_role)::text
    ))
RETURNING id, email, name, coalesce(role, 'candidate')::text AS role,
    email_verified, coalesce(is_locked, false)::boolean AS is_locked, image,
    (coalesce(role, 'candidate') = 'admin' OR onboarding_status IN ('role_selected', 'completed'))::boolean AS onboarding_role_selected,
    (coalesce(role, 'candidate') = 'admin' OR onboarding_status = 'completed')::boolean AS onboarding_completed;
