BEGIN;

-- Provider metadata for voices synced from a TTS provider through the adapter. Legacy rows keep NULLs
-- and stay usable; a row is provider-managed only when provider_voice_id is set.
ALTER TABLE public.voices
    ADD COLUMN provider text,
    ADD COLUMN provider_voice_id text,
    ADD COLUMN locale text,
    ADD COLUMN gender text,
    ADD CONSTRAINT voices_provider_pair_chk CHECK ((provider IS NULL) = (provider_voice_id IS NULL));

-- A provider's voice id identifies one catalog row; the sync upserts on it.
CREATE UNIQUE INDEX voices_provider_voice_uidx ON public.voices (provider, provider_voice_id)
    WHERE provider_voice_id IS NOT NULL;

COMMIT;
