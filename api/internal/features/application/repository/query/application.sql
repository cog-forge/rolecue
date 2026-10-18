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
