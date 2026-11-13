-- Reverts 000035. Roadmaps, topics, assessments and the two links are dropped; the links go first.
BEGIN;

DROP TRIGGER interviews_focus_item_profile_trg ON public.interviews;
DROP FUNCTION public.interview_focus_same_profile();
DROP TRIGGER core_questions_roadmap_item_profile_trg ON public.core_questions;
DROP FUNCTION public.core_question_item_same_profile();

DROP INDEX public.interviews_role_profile_focus_idx;
DROP INDEX public.interviews_focus_item_id_idx;
ALTER TABLE public.interviews
    DROP CONSTRAINT interviews_focus_practice_chk,
    DROP COLUMN focus_item_id;

DROP INDEX public.core_questions_roadmap_item_id_idx;
ALTER TABLE public.core_questions DROP COLUMN roadmap_item_id;

DROP TABLE public.roadmap_assessments;
DROP TABLE public.roadmap_items;
DROP TABLE public.roadmaps;

COMMIT;
