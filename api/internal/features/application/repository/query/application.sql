-- CV screening of Applications. The Recruiter reaches an Application only through a Job Posting
-- they own (job_postings.recruiter_id); someone else's posting is not found.

-- The row lock serializes CV decisions on one posting, so two approvals at once cannot both pass the
-- interview_slot check. Applications are counted by a later statement, after the lock is held.
-- NO KEY UPDATE still conflicts with the FOR UPDATE of posting edits, but not with the key-share
-- lock that inserting an Application takes, so submissions are not held up by screening.
-- name: LockOwnedPosting :one
SELECT status::text AS status, interview_slot
FROM public.job_postings
WHERE id = @id AND recruiter_id = @recruiter_id
FOR NO KEY UPDATE;

-- interview_deadline is set only by CV approval, so it marks every Application ever approved.
-- name: CountCVApprovals :one
SELECT count(*) FROM public.applications
WHERE job_posting_id = @job_posting_id AND interview_deadline IS NOT NULL;

-- statement_timestamp(), not now(): now() is the transaction start, before any wait for the posting
-- lock, and would shorten the Candidate's 24 hours by that wait.
-- name: ApproveCV :one
UPDATE public.applications
SET status = 'interview_eligible', interview_deadline = statement_timestamp() + interval '24 hours',
    updated_at = statement_timestamp()
WHERE id = @id AND job_posting_id = @job_posting_id AND status = 'pending'
RETURNING id, job_posting_id, status::text AS status, interview_deadline, reject_reason, updated_at;

-- name: RejectCV :one
UPDATE public.applications
SET status = 'rejected', reject_reason = sqlc.narg(reject_reason), updated_at = statement_timestamp()
WHERE id = @id AND job_posting_id = @job_posting_id AND status = 'pending'
RETURNING id, job_posting_id, status::text AS status, interview_deadline, reject_reason, updated_at;

-- Checked before the capacity rule, so a missing or already screened Application gets its own
-- error even when the posting is full.
-- name: LockApplication :one
SELECT status::text AS status
FROM public.applications
WHERE id = @id AND job_posting_id = @job_posting_id
FOR UPDATE;

-- name: GetCandidateEligibility :one
SELECT id, job_posting_id, status::text AS status, interview_deadline,
       COALESCE(interview_deadline > now(), false)::boolean AS within_deadline
FROM public.applications
WHERE id = @id AND candidate_id = @candidate_id;

-- The Recruiter's Application list. It joins job_postings, so an Application is reachable only
-- through a posting the caller owns. No column here points at the CV file, a transcript or a
-- recording: evidence is reached through Application Detail only.
-- Every filter is optional (a NULL argument turns it off). search is a LIKE pattern the service
-- has already escaped.
-- name: ListRecruiterApplications :many
SELECT a.id, a.job_posting_id, jp.title AS job_posting_title, a.status::text AS status, a.created_at,
       u.id AS candidate_id, u.name AS candidate_name, u.email AS candidate_email, u.image AS candidate_image,
       COALESCE(a.status = 'interview_eligible' AND a.interview_deadline > now(), false)::boolean AS interview_eligible,
       a.interview_deadline, a.reject_reason, a.updated_at
FROM public.applications a
JOIN public.job_postings jp ON jp.id = a.job_posting_id
JOIN public.users u ON u.id = a.candidate_id
WHERE jp.recruiter_id = @recruiter_id
  AND (sqlc.narg(job_posting_id)::uuid IS NULL OR a.job_posting_id = sqlc.narg(job_posting_id)::uuid)
  AND (sqlc.narg(status)::text IS NULL OR a.status::text = sqlc.narg(status)::text)
  AND (sqlc.narg(search)::text IS NULL
       OR u.name ILIKE '%' || sqlc.narg(search)::text || '%'
       OR u.email ILIKE '%' || sqlc.narg(search)::text || '%')
ORDER BY a.created_at DESC, a.id DESC
LIMIT @page_limit OFFSET @page_offset;

-- Same joins and filters as the list, so a row the list cannot show is not counted either.
-- name: CountRecruiterApplications :one
SELECT count(*)
FROM public.applications a
JOIN public.job_postings jp ON jp.id = a.job_posting_id
JOIN public.users u ON u.id = a.candidate_id
WHERE jp.recruiter_id = @recruiter_id
  AND (sqlc.narg(job_posting_id)::uuid IS NULL OR a.job_posting_id = sqlc.narg(job_posting_id)::uuid)
  AND (sqlc.narg(status)::text IS NULL OR a.status::text = sqlc.narg(status)::text)
  AND (sqlc.narg(search)::text IS NULL
       OR u.name ILIKE '%' || sqlc.narg(search)::text || '%'
       OR u.email ILIKE '%' || sqlc.narg(search)::text || '%');

-- One Application for Application Detail: the list columns, the CV storage key (cv_path, read by the
-- service and never returned), the posting status and capacity, and how many Applications of the
-- posting were ever CV-approved (the same definition as CountCVApprovals). Ownership is part of the
-- filter; the caller separates "not yours" from "not on this posting" with IsPostingOwned.
-- name: GetRecruiterApplication :one
SELECT a.id, a.job_posting_id, jp.title AS job_posting_title, a.status::text AS status, a.created_at,
       u.id AS candidate_id, u.name AS candidate_name, u.email AS candidate_email, u.image AS candidate_image,
       COALESCE(a.status = 'interview_eligible' AND a.interview_deadline > now(), false)::boolean AS interview_eligible,
       a.interview_deadline, a.reject_reason, a.updated_at,
       a.cv_path,
       jp.status::text AS posting_status, jp.interview_slot,
       (SELECT count(*) FROM public.applications c
        WHERE c.job_posting_id = jp.id AND c.interview_deadline IS NOT NULL)::bigint AS cv_approvals
FROM public.applications a
JOIN public.job_postings jp ON jp.id = a.job_posting_id
JOIN public.users u ON u.id = a.candidate_id
WHERE a.id = @id AND a.job_posting_id = @job_posting_id AND jp.recruiter_id = @recruiter_id;

-- Lets a list of one posting tell "not yours" (404) from "no Applications yet" ([]).
-- name: IsPostingOwned :one
SELECT EXISTS (
    SELECT 1 FROM public.job_postings WHERE id = @id AND recruiter_id = @recruiter_id
)::boolean AS owned;

-- Intake closes only from open. The caller holds the posting lock (LockOwnedPosting or
-- LockPostingByID), so the guard reads the status that the lock protects.
-- name: SetPostingIntakeClosed :execrows
UPDATE public.job_postings
SET status = 'intake_closed'
WHERE id = @id AND status = 'open';

-- Every pending Application of the posting is rejected when intake closes, in the same transaction
-- as the posting status change. statement_timestamp() for the same reason as ApproveCV.
-- name: RejectPendingApplications :execrows
UPDATE public.applications
SET status = 'rejected', reject_reason = @reject_reason, updated_at = statement_timestamp()
WHERE job_posting_id = @job_posting_id AND status = 'pending';

-- total counts every Application of the posting; non_terminal counts the ones that can still move
-- (pending, interview_eligible, interviewed). The terminal close needs total >= 1 and non_terminal = 0.
-- name: CountApplicationsForClose :one
SELECT count(*) AS total,
       count(*) FILTER (WHERE status IN ('pending', 'interview_eligible', 'interviewed'))::bigint AS non_terminal
FROM public.applications
WHERE job_posting_id = @job_posting_id;

-- Locks the posting without the owner filter, for system callers that have no Recruiter.
-- name: LockPostingByID :one
SELECT status::text AS status
FROM public.job_postings
WHERE id = @id
FOR NO KEY UPDATE;

-- The terminal close: only from intake_closed, so a repeat or an open posting changes nothing.
-- name: SetPostingClosed :execrows
UPDATE public.job_postings
SET status = 'closed'
WHERE id = @id AND status = 'intake_closed';
