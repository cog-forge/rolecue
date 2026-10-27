BEGIN;

ALTER TABLE public.interviews DROP CONSTRAINT IF EXISTS interviews_practice_has_role_profile;
-- Recruitment sessions without a role profile cannot go back to NOT NULL; they are removed with their
-- children first, as this migration only ever ran on development data.
DELETE FROM public.score_details WHERE interview_id IN (SELECT id FROM public.interviews WHERE role_profile_id IS NULL);
DELETE FROM public.conversation_turns WHERE interview_id IN (SELECT id FROM public.interviews WHERE role_profile_id IS NULL);
DELETE FROM public.interview_questions WHERE interview_id IN (SELECT id FROM public.interviews WHERE role_profile_id IS NULL);
DELETE FROM public.interviews WHERE role_profile_id IS NULL;
ALTER TABLE public.interviews ALTER COLUMN role_profile_id SET NOT NULL;

DROP INDEX IF EXISTS public.interviews_start_key_key;
DROP INDEX IF EXISTS public.interviews_started_at_idx;
ALTER TABLE public.interviews
    DROP COLUMN IF EXISTS start_key,
    DROP COLUMN IF EXISTS record_status,
    DROP COLUMN IF EXISTS evaluation_claimed_at,
    DROP COLUMN IF EXISTS evaluation_status,
    DROP COLUMN IF EXISTS completed_at,
    DROP COLUMN IF EXISTS started_at;

COMMIT;
