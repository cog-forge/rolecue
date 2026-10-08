BEGIN;

CREATE TABLE public.levels (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    description text
);

CREATE TABLE public.job_descriptions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    refinement_note text,
    content_path text NOT NULL,
    level_id uuid NOT NULL REFERENCES public.levels(id)
);

CREATE TYPE public.requirement_type AS ENUM ('required', 'preferred');
CREATE TABLE public.requirements (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    target text NOT NULL,
    jd_id uuid NOT NULL REFERENCES public.job_descriptions(id),
    type public.requirement_type NOT NULL,
    evidence_text text
);
CREATE INDEX requirements_jd_id_idx ON public.requirements(jd_id);

CREATE TABLE public.core_questions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    jd_id uuid NOT NULL REFERENCES public.job_descriptions(id),
    content text NOT NULL
);
CREATE INDEX core_questions_jd_id_idx ON public.core_questions(jd_id);

CREATE TABLE public.role_profiles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    jd_id uuid NOT NULL UNIQUE REFERENCES public.job_descriptions(id),
    candidate_id uuid NOT NULL REFERENCES public.users(id),
    title text NOT NULL
);
CREATE INDEX role_profiles_candidate_id_idx ON public.role_profiles(candidate_id);

COMMIT;
