-- Every filter is optional: a NULL argument disables it. The date window is [created_from, created_to).
-- A pending posting the Recruiter is still preparing is not in the review queue; submission
-- needs a non-empty bank, and removing the last question withdraws it.
-- name: ListPostings :many
SELECT jp.id, jp.title, jp.status::text AS status, jp.created_at,
       u.id AS recruiter_id, u.name AS recruiter_name, u.email AS recruiter_email
FROM public.job_postings jp
JOIN public.users u ON u.id = jp.recruiter_id
WHERE (jp.status <> 'pending' OR jp.review_submitted)
  AND (sqlc.narg(status)::text IS NULL OR jp.status::text = sqlc.narg(status)::text)
  AND (sqlc.narg(recruiter_id)::uuid IS NULL OR jp.recruiter_id = sqlc.narg(recruiter_id)::uuid)
  AND (sqlc.narg(created_from)::timestamptz IS NULL OR jp.created_at >= sqlc.narg(created_from)::timestamptz)
  AND (sqlc.narg(created_to)::timestamptz IS NULL OR jp.created_at < sqlc.narg(created_to)::timestamptz)
ORDER BY jp.created_at DESC, jp.id DESC
LIMIT @page_limit OFFSET @page_offset;

-- name: CountPostings :one
SELECT count(*)
FROM public.job_postings jp
WHERE (jp.status <> 'pending' OR jp.review_submitted)
  AND (sqlc.narg(status)::text IS NULL OR jp.status::text = sqlc.narg(status)::text)
  AND (sqlc.narg(recruiter_id)::uuid IS NULL OR jp.recruiter_id = sqlc.narg(recruiter_id)::uuid)
  AND (sqlc.narg(created_from)::timestamptz IS NULL OR jp.created_at >= sqlc.narg(created_from)::timestamptz)
  AND (sqlc.narg(created_to)::timestamptz IS NULL OR jp.created_at < sqlc.narg(created_to)::timestamptz);

-- name: GetPosting :one
SELECT jp.id, jp.title, jp.status::text AS status, jp.reject_reason, jp.location, jp.employment_type,
       jp.salary_min, jp.salary_max, jp.interview_slot, jp.interview_duration, jp.created_at,
       u.id AS recruiter_id, u.name AS recruiter_name, u.email AS recruiter_email,
       jp.required_avatar_id, a.name AS avatar_name,
       jp.required_voice_id, v.name AS voice_name, COALESCE(jd.content_path, '')::text AS content_path,
       jd.raw_text, jp.review_submitted,
       (SELECT count(*) FROM public.core_questions cq WHERE cq.jd_id = jp.jd_id) AS core_question_count,
       EXISTS (SELECT 1 FROM public.metrics_percentage mp WHERE mp.job_posting_id = jp.id) AS weights_configured
FROM public.job_postings jp
JOIN public.users u ON u.id = jp.recruiter_id
JOIN public.job_descriptions jd ON jd.id = jp.jd_id
LEFT JOIN public.avatars a ON a.id = jp.required_avatar_id
LEFT JOIN public.voices v ON v.id = jp.required_voice_id
WHERE jp.id = @id;

-- The status guard sits in the same statement, so two admins deciding at once cannot both win.
-- Only a submitted posting can be decided. Approval also needs a non-empty core-question bank: the
-- bank is locked once the posting is approved, so approving an empty one would leave the posting
-- with nothing to interview from.
-- name: DecidePosting :execrows
UPDATE public.job_postings
SET status = sqlc.arg(new_status)::text::public.job_posting_status,
    reject_reason = sqlc.narg(reject_reason)::text,
    review_submitted = (sqlc.arg(new_status)::text <> 'rejected')
WHERE job_postings.id = @id AND job_postings.status = 'pending' AND review_submitted
  AND (sqlc.arg(new_status)::text <> 'open'
       OR EXISTS (SELECT 1 FROM public.core_questions cq WHERE cq.jd_id = job_postings.jd_id));
