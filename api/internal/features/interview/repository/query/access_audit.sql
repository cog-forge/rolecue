-- Audit of Recruiter reads of recordings and transcripts (KAN-296). Rows are only inserted by a
-- successful read and only read by the Admin list.

-- name: InsertRecruiterAccess :exec
INSERT INTO public.recruiter_access_audit (recruiter_id, application_id, interview_id, action)
VALUES (@recruiter_id, @application_id, @interview_id, @action);

-- name: ListRecruiterAccessAudit :many
SELECT id, recruiter_id, application_id, interview_id, action, created_at
FROM public.recruiter_access_audit
WHERE (sqlc.narg(recruiter_id)::uuid IS NULL OR recruiter_id = sqlc.narg(recruiter_id)::uuid)
  AND (sqlc.narg(application_id)::uuid IS NULL OR application_id = sqlc.narg(application_id)::uuid)
ORDER BY created_at DESC, id DESC
LIMIT @page_size OFFSET @page_offset;

-- name: CountRecruiterAccessAudit :one
SELECT count(*)::bigint AS total
FROM public.recruiter_access_audit
WHERE (sqlc.narg(recruiter_id)::uuid IS NULL OR recruiter_id = sqlc.narg(recruiter_id)::uuid)
  AND (sqlc.narg(application_id)::uuid IS NULL OR application_id = sqlc.narg(application_id)::uuid);
