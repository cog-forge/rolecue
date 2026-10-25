BEGIN;

DROP INDEX IF EXISTS public.interviews_started_at_idx;
ALTER TABLE public.interviews
    DROP COLUMN IF EXISTS record_status,
    DROP COLUMN IF EXISTS evaluation_claimed_at,
    DROP COLUMN IF EXISTS evaluation_status,
    DROP COLUMN IF EXISTS completed_at,
    DROP COLUMN IF EXISTS started_at;

COMMIT;
