-- Removes only the seeded rows that no job description uses yet; a level in use
-- stays, since deleting it would break job_descriptions.level_id.
BEGIN;

DELETE FROM public.levels l
WHERE l.id IN (
    '00000000-0000-4000-8000-000000000001',
    '00000000-0000-4000-8000-000000000002',
    '00000000-0000-4000-8000-000000000003',
    '00000000-0000-4000-8000-000000000004',
    '00000000-0000-4000-8000-000000000005'
)
AND NOT EXISTS (SELECT 1 FROM public.job_descriptions jd WHERE jd.level_id = l.id);

COMMIT;
