BEGIN;

ALTER TABLE public.job_descriptions
    ADD COLUMN raw_text text,
    ALTER COLUMN content_path DROP NOT NULL,
    ALTER COLUMN level_id DROP NOT NULL;

ALTER TABLE public.role_profiles
    ADD COLUMN status text NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'analyzing', 'review', 'confirmed'));

ALTER TABLE public.requirements
    ADD COLUMN kind text NOT NULL DEFAULT 'skill'
        CHECK (kind IN ('skill', 'technology', 'domain_knowledge')),
    ADD COLUMN category text
        CHECK (category IN ('programming_language', 'framework', 'database', 'tool', 'technology', 'other')),
    ALTER COLUMN type DROP NOT NULL;

COMMIT;
