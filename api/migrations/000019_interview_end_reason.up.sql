-- Session lifecycle (KAN-215, KAN-180, KAN-272). Additive only: no new table.
BEGIN;

-- How a session ended, set by the one end path every ending goes through: the last answer stored
-- (completed), the Candidate leaving early (left), or the System Handler ending an idle session
-- (abandoned). Sessions ended before this migration keep NULL; an unfinished session has NULL.
ALTER TABLE public.interviews
    ADD COLUMN end_reason text,
    ADD CONSTRAINT interviews_end_reason_check
        CHECK (end_reason IS NULL OR end_reason IN ('completed', 'left', 'abandoned'));

-- A recording purged after the retention period keeps an auditable status: record_path is cleared and
-- the status says the object was deleted on purpose, not that it never arrived.
ALTER TABLE public.interviews DROP CONSTRAINT interviews_record_status_check;
ALTER TABLE public.interviews
    ADD CONSTRAINT interviews_record_status_check
        CHECK (record_status IN ('none', 'uploading', 'processing', 'ready', 'failed', 'deleted'));

COMMIT;
