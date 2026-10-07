BEGIN;

CREATE TYPE public.job_posting_status AS ENUM (
    'pending', 'open', 'intake_closed', 'closed', 'rejected'
);
CREATE TYPE public.application_status AS ENUM (
    'pending', 'interview_eligible', 'interviewed', 'approved', 'rejected'
);

CREATE TABLE public.metrics (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL
);

CREATE TABLE public.job_postings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    status public.job_posting_status NOT NULL DEFAULT 'pending',
    jd_id uuid NOT NULL UNIQUE REFERENCES public.job_descriptions(id),
    interview_slot integer,
    required_avatar_id uuid REFERENCES public.avatars(id),
    required_voice_id uuid REFERENCES public.voices(id),
    recruiter_id uuid NOT NULL REFERENCES public.users(id),
    title text NOT NULL,
    reject_reason text,
    location text,
    employment_type text,
    salary_min integer,
    salary_max integer,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    interview_duration integer
);
CREATE INDEX job_postings_recruiter_id_idx ON public.job_postings(recruiter_id);

CREATE TABLE public.metrics_percentage (
    metric_id uuid NOT NULL REFERENCES public.metrics(id),
    percentage double precision NOT NULL,
    job_posting_id uuid NOT NULL REFERENCES public.job_postings(id),
    PRIMARY KEY (job_posting_id, metric_id)
);
CREATE INDEX metrics_percentage_metric_id_idx ON public.metrics_percentage(metric_id);

CREATE TABLE public.applications (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    job_posting_id uuid NOT NULL REFERENCES public.job_postings(id),
    status public.application_status NOT NULL DEFAULT 'pending',
    reject_reason text,
    candidate_id uuid NOT NULL REFERENCES public.users(id),
    cv_path text,
    interview_deadline timestamptz,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX applications_job_posting_id_idx ON public.applications(job_posting_id);
CREATE INDEX applications_candidate_id_idx ON public.applications(candidate_id);

COMMIT;
