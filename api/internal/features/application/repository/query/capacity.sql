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

-- Funding writes the slot once: the caller holds the posting lock and has checked that the posting is
-- open and unfunded, and the IS NULL guard makes a second write a no-op.
-- name: SetInterviewSlot :execrows
UPDATE public.job_postings
SET interview_slot = @interview_slot
WHERE id = @id AND interview_slot IS NULL;
