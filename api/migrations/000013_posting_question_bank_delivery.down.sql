BEGIN;
ALTER TABLE public.job_postings DROP COLUMN review_submitted;
COMMIT;
