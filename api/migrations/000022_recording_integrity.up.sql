-- Recording integrity (KAN-260). Additive only: two columns on interviews.
BEGIN;

-- The ETag of the stored object, taken when the Candidate's upload was verified. A read compares it with
-- the object's current ETag, so an object overwritten after it was marked ready is never served.
-- NULL for a recording that is not ready, and for ready rows written before this migration.
-- record_grant_count counts the upload grants issued for the interview, so a Candidate cannot keep
-- replacing the object indefinitely.
ALTER TABLE public.interviews
    ADD COLUMN record_etag text,
    ADD COLUMN record_grant_count int NOT NULL DEFAULT 0;

COMMIT;
