BEGIN;
DROP INDEX public.voices_provider_voice_uidx;
ALTER TABLE public.voices
    DROP CONSTRAINT voices_provider_pair_chk,
    DROP COLUMN gender,
    DROP COLUMN locale,
    DROP COLUMN provider_voice_id,
    DROP COLUMN provider;
COMMIT;
