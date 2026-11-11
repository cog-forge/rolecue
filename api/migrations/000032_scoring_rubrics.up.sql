-- Scoring rubrics ("Barem", KAN-308 / KAN-325, Product Decision 24).
--
-- Adds exactly two configuration tables, rubrics and rubric_criteria, and two nullable columns on
-- core_questions. Nothing here changes how interviews are scored today: the live scoring path, the
-- interview runtime and question-bank generation ignore these objects until later tasks wire them in.
-- There are no snapshot or version tables, no expected-points table and no manual score override.
-- Results (interview_criterion_scores and friends) belong to a later task.
--
-- A rubric is a reusable grading guide for one kind of question. Each criterion belongs to one rubric
-- and exactly one metric of the existing catalog (public.metrics); the metrics and the posting weights
-- stay as they are.
BEGIN;

CREATE TABLE public.rubrics (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    -- A lower-case slug such as concept_explanation. Deliberately text and not an enum: the starter
    -- set is a proposal for review, not a fixed taxonomy.
    question_type text NOT NULL,
    -- system: managed by an Admin and visible to every Recruiter. recruiter: private to owner_id.
    scope text NOT NULL,
    owner_id uuid REFERENCES public.users(id) ON DELETE CASCADE,
    -- Archiving (is_active = false) stops new assignments; existing references stay valid.
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    -- The service sets this on every edit; it is also the optimistic-concurrency token.
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT rubrics_name_not_blank CHECK (name ~ '\S'),
    CONSTRAINT rubrics_question_type_slug CHECK (question_type ~ '^[a-z][a-z0-9_]*$'),
    CONSTRAINT rubrics_scope_check CHECK (scope IN ('system', 'recruiter')),
    CONSTRAINT rubrics_scope_owner_check CHECK (
        (scope = 'system' AND owner_id IS NULL) OR (scope = 'recruiter' AND owner_id IS NOT NULL)
    )
);
-- Library listings: a Recruiter's own rubrics, and the active system ones.
CREATE INDEX rubrics_owner_id_idx ON public.rubrics(owner_id) WHERE owner_id IS NOT NULL;
CREATE INDEX rubrics_scope_active_idx ON public.rubrics(scope, is_active);

CREATE TABLE public.rubric_criteria (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    rubric_id uuid NOT NULL REFERENCES public.rubrics(id) ON DELETE CASCADE,
    metric_id uuid NOT NULL REFERENCES public.metrics(id),
    name text NOT NULL,
    -- A candidate earns an integer from 0 to max_points on this criterion.
    max_points integer NOT NULL,
    -- Plain text that describes every level from 0 to max_points. It is data for the scorer, not a prompt.
    scoring_guide text NOT NULL,
    -- 1-based order inside the rubric.
    position integer NOT NULL,
    CONSTRAINT rubric_criteria_name_not_blank CHECK (name ~ '\S'),
    CONSTRAINT rubric_criteria_max_points_positive CHECK (max_points > 0),
    CONSTRAINT rubric_criteria_guide_not_blank CHECK (scoring_guide ~ '\S'),
    CONSTRAINT rubric_criteria_position_positive CHECK (position >= 1),
    -- Deferred so a reorder (a swap of two positions) is valid inside one transaction.
    CONSTRAINT rubric_criteria_rubric_position_key UNIQUE (rubric_id, position) DEFERRABLE INITIALLY DEFERRED
);
-- The unique constraint above already indexes rubric_id; metric_id needs its own index for the FK.
CREATE INDEX rubric_criteria_metric_id_idx ON public.rubric_criteria(metric_id);

-- Both columns are optional so every existing question keeps working. A question may lack a rubric
-- while a Recruiter is still preparing it. RESTRICT (the default) means a rubric that a question still
-- uses cannot be hard-deleted; archive it instead.
ALTER TABLE public.core_questions
    ADD COLUMN rubric_id uuid REFERENCES public.rubrics(id),
    ADD COLUMN expected_answer text;
-- The "locked because referenced" check and the delete guard look questions up by rubric.
CREATE INDEX core_questions_rubric_id_idx ON public.core_questions(rubric_id) WHERE rubric_id IS NOT NULL;

-- System templates. Fixed ids keep the seed re-runnable (ON CONFLICT DO NOTHING). Metric ids are the
-- catalog seeded by 000011: 101 Technical Accuracy, 102 Depth of Understanding, 103 Problem-Solving &
-- Approach, 104 Answer Relevance, 105 Communication Clarity. Every guide describes each integer level.
INSERT INTO public.rubrics (id, name, description, question_type, scope, owner_id) VALUES
    ('00000000-0000-4000-8000-000000000201', 'Explain a mechanism and justify a solution',
     'For questions that ask the candidate to explain how something works, discuss its limits and justify a choice for a situation. Score against the expected answer of the question; accept equivalent wording and other valid solutions; do not penalise points the question did not ask for.',
     'concept_explanation', 'system', NULL),
    ('00000000-0000-4000-8000-000000000202', 'Compare options and choose',
     'For questions that ask the candidate to compare two or more approaches and pick one for a situation. Score against the expected answer of the question; accept equivalent wording and other valid choices when they are well justified.',
     'comparison_choice', 'system', NULL),
    ('00000000-0000-4000-8000-000000000203', 'Diagnose and resolve a scenario',
     'For questions that describe a problem, failure or debugging situation and ask how the candidate would find and fix it. Score against the expected answer of the question; accept other valid diagnostic paths.',
     'scenario_debugging', 'system', NULL),
    ('00000000-0000-4000-8000-000000000204', 'Describe hands-on technical experience',
     'For questions that ask the candidate to describe real work they did. Score against the expected answer of the question; do not require named tools or a particular project.',
     'technical_experience', 'system', NULL)
ON CONFLICT (id) DO NOTHING;

INSERT INTO public.rubric_criteria (id, rubric_id, metric_id, name, max_points, scoring_guide, position) VALUES
    -- Explain a mechanism and justify a solution: 2 + 3 on Technical Accuracy, 5 on Problem-Solving (10).
    ('00000000-0000-4000-8000-000000000301', '00000000-0000-4000-8000-000000000201', '00000000-0000-4000-8000-000000000101',
     'Explains the core mechanism correctly', 2,
     '0 = wrong or missing; 1 = partly correct: the main idea is present with a significant gap or error; 2 = correct and complete for the main mechanism.', 1),
    ('00000000-0000-4000-8000-000000000302', '00000000-0000-4000-8000-000000000201', '00000000-0000-4000-8000-000000000101',
     'Explains the relevant limits and trade-offs', 3,
     '0 = incorrect or no relevant content; 1 = one correct point but the main limit is missing; 2 = most limits explained correctly but one important point the question asked for is missing; 3 = complete and accurate.', 2),
    ('00000000-0000-4000-8000-000000000303', '00000000-0000-4000-8000-000000000201', '00000000-0000-4000-8000-000000000103',
     'Recommends a solution that fits the situation', 5,
     '0 = no recommendation, or one that ignores the situation; 1 = a recommendation with no reasoning; 2 = a recommendation with weak or partly wrong reasoning; 3 = a sound recommendation whose reasoning uses some facts of the situation; 4 = a sound recommendation well justified against the facts of the situation and its costs; 5 = fully justified, and names the main risk or alternative and when the choice would change.', 3),

    -- Compare options and choose: 3 Technical Accuracy, 3 Depth of Understanding, 4 Problem-Solving (10).
    ('00000000-0000-4000-8000-000000000311', '00000000-0000-4000-8000-000000000202', '00000000-0000-4000-8000-000000000101',
     'Describes each option correctly', 3,
     '0 = descriptions wrong or missing; 1 = one option described correctly, or several with major errors; 2 = all options described correctly with minor gaps; 3 = all options described accurately and completely.', 1),
    ('00000000-0000-4000-8000-000000000312', '00000000-0000-4000-8000-000000000202', '00000000-0000-4000-8000-000000000102',
     'Identifies the trade-offs between the options', 3,
     '0 = no trade-offs identified; 1 = names a trade-off without explaining why it matters; 2 = explains the main trade-offs; 3 = explains the trade-offs and the conditions under which each option is the better one.', 2),
    ('00000000-0000-4000-8000-000000000313', '00000000-0000-4000-8000-000000000202', '00000000-0000-4000-8000-000000000103',
     'Chooses an option that fits the situation and justifies it', 4,
     '0 = no choice made; 1 = a choice with no reasons; 2 = a choice with generic reasons that are not tied to the situation; 3 = a choice justified with the facts of the situation; 4 = a choice justified with the facts, the costs and the risks of the options not chosen.', 3),

    -- Diagnose and resolve a scenario: 4 Problem-Solving, 3 Technical Accuracy, 2 Depth, 1 Clarity (10).
    ('00000000-0000-4000-8000-000000000321', '00000000-0000-4000-8000-000000000203', '00000000-0000-4000-8000-000000000103',
     'Follows a structured diagnosis', 4,
     '0 = no method, only guesses; 1 = jumps to a single guess without checking it; 2 = lists plausible causes but with no order and no way to check them; 3 = prioritises the causes and says how to check each; 4 = systematic: narrows down with concrete checks, uses the facts given, and confirms the cause before fixing.', 1),
    ('00000000-0000-4000-8000-000000000322', '00000000-0000-4000-8000-000000000203', '00000000-0000-4000-8000-000000000101',
     'Identifies a correct cause and fix', 3,
     '0 = wrong cause and wrong fix; 1 = a related cause, or a fix that treats only the symptom; 2 = a correct cause or a correct fix, but not both, or not explained well; 3 = a correct root cause and an appropriate fix, explained accurately.', 2),
    ('00000000-0000-4000-8000-000000000323', '00000000-0000-4000-8000-000000000203', '00000000-0000-4000-8000-000000000102',
     'Considers side effects and prevention', 2,
     '0 = neither is considered; 1 = mentions either the risks of the fix or a way to prevent a recurrence; 2 = covers both the risks of the fix and how to prevent a recurrence.', 3),
    ('00000000-0000-4000-8000-000000000324', '00000000-0000-4000-8000-000000000203', '00000000-0000-4000-8000-000000000105',
     'Explains the steps clearly and in order', 1,
     '0 = hard to follow or out of order; 1 = the steps are stated clearly in a logical order.', 4),

    -- Describe hands-on technical experience: 2 Answer Relevance, 3 Depth, 3 Technical Accuracy, 2 Clarity (10).
    ('00000000-0000-4000-8000-000000000331', '00000000-0000-4000-8000-000000000204', '00000000-0000-4000-8000-000000000104',
     'Answers the question asked and states their own role', 2,
     '0 = off topic, or no personal role can be seen; 1 = related to the question but the role or the point asked is unclear; 2 = addresses the question directly and states what the candidate did personally.', 1),
    ('00000000-0000-4000-8000-000000000332', '00000000-0000-4000-8000-000000000204', '00000000-0000-4000-8000-000000000102',
     'Gives concrete decisions, difficulties and results', 3,
     '0 = only generalities; 1 = some specifics but no reasons and no outcome; 2 = concrete decisions with reasons, but the difficulties or the outcome are missing; 3 = concrete decisions, the reasons for them, the difficulties met and the outcome.', 2),
    ('00000000-0000-4000-8000-000000000333', '00000000-0000-4000-8000-000000000204', '00000000-0000-4000-8000-000000000101',
     'Describes the technologies and practices correctly', 3,
     '0 = descriptions mostly wrong; 1 = several errors or vague terms; 2 = mostly correct with minor inaccuracies; 3 = accurate and appropriate use of the technologies and terms.', 3),
    ('00000000-0000-4000-8000-000000000334', '00000000-0000-4000-8000-000000000204', '00000000-0000-4000-8000-000000000105',
     'Communicates in a clear structure', 2,
     '0 = confusing; 1 = understandable but disorganised or padded; 2 = clear, organised and to the point.', 4)
ON CONFLICT (id) DO NOTHING;

COMMIT;
