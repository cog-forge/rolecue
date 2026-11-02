-- Live interview turns (KAN-52, KAN-60). The realtime interview engine freezes what the interviewer
-- actually said and when, so a stored turn keeps the spoken question even if the bank changes later, and
-- it freezes the provider voice of a session, so a later voice catalog change never rewrites history.
-- Additive only: existing rows keep NULLs (never a made-up time) and the REST answer path keeps working.
BEGIN;

-- The question as the interviewer asked it, set once on its first delivery. NULL means it was not
-- delivered by the live engine yet; readers then fall back to the bank text, as before.
ALTER TABLE public.interview_questions
    ADD COLUMN spoken_text text,
    ADD COLUMN asked_at timestamptz,
    ADD CONSTRAINT interview_questions_asked_pair_chk CHECK ((spoken_text IS NULL) = (asked_at IS NULL));

-- kind tells a core question from a bounded follow-up (KAN-56 writes follow-ups later). asked_at is the
-- delivery time of the question the turn answers, answered_at when the answer was stored. Rows written
-- before this migration have no times.
ALTER TABLE public.conversation_turns
    ADD COLUMN kind text NOT NULL DEFAULT 'core',
    ADD COLUMN asked_at timestamptz,
    ADD COLUMN answered_at timestamptz,
    ADD CONSTRAINT conversation_turns_kind_chk CHECK (kind IN ('core', 'follow_up'));
ALTER TABLE public.conversation_turns ALTER COLUMN answered_at SET DEFAULT statement_timestamp();

-- One turn per position: a retried or replayed answer can never be stored twice, even outside the
-- session lock the repository takes.
ALTER TABLE public.conversation_turns
    ADD CONSTRAINT conversation_turns_interview_position_key UNIQUE (interview_id, position);

-- The provider voice (voices.provider_voice_id, or the configured default) the session speaks with, set
-- once on the first delivery.
ALTER TABLE public.interviews ADD COLUMN provider_voice_id text;

COMMIT;
