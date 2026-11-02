BEGIN;

ALTER TABLE public.interviews DROP COLUMN provider_voice_id;

ALTER TABLE public.conversation_turns
    DROP CONSTRAINT conversation_turns_interview_position_key,
    DROP CONSTRAINT conversation_turns_kind_chk,
    DROP COLUMN answered_at,
    DROP COLUMN asked_at,
    DROP COLUMN kind;

ALTER TABLE public.interview_questions
    DROP CONSTRAINT interview_questions_asked_pair_chk,
    DROP COLUMN asked_at,
    DROP COLUMN spoken_text;

COMMIT;
