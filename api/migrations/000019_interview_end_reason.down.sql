BEGIN;

-- The old CHECK has no deleted status; a purged recording has no object either way, so it reads as
-- failed after the rollback.
UPDATE public.interviews SET record_status = 'failed' WHERE record_status = 'deleted';
ALTER TABLE public.interviews DROP CONSTRAINT interviews_record_status_check;
ALTER TABLE public.interviews
    ADD CONSTRAINT interviews_record_status_check
        CHECK (record_status IN ('none', 'uploading', 'processing', 'ready', 'failed'));

ALTER TABLE public.interviews DROP CONSTRAINT IF EXISTS interviews_end_reason_check;
ALTER TABLE public.interviews DROP COLUMN IF EXISTS end_reason;

COMMIT;
