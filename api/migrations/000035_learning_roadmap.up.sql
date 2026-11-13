-- Learning Roadmap foundation (KAN-342, RM-01; Learning-Roadmaps-Spec section 5).
--
-- Every Role Profile owns exactly one Roadmap. Its topics are rows of roadmap_items, and each practice
-- question that was evaluated links to the topic that receives its result through roadmap_assessments:
--
--   roadmaps             one per Role Profile; generation_status only (pending -> generating -> ready | failed)
--   roadmap_items        a topic: title, description, position; the backend sorts by position then id
--   roadmap_assessments  (interview, assignment position) -> topic; no score, feedback or transcript here
--   core_questions.roadmap_item_id   the one primary topic of a practice bank question (NULL: not mapped)
--   interviews.focus_item_id         NULL = general practice; a topic = practice of that topic only
--
-- There is no archived_at, learning_status, score, progress, snapshot or version column: progress is
-- computed from interview_questions.score on read. updated_at on roadmaps is the fencing token of a
-- generation: a worker writes a result only while the value it read is unchanged.
--
-- Erasure: deleting a Role Profile cascades to its Roadmap and topics; questions that pointed at a topic keep
-- existing with NULL (account erasure deletes them next). Assessments go with their interview question, so
-- the existing erasure order (interview questions before profiles) needs no change. A topic that still has
-- assessments or a focused interview cannot be deleted on its own (RESTRICT): history never changes meaning.
BEGIN;

CREATE TABLE public.roadmaps (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    role_profile_id uuid NOT NULL UNIQUE REFERENCES public.role_profiles(id) ON DELETE CASCADE,
    generation_status text NOT NULL DEFAULT 'pending',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT roadmaps_generation_status_chk CHECK (generation_status IN ('pending', 'generating', 'ready', 'failed'))
);

CREATE TABLE public.roadmap_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    roadmap_id uuid NOT NULL REFERENCES public.roadmaps(id) ON DELETE CASCADE,
    title text NOT NULL,
    description text,
    position integer NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT roadmap_items_title_chk CHECK (btrim(title) <> ''),
    CONSTRAINT roadmap_items_position_chk CHECK (position >= 0)
);
-- Read path: the topics of a Roadmap in display order. position is not unique on purpose (id breaks ties).
CREATE INDEX roadmap_items_roadmap_position_idx ON public.roadmap_items(roadmap_id, position, id);

CREATE TABLE public.roadmap_assessments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    roadmap_item_id uuid NOT NULL REFERENCES public.roadmap_items(id),
    interview_id uuid NOT NULL REFERENCES public.interviews(id) ON DELETE CASCADE,
    -- The assignment position of the core question in the session (interview_questions.position).
    question_position integer NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    -- A core question of a session has one primary topic, and a retry cannot add a second row.
    CONSTRAINT roadmap_assessments_question_key UNIQUE (interview_id, question_position),
    FOREIGN KEY (interview_id, question_position)
        REFERENCES public.interview_questions(interview_id, position) ON DELETE CASCADE
);
-- Read paths: the history of a topic, and the topic's latest result per question (join to interview_questions).
CREATE INDEX roadmap_assessments_item_idx ON public.roadmap_assessments(roadmap_item_id, interview_id, question_position);

ALTER TABLE public.core_questions
    ADD COLUMN roadmap_item_id uuid REFERENCES public.roadmap_items(id) ON DELETE SET NULL;
CREATE INDEX core_questions_roadmap_item_id_idx ON public.core_questions(roadmap_item_id) WHERE roadmap_item_id IS NOT NULL;

ALTER TABLE public.interviews
    ADD COLUMN focus_item_id uuid REFERENCES public.roadmap_items(id),
    -- Recruitment never has a focus; the topic belongs to the profile of the session (trigger below).
    ADD CONSTRAINT interviews_focus_practice_chk CHECK (focus_item_id IS NULL OR type = 'practice');
CREATE INDEX interviews_focus_item_id_idx ON public.interviews(focus_item_id) WHERE focus_item_id IS NOT NULL;
-- "Does this profile have a practice session / an unfinished one for this topic".
CREATE INDEX interviews_role_profile_focus_idx ON public.interviews(role_profile_id, focus_item_id);

-- A foreign key alone cannot say that the topic belongs to the same profile, so two triggers do.
-- They only run when the new column is set, so the existing writes of both tables are untouched.
CREATE FUNCTION public.core_question_item_same_profile() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM public.roadmap_items ri
        JOIN public.roadmaps r ON r.id = ri.roadmap_id
        JOIN public.role_profiles rp ON rp.id = r.role_profile_id
        WHERE ri.id = NEW.roadmap_item_id AND rp.jd_id = NEW.jd_id
    ) THEN
        RAISE EXCEPTION 'roadmap item % does not belong to the profile of job description %', NEW.roadmap_item_id, NEW.jd_id
            USING ERRCODE = '23514', CONSTRAINT = 'core_questions_roadmap_item_profile_chk';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER core_questions_roadmap_item_profile_trg
    BEFORE INSERT OR UPDATE OF roadmap_item_id, jd_id ON public.core_questions
    FOR EACH ROW WHEN (NEW.roadmap_item_id IS NOT NULL)
    EXECUTE FUNCTION public.core_question_item_same_profile();

CREATE FUNCTION public.interview_focus_same_profile() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM public.roadmap_items ri
        JOIN public.roadmaps r ON r.id = ri.roadmap_id
        WHERE ri.id = NEW.focus_item_id AND r.role_profile_id = NEW.role_profile_id
    ) THEN
        RAISE EXCEPTION 'roadmap item % does not belong to role profile %', NEW.focus_item_id, NEW.role_profile_id
            USING ERRCODE = '23514', CONSTRAINT = 'interviews_focus_item_profile_chk';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER interviews_focus_item_profile_trg
    BEFORE INSERT OR UPDATE OF focus_item_id, role_profile_id ON public.interviews
    FOR EACH ROW WHEN (NEW.focus_item_id IS NOT NULL)
    EXECUTE FUNCTION public.interview_focus_same_profile();

-- Existing Role Profiles get a pending Roadmap row. Nothing is generated for them: a profile whose
-- requirements are confirmed is generated by an explicit retry (RM-04), never by this migration.
INSERT INTO public.roadmaps (role_profile_id) SELECT id FROM public.role_profiles;

COMMIT;
