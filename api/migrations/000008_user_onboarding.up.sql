BEGIN;
CREATE TYPE public.onboarding_status AS ENUM ('pending', 'role_selected', 'completed');
ALTER TABLE public.users
    ADD COLUMN onboarding_status public.onboarding_status NOT NULL DEFAULT 'pending';
COMMIT;
