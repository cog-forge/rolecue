-- Admin Interview Behavior Guidelines (KAN-276, Brain Decision 16.6). Append-only versions: every save
-- inserts a row, rows are never updated or deleted, and the current guideline is the highest version. No
-- row means the built-in defaults. Every column is a bounded value (enum, 0 to 3, or a subset of a closed
-- catalog); there is no free text, so nothing an Admin types ever reaches a prompt. updated_by is a plain
-- column, not a foreign key: the version history must outlive the account. interviews records the version a
-- session started under; NULL means the defaults, which is what every earlier interview ran under.
BEGIN;

CREATE TABLE public.interview_behavior_guidelines (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    version         integer NOT NULL UNIQUE,
    tone            text NOT NULL,
    question_length text NOT NULL,
    follow_up_max   smallint,
    follow_up_focus text NOT NULL,
    avoid_topics    text[] NOT NULL DEFAULT '{}',
    updated_by      uuid NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT interview_behavior_guidelines_version_check CHECK (version >= 1),
    CONSTRAINT interview_behavior_guidelines_tone_check CHECK (tone IN ('neutral', 'friendly', 'formal', 'challenging')),
    CONSTRAINT interview_behavior_guidelines_length_check CHECK (question_length IN ('standard', 'concise')),
    CONSTRAINT interview_behavior_guidelines_max_check CHECK (follow_up_max IS NULL OR follow_up_max BETWEEN 0 AND 3),
    CONSTRAINT interview_behavior_guidelines_focus_check CHECK (follow_up_focus IN ('standard', 'vague_only', 'thorough')),
    CONSTRAINT interview_behavior_guidelines_topics_check CHECK (
        avoid_topics <@ ARRAY['compensation', 'personal_life', 'politics_religion', 'former_employers', 'location_nationality']::text[]
        AND cardinality(avoid_topics) <= 5
    )
);

ALTER TABLE public.interviews
    ADD COLUMN behavior_guideline_id uuid REFERENCES public.interview_behavior_guidelines(id);

COMMIT;
