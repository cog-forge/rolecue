-- Interview evaluation and report (rubric grading, KAN-334/335/336). Criterion results live in
-- interview_criterion_scores, question totals in interview_questions.score, session metric scores and the
-- weights they were combined with in score_details, the overall, its outcome and the summary in interviews,
-- and per-question feedback on the core turn in conversation_turns. Scores are numeric(5,2) in the database
-- converted to float64 by the repository (dbnum). The posting weights are read with the ListMetrics and ListPostingWeights queries of
-- evaluation.sql.

-- The interview with the owners the report visibility rules need. The Application and its posting are
-- joined for recruitment sessions only, so their columns are null for practice.
-- name: GetInterviewRecord :one
SELECT i.id, i.type::text AS type, i.started_at, i.completed_at, i.end_reason, i.evaluation_status,
       i.score, i.score_outcome, i.score_withheld_reason, i.feedback, i.injection_flagged,
       COALESCE(rp.candidate_id, a.candidate_id)::uuid AS candidate_id, a.job_posting_id, jp.recruiter_id,
       (SELECT count(*) FROM public.interview_questions iq WHERE iq.interview_id = i.id)::int AS question_count
FROM public.interviews i
LEFT JOIN public.role_profiles rp ON rp.id = i.role_profile_id
LEFT JOIN public.applications a ON a.id = i.application_id
LEFT JOIN public.job_postings jp ON jp.id = a.job_posting_id
WHERE i.id = @id;

-- Locks the interview row for the one transaction that writes its evaluation.
-- name: LockInterviewForEvaluation :one
SELECT evaluation_status
FROM public.interviews
WHERE id = @id
FOR UPDATE;

-- question_number is the core question a turn belongs to (a follow-up follows its core question), derived
-- like ListInterviewTurns in the interview feature. It equals interview_questions.position of that core
-- question, because the runtime asks the assigned questions in position order and stores one core turn each.
-- name: ListEvaluationTurns :many
SELECT position,
       (count(*) FILTER (WHERE kind = 'core') OVER (ORDER BY position))::int AS question_number,
       kind, question, answer, feedback
FROM public.conversation_turns
WHERE interview_id = @interview_id
ORDER BY position;

-- The core questions assigned to a session with their grading configuration. An archived rubric still
-- grades the questions that already use it (archiving only stops new assignments). The expected answer is
-- read only by the grader and is never returned to a client.
-- name: ListGradingQuestions :many
SELECT iq.position, cq.rubric_id, cq.expected_answer, r.name AS rubric_name, r.description AS rubric_description
FROM public.interview_questions iq
JOIN public.core_questions cq ON cq.id = iq.core_question_id
LEFT JOIN public.rubrics r ON r.id = cq.rubric_id
WHERE iq.interview_id = @interview_id
ORDER BY iq.position;

-- name: ListGradingCriteria :many
SELECT c.id, c.rubric_id, c.metric_id, m.name AS metric_name, c.name, c.max_points, c.scoring_guide
FROM public.rubric_criteria c
JOIN public.metrics m ON m.id = c.metric_id
WHERE c.rubric_id = ANY(@rubric_ids::uuid[])
ORDER BY c.rubric_id, c.position;

-- Clears anything a previous, uncommitted-elsewhere attempt could have left. Results are only ever written
-- inside the completing transaction, so this is a guard, not a repair.
-- name: DeleteCriterionScores :exec
DELETE FROM public.interview_criterion_scores WHERE interview_id = @interview_id;

-- name: DeleteScoreDetails :exec
DELETE FROM public.score_details WHERE interview_id = @interview_id;

-- name: InsertCriterionScore :exec
INSERT INTO public.interview_criterion_scores
    (interview_id, question_position, rubric_criterion_id, points, evidence, evidence_turn_position, feedback)
VALUES (@interview_id, @question_position, @rubric_criterion_id, @points,
        sqlc.narg(evidence), sqlc.narg(evidence_turn_position), sqlc.narg(feedback));

-- name: SetQuestionScore :exec
UPDATE public.interview_questions
SET score = CAST(sqlc.arg(score)::double precision AS numeric(5,2))
WHERE interview_id = @interview_id AND position = @position;

-- name: InsertScoreDetail :exec
INSERT INTO public.score_details (interview_id, metric_id, score, weight, question_count)
VALUES (@interview_id, @metric_id, CAST(sqlc.narg(score)::double precision AS numeric(5,2)), @weight, @question_count);

-- name: SetTurnFeedback :exec
UPDATE public.conversation_turns
SET feedback = @feedback
WHERE interview_id = @interview_id AND position = @position;

-- name: CompleteInterviewEvaluation :exec
UPDATE public.interviews
SET score = CAST(sqlc.narg(overall_score)::double precision AS numeric(5,2)), score_outcome = @score_outcome,
    score_withheld_reason = sqlc.narg(score_withheld_reason), feedback = sqlc.narg(feedback),
    injection_flagged = @injection_flagged, evaluation_status = 'completed'
WHERE id = @id;

-- A completed evaluation is never turned back into a failed one.
-- name: MarkInterviewEvaluationFailed :exec
UPDATE public.interviews
SET evaluation_status = 'failed'
WHERE id = @id AND evaluation_status <> 'completed';

-- The session metric scores with the weight each was combined with. score is null for a weighted metric no
-- graded question assessed.
-- name: ListInterviewScores :many
SELECT sd.metric_id, m.name, sd.score, sd.weight, sd.question_count, sd.evidence
FROM public.score_details sd
JOIN public.metrics m ON m.id = sd.metric_id
WHERE sd.interview_id = @interview_id
ORDER BY m.name, m.id;

-- Every assigned core question with its total (null when it was not graded) and its rubric. rubric_scope tells
-- a system template, whose text any reader may see, from a Recruiter's private rubric.
-- name: ListQuestionScores :many
SELECT iq.position, iq.score, cq.rubric_id, r.name AS rubric_name, r.scope AS rubric_scope
FROM public.interview_questions iq
JOIN public.core_questions cq ON cq.id = iq.core_question_id
LEFT JOIN public.rubrics r ON r.id = cq.rubric_id
WHERE iq.interview_id = @interview_id
ORDER BY iq.position;

-- The criterion results of a session with the criterion as it was graded (criteria are locked once used).
-- name: ListCriterionScores :many
SELECT s.question_position, s.rubric_criterion_id, c.name, c.metric_id, m.name AS metric_name, c.max_points,
       s.points, s.evidence, s.evidence_turn_position, s.feedback
FROM public.interview_criterion_scores s
JOIN public.rubric_criteria c ON c.id = s.rubric_criterion_id
JOIN public.metrics m ON m.id = c.metric_id
WHERE s.interview_id = @interview_id
ORDER BY s.question_position, c.position;
