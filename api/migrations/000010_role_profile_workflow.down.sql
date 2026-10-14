BEGIN;

ALTER TABLE public.requirements
    DROP COLUMN kind,
    DROP COLUMN category,
    ALTER COLUMN type SET NOT NULL;

ALTER TABLE public.role_profiles DROP COLUMN status;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM public.job_descriptions WHERE raw_text IS NOT NULL) THEN
        RAISE EXCEPTION 'Cannot remove Role Profile data while raw JD text exists';
    END IF;
END $$;

ALTER TABLE public.job_descriptions
    DROP COLUMN raw_text,
    ALTER COLUMN content_path SET NOT NULL,
    ALTER COLUMN level_id SET NOT NULL;

COMMIT;
