-- Recording consent (KAN-297). The Candidate ticks a versioned consent text before the recording starts;
-- the API stores which version and when, with the upload grant. Both columns are NULL for interviews that
-- never recorded a consent. Additive only.
BEGIN;

ALTER TABLE public.interviews
    ADD COLUMN recording_consent_version text,
    ADD COLUMN recording_consented_at timestamptz,
    ADD CONSTRAINT interviews_recording_consent_pair_chk
        CHECK ((recording_consent_version IS NULL) = (recording_consented_at IS NULL));

COMMIT;
