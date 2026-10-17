-- name: ListActiveVoices :many
SELECT id, name
FROM public.voices
WHERE is_active
ORDER BY lower(name), id
LIMIT @page_limit OFFSET @page_offset;

-- name: CountActiveVoices :one
SELECT count(*) FROM public.voices WHERE is_active;
