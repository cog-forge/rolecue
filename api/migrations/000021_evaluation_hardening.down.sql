BEGIN;

ALTER TABLE public.interviews
    DROP COLUMN IF EXISTS evaluation_attempts,
    DROP COLUMN IF EXISTS injection_flagged;

ALTER TABLE public.score_details
    DROP COLUMN IF EXISTS evidence;

COMMIT;
