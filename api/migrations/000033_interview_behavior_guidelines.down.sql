BEGIN;

ALTER TABLE public.interviews DROP COLUMN IF EXISTS behavior_guideline_id;
DROP TABLE IF EXISTS public.interview_behavior_guidelines;

COMMIT;
