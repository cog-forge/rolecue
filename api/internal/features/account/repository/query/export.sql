-- name: ExportProfile :one
SELECT id, email, name, COALESCE(role, 'candidate')::text AS role, image, company_name, company_website, created_at
FROM public.users
WHERE id = @id;

-- name: ExportRoleProfiles :many
SELECT id, title, status, created_at
FROM public.role_profiles
WHERE candidate_id = @id
ORDER BY created_at, id;

-- name: ExportApplications :many
SELECT a.id, jp.title, a.status::text AS status, a.reject_reason, a.interview_deadline, a.created_at, a.updated_at, a.cv_path
FROM public.applications a
JOIN public.job_postings jp ON jp.id = a.job_posting_id
WHERE a.candidate_id = @id
ORDER BY a.created_at, a.id;

-- A recording key is set only for a recording whose upload finished (record_status ready).
-- name: ExportInterviews :many
SELECT i.id, i.type::text AS type, i.application_id, i.started_at, i.completed_at, i.end_reason,
       i.score, i.score_outcome, i.feedback,
       CASE WHEN i.record_status = 'ready' THEN i.record_path END AS record_key
FROM public.interviews i
LEFT JOIN public.applications a ON a.id = i.application_id
LEFT JOIN public.role_profiles rp ON rp.id = i.role_profile_id
WHERE a.candidate_id = @id OR rp.candidate_id = @id
ORDER BY i.started_at, i.id;

-- name: ExportTurns :many
SELECT position, kind, question, answer, feedback, asked_at, answered_at
FROM public.conversation_turns
WHERE interview_id = @interview_id
ORDER BY position;

-- name: ExportScores :many
SELECT m.name AS metric, s.score, s.weight, s.evidence
FROM public.score_details s
JOIN public.metrics m ON m.id = s.metric_id
WHERE s.interview_id = @interview_id
ORDER BY m.name;

-- The rubric of a practice question is a system template, so its criterion names are not anyone's private
-- text; expected answers and scoring guides are never selected.
-- name: ExportCriteria :many
SELECT s.question_position, c.name AS criterion, m.name AS metric, s.points, c.max_points, s.evidence, s.feedback
FROM public.interview_criterion_scores s
JOIN public.rubric_criteria c ON c.id = s.rubric_criterion_id
JOIN public.metrics m ON m.id = c.metric_id
WHERE s.interview_id = @interview_id
ORDER BY s.question_position, c.position;

-- name: ExportPostings :many
SELECT id, title, status::text AS status, location, employment_type, salary_min, salary_max, created_at, published_at
FROM public.job_postings
WHERE recruiter_id = @id
ORDER BY created_at, id;
