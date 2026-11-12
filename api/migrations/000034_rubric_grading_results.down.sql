-- Reverts 000034. Criterion results and question totals are dropped, and the criteria added to the system
-- templates are removed (the results that referenced them go first); metric rows without a score (weighted
-- metrics no question assessed) cannot be stored as integers and are deleted; two-decimal scores are rounded
-- half away from zero back to whole numbers.
BEGIN;

DROP TABLE public.interview_criterion_scores;

-- The cross-cutting criteria added to the system templates.
DELETE FROM public.rubric_criteria WHERE id IN (
    '00000000-0000-4000-8000-000000000304', '00000000-0000-4000-8000-000000000305',
    '00000000-0000-4000-8000-000000000314', '00000000-0000-4000-8000-000000000315',
    '00000000-0000-4000-8000-000000000325');

ALTER TABLE public.interview_questions
    DROP CONSTRAINT interview_questions_score_range_chk,
    DROP COLUMN score;

DELETE FROM public.score_details WHERE score IS NULL;
ALTER TABLE public.score_details
    DROP CONSTRAINT score_details_unscored_has_weight_chk,
    DROP CONSTRAINT score_details_question_count_chk,
    DROP CONSTRAINT score_details_weight_chk,
    DROP CONSTRAINT score_details_score_range_chk,
    DROP COLUMN question_count,
    DROP COLUMN weight,
    ALTER COLUMN score TYPE integer USING round(score)::integer,
    ALTER COLUMN score SET NOT NULL;

ALTER TABLE public.interviews
    DROP CONSTRAINT interviews_score_outcome_consistent_chk,
    DROP CONSTRAINT interviews_score_withheld_reason_chk,
    DROP CONSTRAINT interviews_score_outcome_chk,
    DROP CONSTRAINT interviews_score_range_chk,
    DROP COLUMN score_withheld_reason,
    DROP COLUMN score_outcome,
    ALTER COLUMN score TYPE integer USING round(score)::integer;

COMMIT;
