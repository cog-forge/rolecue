-- Interview capacity of a Job Posting. The posting itself is read and locked with LockOwnedPosting and
-- its CV approvals with CountCVApprovals (application.sql); the queries here cover what else the
-- capacity state needs.

-- consumed counts the interviews that used a slot: one row per Application that has an interview
-- (interviews.application_id is UNIQUE, so a resumed session counts once). Submitted Applications
-- never count.
-- name: CountConsumedInterviews :one
SELECT count(*)::bigint AS consumed
FROM public.interviews i
JOIN public.applications a ON a.id = i.application_id
WHERE a.job_posting_id = @job_posting_id;
