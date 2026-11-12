-- Rubric grading results (KAN-335, RB-11; Scoring-Rubrics-Spec section 4.2).
--
-- Every interview is now graded criterion by criterion against the rubric of each core question; the backend
-- aggregates criterion points -> question metric -> session metric -> weighted overall. This migration stores
-- that chain so a report can explain the overall without recomputing it from data that may change later:
--
--   interview_criterion_scores      points, evidence and feedback of each criterion of each graded question
--   interview_questions.score       the question total (100 x earned / max over its criteria), for display only
--   score_details.score             the session metric score, now with two decimals; NULL for a weighted metric
--                                   no graded question assessed (it is listed, never counted as 0)
--   score_details.weight            the metric weight the overall was calculated with (posting weights, or the
--                                   equal-split defaults, which are not stored anywhere else)
--   score_details.question_count    how many graded questions assessed the metric
--   interviews.score                the weighted overall, now with two decimals; NULL when withheld
--   interviews.score_outcome        complete, partial or withheld; NULL for evaluations from before this change
--   interviews.score_withheld_reason why the overall is withheld
--
-- There is still no snapshot, version or expected-points table and no manual override: rubric criteria are
-- locked once a question references them (RB-07), so a criterion id is enough to read its metric and maximum.
-- Existing rows are kept: their integer scores convert exactly, and a completed evaluation with no outcome is
-- an evaluation made before rubric grading (the report shows it without a breakdown).
BEGIN;

ALTER TABLE public.interviews
    ALTER COLUMN score TYPE numeric(5,2) USING score::numeric(5,2),
    ADD COLUMN score_outcome text,
    ADD COLUMN score_withheld_reason text,
    ADD CONSTRAINT interviews_score_range_chk CHECK (score IS NULL OR (score >= 0 AND score <= 100)),
    ADD CONSTRAINT interviews_score_outcome_chk CHECK (score_outcome IN ('complete', 'partial', 'withheld')),
    ADD CONSTRAINT interviews_score_withheld_reason_chk CHECK (
        score_withheld_reason IN ('missing_metric_coverage', 'partial_session', 'no_answers', 'no_gradable_questions')),
    -- A withheld result has a reason and no overall; a complete or partial one has an overall and no reason.
    ADD CONSTRAINT interviews_score_outcome_consistent_chk CHECK (
        score_outcome IS NULL
        OR (score_outcome = 'withheld' AND score IS NULL AND score_withheld_reason IS NOT NULL)
        OR (score_outcome IN ('complete', 'partial') AND score IS NOT NULL AND score_withheld_reason IS NULL));

ALTER TABLE public.score_details
    ALTER COLUMN score TYPE numeric(5,2) USING score::numeric(5,2),
    ALTER COLUMN score DROP NOT NULL,
    ADD COLUMN weight double precision,
    ADD COLUMN question_count integer,
    ADD CONSTRAINT score_details_score_range_chk CHECK (score IS NULL OR (score >= 0 AND score <= 100)),
    ADD CONSTRAINT score_details_weight_chk CHECK (weight IS NULL OR weight >= 0),
    ADD CONSTRAINT score_details_question_count_chk CHECK (question_count IS NULL OR question_count >= 0),
    -- Only a rubric evaluation writes a metric without a score, and it always records the weight.
    ADD CONSTRAINT score_details_unscored_has_weight_chk CHECK (score IS NOT NULL OR weight IS NOT NULL);

ALTER TABLE public.interview_questions
    ADD COLUMN score numeric(5,2),
    ADD CONSTRAINT interview_questions_score_range_chk CHECK (score IS NULL OR (score >= 0 AND score <= 100));

CREATE TABLE public.interview_criterion_scores (
    interview_id uuid NOT NULL,
    -- The core question as it was assigned to the session (interview_questions.position).
    question_position integer NOT NULL,
    -- The criterion of the rubric of that core question. RESTRICT: a criterion with results cannot go away.
    rubric_criterion_id uuid NOT NULL REFERENCES public.rubric_criteria(id),
    -- 0..max_points of the criterion; the service checks the maximum before anything is written.
    points integer NOT NULL,
    -- A verbatim quote from an answer to this core question or one of its follow-ups, and the conversation
    -- position of that answer. Both are NULL only for 0 points with nothing to quote.
    evidence text,
    evidence_turn_position integer,
    feedback text,
    PRIMARY KEY (interview_id, question_position, rubric_criterion_id),
    -- Results go with their session: erasing an interview's questions or turns removes them too.
    FOREIGN KEY (interview_id, question_position)
        REFERENCES public.interview_questions(interview_id, position) ON DELETE CASCADE,
    FOREIGN KEY (interview_id, evidence_turn_position)
        REFERENCES public.conversation_turns(interview_id, position) ON DELETE CASCADE,
    CONSTRAINT interview_criterion_scores_points_chk CHECK (points >= 0),
    CONSTRAINT interview_criterion_scores_evidence_pair_chk CHECK ((evidence IS NULL) = (evidence_turn_position IS NULL)),
    CONSTRAINT interview_criterion_scores_evidence_needed_chk CHECK (points = 0 OR evidence IS NOT NULL)
);
-- The FK to rubric_criteria is checked when a criterion is deleted.
CREATE INDEX interview_criterion_scores_criterion_idx ON public.interview_criterion_scores(rubric_criterion_id);

-- Cross-cutting metrics in every system template (product owner decision, 2026-11-12). Every question graded
-- with a system template now also measures Answer Relevance (metric 104) and Communication Clarity (metric
-- 105), so a practice session weighted equally over the catalog is not withheld only because those two
-- metrics were never assessed. 201 gets both, 202 gets both, 203 gets Answer Relevance (it already has
-- Clarity), 204 already has both. Depth (102) is still missing from 201 and Problem-Solving (103) from 204.
-- These are system rows changed by a migration, so the API's RUBRIC_LOCKED rule does not apply: a question
-- already bound to one of these templates gets a larger maximum. No question has been graded before this
-- migration, so no stored result changes. Fixed ids keep it re-runnable; the down migration removes them.
INSERT INTO public.rubric_criteria (id, rubric_id, metric_id, name, max_points, scoring_guide, position) VALUES
    ('00000000-0000-4000-8000-000000000304', '00000000-0000-4000-8000-000000000201', '00000000-0000-4000-8000-000000000104',
     'Answers the question that was asked', 2,
     '0 = off topic, or answers a different question; 1 = related to the question but drifts, or leaves a part that was asked unanswered; 2 = addresses the question asked directly and stays on it.', 4),
    ('00000000-0000-4000-8000-000000000305', '00000000-0000-4000-8000-000000000201', '00000000-0000-4000-8000-000000000105',
     'Explains the reasoning in a clear structure', 2,
     '0 = confusing; 1 = understandable but disorganised or padded; 2 = clear, organised and to the point.', 5),
    ('00000000-0000-4000-8000-000000000314', '00000000-0000-4000-8000-000000000202', '00000000-0000-4000-8000-000000000104',
     'Compares the options the question asked about', 2,
     '0 = off topic, or compares other options than the ones asked about; 1 = related, but leaves an option or the situation asked about out; 2 = compares exactly the options asked about, for the situation given.', 4),
    ('00000000-0000-4000-8000-000000000315', '00000000-0000-4000-8000-000000000202', '00000000-0000-4000-8000-000000000105',
     'Communicates the comparison clearly', 2,
     '0 = confusing; 1 = understandable but disorganised or padded; 2 = clear, organised and to the point.', 5),
    ('00000000-0000-4000-8000-000000000325', '00000000-0000-4000-8000-000000000203', '00000000-0000-4000-8000-000000000104',
     'Works on the scenario that was described', 2,
     '0 = ignores the scenario, or solves a different problem; 1 = related to the scenario but ignores facts it gave; 2 = works from the facts of the scenario described.', 5)
ON CONFLICT (id) DO NOTHING;

COMMIT;
