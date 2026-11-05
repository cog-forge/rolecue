BEGIN;

ALTER TABLE public.applications DROP COLUMN decided_at;

COMMIT;
