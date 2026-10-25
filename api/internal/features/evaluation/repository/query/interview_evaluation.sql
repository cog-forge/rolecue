-- Interview evaluation and report. Scores live in score_details, per-question feedback in
-- conversation_turns and the overall score, summary and status in interviews. The posting weights are
-- read with the ListMetrics and ListPostingWeights queries of evaluation.sql.

-- The interview with the owners the report visibility rules need. The Application and its posting are
-- joined for recruitment sessions only, so their columns are null for practice.
-- name: GetInterviewRecord :one
SELECT i.id, i.type::text AS type, i.started_at, i.completed_at, i.evaluation_status, i.score, i.feedback,
       rp.candidate_id, a.job_posting_id, jp.recruiter_id
FROM public.interviews i
JOIN public.role_profiles rp ON rp.id = i.role_profile_id
LEFT JOIN public.applications a ON a.id = i.application_id
LEFT JOIN public.job_postings jp ON jp.id = a.job_posting_id
WHERE i.id = @id;

-- Locks the interview row for the one transaction that writes its evaluation.
-- name: LockInterviewForEvaluation :one
SELECT evaluation_status
FROM public.interviews
WHERE id = @id
FOR UPDATE;

-- name: ListEvaluationTurns :many
SELECT position, question, answer, feedback
FROM public.conversation_turns
WHERE interview_id = @interview_id
ORDER BY position;

-- name: ListInterviewScores :many
SELECT sd.metric_id, m.name, sd.score
FROM public.score_details sd
JOIN public.metrics m ON m.id = sd.metric_id
WHERE sd.interview_id = @interview_id
ORDER BY m.name, m.id;

-- name: UpsertScoreDetail :exec
INSERT INTO public.score_details (interview_id, metric_id, score)
VALUES (@interview_id, @metric_id, @score)
ON CONFLICT (interview_id, metric_id) DO UPDATE SET score = EXCLUDED.score;

-- name: SetTurnFeedback :exec
UPDATE public.conversation_turns
SET feedback = @feedback
WHERE interview_id = @interview_id AND position = @position;

-- name: CompleteInterviewEvaluation :exec
UPDATE public.interviews
SET score = @overall_score, feedback = sqlc.narg(feedback), evaluation_status = 'completed'
WHERE id = @id;

-- A completed evaluation is never turned back into a failed one.
-- name: MarkInterviewEvaluationFailed :exec
UPDATE public.interviews
SET evaluation_status = 'failed'
WHERE id = @id AND evaluation_status <> 'completed';
