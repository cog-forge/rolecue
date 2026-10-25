-- Interview session state the frozen interviews table could not hold: when a session started and
-- completed, whether its evaluation is pending, finished or failed, and the state of a recruitment
-- recording. Additive only: no new table, and existing rows take the defaults.
BEGIN;

ALTER TABLE public.interviews
    ADD COLUMN started_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ADD COLUMN completed_at timestamptz,
    ADD COLUMN evaluation_status text NOT NULL DEFAULT 'pending'
        CHECK (evaluation_status IN ('pending', 'completed', 'failed')),
    ADD COLUMN record_status text NOT NULL DEFAULT 'none'
        CHECK (record_status IN ('none', 'uploading', 'processing', 'ready', 'failed'));

CREATE INDEX interviews_started_at_idx ON public.interviews(started_at);

COMMIT;
