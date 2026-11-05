BEGIN;

ALTER TABLE public.interviews
    DROP CONSTRAINT IF EXISTS interviews_recording_consent_pair_chk,
    DROP COLUMN IF EXISTS recording_consented_at,
    DROP COLUMN IF EXISTS recording_consent_version;

COMMIT;
