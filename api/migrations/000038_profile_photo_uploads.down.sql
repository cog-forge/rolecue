BEGIN;

UPDATE public.users SET image = NULL
WHERE profile_photo_key IS NOT NULL
    AND image = '/profile/photos/' || id::text || '/' || profile_photo_key::text;
ALTER TABLE public.users DROP COLUMN profile_photo_key;

DROP INDEX public.upload_grants_profile_photo_created_at_idx;
DELETE FROM public.upload_grants WHERE purpose = 'profile_photo';

ALTER TABLE public.upload_grants DROP CONSTRAINT upload_grants_purpose_check;
ALTER TABLE public.upload_grants ADD CONSTRAINT upload_grants_purpose_check
    CHECK (purpose IN ('role_profile_pdf', 'job_posting_pdf'));

COMMIT;
