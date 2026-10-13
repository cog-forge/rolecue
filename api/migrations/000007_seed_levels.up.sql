-- Seeds the seniority levels that JD extraction produces (jd/domain.Seniority).
-- levels.name holds the extraction value as is, so a level is resolved with
-- lower(name) = <seniority>. A JD whose extraction has no seniority gets no
-- default level: the user picks one during review before the JD is saved,
-- because job_descriptions.level_id is NOT NULL.
-- Re-running is safe: a level is skipped when its id or its name already exists.
BEGIN;

INSERT INTO public.levels (id, name, description)
SELECT v.id, v.name, v.description
FROM (VALUES
    ('00000000-0000-4000-8000-000000000001'::uuid, 'intern', 'Internship or student role'),
    ('00000000-0000-4000-8000-000000000002'::uuid, 'junior', 'Early-career role'),
    ('00000000-0000-4000-8000-000000000003'::uuid, 'mid', 'Mid-level role'),
    ('00000000-0000-4000-8000-000000000004'::uuid, 'senior', 'Senior role'),
    ('00000000-0000-4000-8000-000000000005'::uuid, 'lead', 'Lead or principal role')
) AS v(id, name, description)
WHERE NOT EXISTS (
    SELECT 1 FROM public.levels l
    WHERE l.id = v.id OR lower(l.name) = v.name
);

COMMIT;
