-- name: HasPhotoUpload :one
SELECT EXISTS (
    SELECT 1 FROM public.upload_grants
    WHERE key = @key AND owner_id = @owner_id AND purpose = 'profile_photo'
      AND consumed_at IS NULL AND created_at > @issued_after
);

-- name: ConsumePhotoUpload :execrows
UPDATE public.upload_grants SET consumed_at = CURRENT_TIMESTAMP
WHERE key = @key AND owner_id = @owner_id AND purpose = 'profile_photo'
  AND consumed_at IS NULL AND created_at > @issued_after;

-- name: ListExpiredPhotoUploads :many
SELECT key FROM public.upload_grants
WHERE purpose = 'profile_photo' AND created_at < @issued_before
ORDER BY created_at LIMIT 8;

-- name: RemovePhotoUpload :exec
DELETE FROM public.upload_grants WHERE key = @key AND purpose = 'profile_photo';

-- name: LockProfilePhoto :one
SELECT profile_photo_key FROM public.users WHERE id = @id FOR UPDATE;

-- name: OwnsProfilePhoto :one
SELECT EXISTS (
    SELECT 1 FROM public.users WHERE id = @owner_id AND profile_photo_key = @key::uuid
);
