BEGIN;

DROP TABLE public.applications;
DROP TABLE public.metrics_percentage;
DROP TABLE public.job_postings;
DROP TABLE public.metrics;
DROP TYPE public.application_status;
DROP TYPE public.job_posting_status;

COMMIT;
