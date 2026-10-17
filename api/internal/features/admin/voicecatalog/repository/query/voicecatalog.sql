-- Both filters are optional: a NULL argument disables it. name_pattern is an ILIKE pattern the
-- repository builds with %, _ and \ escaped.
-- name: ListVoices :many
SELECT v.id, v.name, v.is_active,
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
SELECT v.id, v.name, v.is_active,
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
