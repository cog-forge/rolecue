-- Both filters are optional: a NULL argument disables it. The interview_type filter is compared as
-- text so the generated parameter stays a plain string.
-- name: ListAdminInterviews :many
SELECT i.id,
       i.type::text AS type,
       i.score,
       (i.feedback IS NOT NULL OR i.score IS NOT NULL)::boolean AS has_report,
       (i.record_path IS NOT NULL)::boolean AS has_recording,
       u.id AS owner_id,
       u.name AS owner_name,
       u.email AS owner_email,
       jp.id AS job_posting_id,
       jp.title AS job_posting_title
FROM public.interviews i
JOIN public.role_profiles rp ON rp.id = i.role_profile_id
JOIN public.users u ON u.id = rp.candidate_id
LEFT JOIN public.applications a ON a.id = i.application_id
LEFT JOIN public.job_postings jp ON jp.id = a.job_posting_id
WHERE (sqlc.narg(interview_type)::text IS NULL OR i.type::text = sqlc.narg(interview_type)::text)
  AND (sqlc.narg(candidate_id)::uuid IS NULL OR rp.candidate_id = sqlc.narg(candidate_id)::uuid)
  AND (sqlc.narg(job_posting_id)::uuid IS NULL OR a.job_posting_id = sqlc.narg(job_posting_id)::uuid)
ORDER BY i.id
LIMIT @page_limit OFFSET @page_offset;

-- name: CountAdminInterviews :one
SELECT count(*)
FROM public.interviews i
JOIN public.role_profiles rp ON rp.id = i.role_profile_id
LEFT JOIN public.applications a ON a.id = i.application_id
WHERE (sqlc.narg(interview_type)::text IS NULL OR i.type::text = sqlc.narg(interview_type)::text)
  AND (sqlc.narg(candidate_id)::uuid IS NULL OR rp.candidate_id = sqlc.narg(candidate_id)::uuid)
  AND (sqlc.narg(job_posting_id)::uuid IS NULL OR a.job_posting_id = sqlc.narg(job_posting_id)::uuid);
