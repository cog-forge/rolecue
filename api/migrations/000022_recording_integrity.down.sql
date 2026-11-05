BEGIN;

ALTER TABLE public.interviews
    DROP COLUMN IF EXISTS record_grant_count,
    DROP COLUMN IF EXISTS record_etag;

COMMIT;
