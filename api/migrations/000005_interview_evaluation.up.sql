BEGIN;

CREATE TYPE public.interview_type AS ENUM ('practice', 'recruitment');

CREATE TABLE public.interviews (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    role_profile_id uuid NOT NULL REFERENCES public.role_profiles(id),
    feedback text,
    score integer,
    avatar_id uuid REFERENCES public.avatars(id),
    voice_id uuid REFERENCES public.voices(id),
    type public.interview_type NOT NULL,
    record_path text,
    application_id uuid UNIQUE REFERENCES public.applications(id),
    CONSTRAINT interviews_application_type_check CHECK (
        (type = 'practice' AND application_id IS NULL)
        OR (type = 'recruitment' AND application_id IS NOT NULL)
    )
);
CREATE INDEX interviews_role_profile_id_idx ON public.interviews(role_profile_id);
CREATE INDEX interviews_avatar_id_idx ON public.interviews(avatar_id);
CREATE INDEX interviews_voice_id_idx ON public.interviews(voice_id);

CREATE TABLE public.conversation_turns (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    interview_id uuid NOT NULL REFERENCES public.interviews(id),
    position integer NOT NULL,
    question text NOT NULL,
    answer text,
    feedback text
);
CREATE INDEX conversation_turns_interview_id_idx ON public.conversation_turns(interview_id);

CREATE TABLE public.interview_questions (
    interview_id uuid NOT NULL REFERENCES public.interviews(id),
    position integer NOT NULL,
    core_question_id uuid NOT NULL REFERENCES public.core_questions(id),
    PRIMARY KEY (interview_id, position)
);
CREATE INDEX interview_questions_core_question_id_idx ON public.interview_questions(core_question_id);

CREATE TABLE public.score_details (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    interview_id uuid NOT NULL REFERENCES public.interviews(id),
    metric_id uuid NOT NULL REFERENCES public.metrics(id),
    score integer NOT NULL,
    CONSTRAINT score_details_interview_metric_unique UNIQUE (interview_id, metric_id)
);
CREATE INDEX score_details_metric_id_idx ON public.score_details(metric_id);

COMMIT;
