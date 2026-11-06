BEGIN;

ALTER TABLE public.job_postings DROP COLUMN IF EXISTS erased_interviews;

COMMIT;
