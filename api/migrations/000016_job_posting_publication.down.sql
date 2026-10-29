BEGIN;
DROP INDEX public.job_postings_live_publication_idx;
ALTER TABLE public.job_postings DROP COLUMN published_at;
COMMIT;
