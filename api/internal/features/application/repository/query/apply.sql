-- Candidate side of Applications: applying to an open Job Posting and reading the Candidate's own
-- Applications. Every query filters by the Candidate, so another Candidate's Application is not found.

-- Serializes applications of one Candidate to one posting (KTD6). The key is hashed, so the lock
-- needs no migration. It is released at commit or rollback.
-- name: LockApplyKey :exec
SELECT pg_advisory_xact_lock(hashtextextended(@lock_key::text, 0));

-- FOR SHARE conflicts with the NO KEY UPDATE lock that Close Intake and screening take, so a submission
-- waits for an in-flight close and then sees intake_closed. A weaker lock would let an Application slip
-- in beside the close, survive its pending rejection, and keep the posting from ever closing. The
-- recruiter lock check mirrors the job board: a locked Recruiter's posting is hidden, not closed.
-- funded mirrors it too: an open posting without interview slots does not take Applications yet.
-- name: LockApplyPosting :one
SELECT jp.status::text AS status,
       (jp.interview_slot IS NOT NULL)::boolean AS funded,
       (u.is_locked IS TRUE AND (u.lock_expires_at IS NULL OR u.lock_expires_at > now()))::boolean AS recruiter_locked
FROM public.job_postings jp
JOIN public.users u ON u.id = jp.recruiter_id
WHERE jp.id = @id
FOR SHARE OF jp;

-- The same state without a lock, read before the CV is uploaded so an unfunded posting is refused
-- without storing anything. The insert re-checks it under the lock.
-- name: GetApplyPostingState :one
SELECT jp.status::text AS status,
       (jp.interview_slot IS NOT NULL)::boolean AS funded,
       (u.is_locked IS TRUE AND (u.lock_expires_at IS NULL OR u.lock_expires_at > now()))::boolean AS recruiter_locked
FROM public.job_postings jp
JOIN public.users u ON u.id = jp.recruiter_id
WHERE jp.id = @id;

-- name: CandidateHasApplication :one
SELECT EXISTS (
    SELECT 1 FROM public.applications
    WHERE job_posting_id = @job_posting_id AND candidate_id = @candidate_id
)::boolean AS applied;

-- New Applications start pending. cv_path is the storage key of the uploaded PDF.
-- name: InsertCandidateApplication :one
INSERT INTO public.applications (job_posting_id, candidate_id, status, cv_path)
VALUES (@job_posting_id, @candidate_id, 'pending', @cv_path)
RETURNING id, job_posting_id, status::text AS status, created_at;

-- The Candidate's own Applications, newest first. cv_path is not selected. The interview columns carry the
-- session id, completion and score only: no transcript, recording or feedback text.
-- name: ListCandidateApplications :many
SELECT a.id, a.job_posting_id, jp.title AS job_posting_title, a.status::text AS status,
       a.reject_reason, a.interview_deadline, a.created_at,
       i.id AS interview_id, i.completed_at AS interview_completed_at,
       i.evaluation_status AS interview_evaluation_status, i.score AS interview_score
FROM public.applications a
JOIN public.job_postings jp ON jp.id = a.job_posting_id
LEFT JOIN public.interviews i ON i.application_id = a.id
WHERE a.candidate_id = @candidate_id
ORDER BY a.created_at DESC, a.id DESC
LIMIT @page_limit OFFSET @page_offset;

-- name: CountCandidateApplications :one
SELECT count(*) FROM public.applications WHERE candidate_id = @candidate_id;

-- name: GetCandidateApplication :one
SELECT a.id, a.job_posting_id, jp.title AS job_posting_title, a.status::text AS status,
       a.reject_reason, a.interview_deadline, a.created_at,
       i.id AS interview_id, i.completed_at AS interview_completed_at,
       i.evaluation_status AS interview_evaluation_status, i.score AS interview_score
FROM public.applications a
JOIN public.job_postings jp ON jp.id = a.job_posting_id
LEFT JOIN public.interviews i ON i.application_id = a.id
WHERE a.id = @id AND a.candidate_id = @candidate_id;
