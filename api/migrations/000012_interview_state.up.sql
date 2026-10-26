-- Interview session state the frozen interviews table could not hold: when a session started and
-- completed, whether its evaluation is pending, finished or failed (and when a run last claimed it), and the state of a recruitment
-- recording. Additive only: no new table, and existing rows take the defaults.
BEGIN;

ALTER TABLE public.interviews
    ADD COLUMN started_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ADD COLUMN completed_at timestamptz,
    ADD COLUMN evaluation_status text NOT NULL DEFAULT 'pending'
        CHECK (evaluation_status IN ('pending', 'completed', 'failed')),
    ADD COLUMN evaluation_claimed_at timestamptz,
    ADD COLUMN record_status text NOT NULL DEFAULT 'none'
        CHECK (record_status IN ('none', 'uploading', 'processing', 'ready', 'failed'));

CREATE INDEX interviews_started_at_idx ON public.interviews(started_at);

-- A practice start may carry a client idempotency key (scoped to the Candidate by the service), so a
-- retried request returns the session it created instead of charging again.
ALTER TABLE public.interviews ADD COLUMN start_key text;
CREATE UNIQUE INDEX interviews_start_key_key ON public.interviews(start_key) WHERE start_key IS NOT NULL;

-- A recruitment session belongs to an Application and the Recruiter's posting, not to a practice Role
-- Profile, so it stores no role profile; ownership then comes from applications.candidate_id.
ALTER TABLE public.interviews ALTER COLUMN role_profile_id DROP NOT NULL;
ALTER TABLE public.interviews
    ADD CONSTRAINT interviews_practice_has_role_profile
    CHECK (type <> 'practice' OR role_profile_id IS NOT NULL);

COMMIT;
