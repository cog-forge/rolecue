BEGIN;
ALTER TABLE public.users DROP COLUMN onboarding_status;
DROP TYPE public.onboarding_status;
COMMIT;
