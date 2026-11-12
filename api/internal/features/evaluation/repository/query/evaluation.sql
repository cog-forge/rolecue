-- Posting evaluation weights live in metrics_percentage and are separate from the question bank:
-- no query here reads or writes core_questions.

-- name: ListMetrics :many
SELECT id, name
FROM public.metrics
ORDER BY name, id;

-- name: GetRecruiterPostingStatus :one
SELECT status::text AS status
FROM public.job_postings
WHERE id = @id AND recruiter_id = @recruiter_id;

-- name: ListPostingWeights :many
SELECT mp.metric_id, m.name, mp.percentage
FROM public.metrics_percentage mp
JOIN public.metrics m ON m.id = mp.metric_id
WHERE mp.job_posting_id = @job_posting_id
ORDER BY m.name, m.id;

-- Locks the posting so its status cannot change while the weights are replaced; the repository
-- then applies domain.EditableStatus to the locked status. review_submitted tells whether the posting is in
-- the Admin queue, which a weight change that uncovers the bank must leave (KAN-377).
-- name: LockEditablePosting :one
SELECT status::text AS status, review_submitted
FROM public.job_postings
WHERE id = @id AND recruiter_id = @recruiter_id
FOR UPDATE;

-- name: DeletePostingWeights :exec
DELETE FROM public.metrics_percentage
WHERE job_posting_id = @job_posting_id;

-- name: InsertPostingWeight :exec
INSERT INTO public.metrics_percentage (job_posting_id, metric_id, percentage)
VALUES (@job_posting_id, @metric_id, @percentage);

-- A weight change that leaves the bank unable to cover the active metrics takes the posting out of the Admin
-- queue, like a change to a question does (questionbank WithdrawSubmission). Called under the posting lock.
-- name: WithdrawSubmission :execrows
UPDATE public.job_postings SET review_submitted = false WHERE id = @id AND review_submitted;
