-- The Candidate's own session history, newest first. The Candidate is taken through the role profile of
-- each session, so another Candidate's sessions never match. The interview_type filter is optional: a
-- NULL argument disables it. The order ends on the id so a page boundary never splits or repeats rows.
-- Only list columns are selected; questions, turns, feedback and the recording path are never read here.
-- name: ListCandidateInterviews :many
SELECT i.id,
       i.type::text AS type,
       i.started_at,
       i.completed_at,
       i.evaluation_status,
       i.score,
       rp.title AS role_profile_title,
       jp.title AS job_posting_title,
       i.application_id
FROM public.interviews i
JOIN public.role_profiles rp ON rp.id = i.role_profile_id
LEFT JOIN public.applications a ON a.id = i.application_id
LEFT JOIN public.job_postings jp ON jp.id = a.job_posting_id
WHERE rp.candidate_id = @candidate_id
  AND (sqlc.narg(interview_type)::text IS NULL OR i.type::text = sqlc.narg(interview_type)::text)
ORDER BY i.started_at DESC, i.id DESC
LIMIT @page_limit OFFSET @page_offset;

-- name: CountCandidateInterviews :one
SELECT count(*)::bigint AS total
FROM public.interviews i
JOIN public.role_profiles rp ON rp.id = i.role_profile_id
WHERE rp.candidate_id = @candidate_id
  AND (sqlc.narg(interview_type)::text IS NULL OR i.type::text = sqlc.narg(interview_type)::text);
