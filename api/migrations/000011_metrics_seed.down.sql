-- Removes only the seeded metrics that no posting weight or score uses yet; a metric in use stays,
-- since deleting it would break metrics_percentage.metric_id or score_details.metric_id.
BEGIN;

DELETE FROM public.metrics m
WHERE m.id IN (
    '00000000-0000-4000-8000-000000000101',
    '00000000-0000-4000-8000-000000000102',
    '00000000-0000-4000-8000-000000000103',
    '00000000-0000-4000-8000-000000000104',
    '00000000-0000-4000-8000-000000000105'
)
AND NOT EXISTS (SELECT 1 FROM public.metrics_percentage mp WHERE mp.metric_id = m.id)
AND NOT EXISTS (SELECT 1 FROM public.score_details sd WHERE sd.metric_id = m.id);

DROP INDEX IF EXISTS public.metrics_name_lower_key;

COMMIT;
