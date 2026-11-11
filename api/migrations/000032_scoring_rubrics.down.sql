-- Reverts 000032. Dropping the core_questions columns first removes every reference to a rubric, so the
-- tables can then be dropped (the criteria and the seeded templates go with them).
BEGIN;

DROP INDEX IF EXISTS public.core_questions_rubric_id_idx;
ALTER TABLE public.core_questions
    DROP COLUMN IF EXISTS expected_answer,
    DROP COLUMN IF EXISTS rubric_id;

DROP TABLE IF EXISTS public.rubric_criteria;
DROP TABLE IF EXISTS public.rubrics;

COMMIT;
