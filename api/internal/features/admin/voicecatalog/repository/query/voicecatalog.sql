-- Both filters are optional: a NULL argument disables it. name_pattern is an ILIKE pattern the
-- repository builds with %, _ and \ escaped.
-- name: ListVoices :many
SELECT v.id, v.name, v.is_active, v.provider, v.provider_voice_id, v.locale, v.gender,
       (SELECT count(*) FROM public.interviews i WHERE i.voice_id = v.id) AS interview_count,
       (SELECT count(*) FROM public.job_postings jp WHERE jp.required_voice_id = v.id) AS job_posting_count
FROM public.voices v
WHERE (sqlc.narg(is_active)::boolean IS NULL OR v.is_active = sqlc.narg(is_active)::boolean)
  AND (sqlc.narg(name_pattern)::text IS NULL OR v.name ILIKE sqlc.narg(name_pattern)::text ESCAPE '\')
ORDER BY lower(v.name), v.id
LIMIT @page_limit OFFSET @page_offset;

-- name: CountVoices :one
SELECT count(*)
FROM public.voices v
WHERE (sqlc.narg(is_active)::boolean IS NULL OR v.is_active = sqlc.narg(is_active)::boolean)
  AND (sqlc.narg(name_pattern)::text IS NULL OR v.name ILIKE sqlc.narg(name_pattern)::text ESCAPE '\');

-- name: GetVoice :one
SELECT v.id, v.name, v.is_active, v.provider, v.provider_voice_id, v.locale, v.gender,
       (SELECT count(*) FROM public.interviews i WHERE i.voice_id = v.id) AS interview_count,
       (SELECT count(*) FROM public.job_postings jp WHERE jp.required_voice_id = v.id) AS job_posting_count
FROM public.voices v
WHERE v.id = @id;

-- FOR UPDATE conflicts with the FOR KEY SHARE lock that inserting a referencing interview or
-- posting takes, so no new reference can appear between counting and deleting.
-- name: LockVoice :one
SELECT id FROM public.voices WHERE id = @id FOR UPDATE;

-- name: CountVoiceReferences :one
SELECT (SELECT count(*) FROM public.interviews WHERE voice_id = @id::uuid)
     + (SELECT count(*) FROM public.job_postings WHERE required_voice_id = @id::uuid) AS total;

-- name: DeactivateVoice :exec
UPDATE public.voices SET is_active = false WHERE id = @id;

-- name: DeleteVoice :exec
DELETE FROM public.voices WHERE id = @id;

-- Reactivating takes the same row lock as Remove, so it waits for a removal in progress and then
-- finds no row if the voice was deleted.
-- name: ActivateVoice :execrows
UPDATE public.voices SET is_active = true WHERE id = @id;

-- Serializes concurrent syncs of one provider for the length of the transaction.
-- name: LockProviderSync :exec
SELECT pg_advisory_xact_lock(hashtextextended('voice_sync:' || @provider::text, 0));

-- One statement upserts the whole fetched list, passed as a JSON array. A row that already matches is skipped by the WHERE,
-- so it is not returned and counts as unchanged. An update never touches is_active: an Admin's
-- deactivation survives a sync. xmax = 0 is true only for a freshly inserted row.
-- name: UpsertProviderVoices :many
INSERT INTO public.voices (name, provider, provider_voice_id, locale, gender)
SELECT t.value->>'name', @provider::text, t.value->>'provider_voice_id', t.value->>'locale', NULLIF(t.value->>'gender', '')
FROM jsonb_array_elements(@voices::jsonb) AS t
ON CONFLICT (provider, provider_voice_id) WHERE provider_voice_id IS NOT NULL DO UPDATE
    SET name = EXCLUDED.name, locale = EXCLUDED.locale, gender = EXCLUDED.gender
    WHERE (voices.name, voices.locale, voices.gender) IS DISTINCT FROM (EXCLUDED.name, EXCLUDED.locale, EXCLUDED.gender)
RETURNING (xmax = 0)::boolean AS inserted;

-- Voices the provider stopped offering are only deactivated; interviews and postings keep them.
-- Legacy rows (no provider) never match.
-- name: DeactivateMissingProviderVoices :execrows
UPDATE public.voices SET is_active = false
WHERE provider = @provider::text AND provider_voice_id IS NOT NULL AND is_active
  AND provider_voice_id <> ALL(@provider_voice_ids::text[]);
