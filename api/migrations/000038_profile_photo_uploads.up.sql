BEGIN;

ALTER TABLE public.users ADD COLUMN profile_photo_key uuid;

ALTER TABLE public.upload_grants DROP CONSTRAINT upload_grants_purpose_check;
ALTER TABLE public.upload_grants ADD CONSTRAINT upload_grants_purpose_check
    CHECK (purpose IN ('role_profile_pdf', 'job_posting_pdf', 'profile_photo'));

CREATE INDEX upload_grants_profile_photo_created_at_idx ON public.upload_grants (created_at)
    WHERE purpose = 'profile_photo';

COMMIT;
