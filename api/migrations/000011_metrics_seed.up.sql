-- Seeds the evaluation metrics Recruiters weight per Job Posting (metrics_percentage). These are
-- the brain's baseline dimensions; the brain calls them illustrative, so a later migration may
-- change them. The API reads the catalog from this table and never hardcodes it.
-- Re-running is safe: a metric is skipped when its id or its name already exists.
BEGIN;

CREATE UNIQUE INDEX IF NOT EXISTS metrics_name_lower_key ON public.metrics (lower(name));

INSERT INTO public.metrics (id, name)
SELECT v.id, v.name
FROM (VALUES
    ('00000000-0000-4000-8000-000000000101'::uuid, 'Technical Accuracy'),
    ('00000000-0000-4000-8000-000000000102'::uuid, 'Depth of Understanding'),
    ('00000000-0000-4000-8000-000000000103'::uuid, 'Problem-Solving & Approach'),
    ('00000000-0000-4000-8000-000000000104'::uuid, 'Answer Relevance'),
    ('00000000-0000-4000-8000-000000000105'::uuid, 'Communication Clarity')
) AS v(id, name)
WHERE NOT EXISTS (
    SELECT 1 FROM public.metrics m
    WHERE m.id = v.id OR lower(m.name) = lower(v.name)
);

COMMIT;
