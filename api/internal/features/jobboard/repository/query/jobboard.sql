-- The job board is the Candidate's read-only view of Job Postings. Only status 'open' (approved by
-- an Admin and still accepting Applications) is discoverable, and only once its Recruiter has funded
-- interview slots (interview_slot IS NOT NULL): funding is what opens intake to Candidates. An
-- approved posting that is not funded yet is not found, like every other status.
-- A posting whose Recruiter is currently locked is hidden too (is_locked with no expiry or an
-- expiry still ahead). Nothing is written: once the lock is lifted or expires the posting shows
-- again, and its status, Applications and refunds are untouched. Every query repeats the predicate.
-- Queries never select the question bank, evaluation weights, capacity, the reject reason or the
-- required avatar/voice.

-- Every filter is optional: a NULL argument disables it. search and location are LIKE patterns the
-- service has already escaped; employment_type matches the whole value, ignoring case.
-- name: ListOpenPostings :many
SELECT jp.id, jp.title, u.company_name, jp.location, jp.employment_type, jp.salary_min, jp.salary_max,
       lower(l.name) AS level_name, jp.created_at
FROM public.job_postings jp
JOIN public.users u ON u.id = jp.recruiter_id
JOIN public.job_descriptions jd ON jd.id = jp.jd_id
JOIN public.levels l ON l.id = jd.level_id
WHERE jp.status = 'open' AND jp.interview_slot IS NOT NULL
  AND NOT (u.is_locked IS TRUE AND (u.lock_expires_at IS NULL OR u.lock_expires_at > now()))
  AND (sqlc.narg(search)::text IS NULL OR jp.title ILIKE '%' || sqlc.narg(search)::text || '%')
  AND (sqlc.narg(location)::text IS NULL OR jp.location ILIKE '%' || sqlc.narg(location)::text || '%')
  AND (sqlc.narg(employment_type)::text IS NULL OR lower(jp.employment_type) = lower(sqlc.narg(employment_type)::text))
ORDER BY jp.created_at DESC, jp.id DESC
LIMIT @page_limit OFFSET @page_offset;

-- Joins the same tables as the list, so a posting the list cannot show is not counted either.
-- name: CountOpenPostings :one
SELECT count(*)
FROM public.job_postings jp
JOIN public.users u ON u.id = jp.recruiter_id
JOIN public.job_descriptions jd ON jd.id = jp.jd_id
JOIN public.levels l ON l.id = jd.level_id
WHERE jp.status = 'open' AND jp.interview_slot IS NOT NULL
  AND NOT (u.is_locked IS TRUE AND (u.lock_expires_at IS NULL OR u.lock_expires_at > now()))
  AND (sqlc.narg(search)::text IS NULL OR jp.title ILIKE '%' || sqlc.narg(search)::text || '%')
  AND (sqlc.narg(location)::text IS NULL OR jp.location ILIKE '%' || sqlc.narg(location)::text || '%')
  AND (sqlc.narg(employment_type)::text IS NULL OR lower(jp.employment_type) = lower(sqlc.narg(employment_type)::text));

-- name: GetOpenPosting :one
SELECT jp.id, jp.jd_id, jp.title, u.company_name, u.company_website, jp.location, jp.employment_type,
       jp.salary_min, jp.salary_max, jp.interview_duration, lower(l.name) AS level_name, jp.created_at,
       jd.content_path, jd.raw_text,
       EXISTS (
           SELECT 1 FROM public.applications a
           WHERE a.job_posting_id = jp.id AND a.candidate_id = @candidate_id
       ) AS already_applied
FROM public.job_postings jp
JOIN public.users u ON u.id = jp.recruiter_id
JOIN public.job_descriptions jd ON jd.id = jp.jd_id
JOIN public.levels l ON l.id = jd.level_id
WHERE jp.id = @id AND jp.status = 'open' AND jp.interview_slot IS NOT NULL
  AND NOT (u.is_locked IS TRUE AND (u.lock_expires_at IS NULL OR u.lock_expires_at > now()));

-- name: ListPostingRequirements :many
SELECT name, target, type::text AS type
FROM public.requirements
WHERE jd_id = @jd_id
ORDER BY name, id;
