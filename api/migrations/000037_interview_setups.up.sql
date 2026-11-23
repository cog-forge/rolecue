-- Persisted pre-interview setup (KAN-231). One row keeps the interviewer avatar, the voice and the room a
-- Candidate chose before a practice interview, or the room they chose for a recruitment interview:
--
--   practice      role_profile_id set; avatar_id, voice_id and environment are the Candidate's choices (NULL: not
--                 chosen yet). One setup per Role Profile.
--   recruitment   application_id set; only environment is the Candidate's. The avatar and voice are the ones the
--                 Recruiter locked on the posting and are read from there, never stored here (the CHECK keeps
--                 them NULL). One setup per Application.
--
-- The row is a draft the Candidate comes back to. It is not an interview: starting still goes through
-- POST /interviews, which neither reads nor writes this table, and nothing here debits coins or uses a slot.
-- Difficulty and duration are not part of the setup (they come from the Job Description and the bank).
-- environment is a room id of the picker; the allowed ids live in code (single list), not in SQL.
--
-- Delete actions:
--   role_profile_id   CASCADE   a Role Profile that goes (account erasure) takes its setup with it
--   application_id    CASCADE   so does an Application (the Candidate's, or the posting Recruiter's erasure)
--   avatar_id         SET NULL  deleting an avatar must keep working; the setup then reports avatar_required
--   voice_id          SET NULL  same for a removed voice
BEGIN;

CREATE TABLE public.interview_setups (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    type public.interview_type NOT NULL,
    role_profile_id uuid UNIQUE REFERENCES public.role_profiles(id) ON DELETE CASCADE,
    application_id uuid UNIQUE REFERENCES public.applications(id) ON DELETE CASCADE,
    avatar_id uuid REFERENCES public.avatars(id) ON DELETE SET NULL,
    voice_id uuid REFERENCES public.voices(id) ON DELETE SET NULL,
    environment text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT interview_setups_target_chk CHECK (
        (type = 'practice' AND role_profile_id IS NOT NULL AND application_id IS NULL)
        OR (type = 'recruitment' AND application_id IS NOT NULL AND role_profile_id IS NULL
            AND avatar_id IS NULL AND voice_id IS NULL)),
    CONSTRAINT interview_setups_environment_chk CHECK (environment IS NULL OR (btrim(environment) <> '' AND length(environment) <= 64))
);

-- The unique constraints index the targets. The choices are indexed for the SET NULL on an avatar or voice delete.
CREATE INDEX interview_setups_avatar_id_idx ON public.interview_setups(avatar_id) WHERE avatar_id IS NOT NULL;
CREATE INDEX interview_setups_voice_id_idx ON public.interview_setups(voice_id) WHERE voice_id IS NOT NULL;

COMMIT;
