-- Admin overview aggregates. Every query is read-only and returns one row of counts, so a status
-- that has no rows still comes back as 0. The window is [from, to): from is inclusive, to exclusive.

-- name: CountOverviewAccounts :one
-- Admin accounts are excluded on purpose. locked means is_locked and the lock has not expired yet
-- (a NULL lock_expires_at never expires); @window_to is the same clock the window ends at.
SELECT
    count(*) FILTER (WHERE role = 'candidate') AS candidates,
    count(*) FILTER (WHERE role = 'recruiter') AS recruiters,
    count(*) FILTER (
        WHERE is_locked AND (lock_expires_at IS NULL OR lock_expires_at > @window_to)
    ) AS locked,
    count(*) FILTER (WHERE created_at >= @window_from AND created_at < @window_to) AS new_in_window
FROM public.users
WHERE role IN ('candidate', 'recruiter');

-- name: CountOverviewJobPostings :one
-- All-time counts by status.
SELECT
    count(*) FILTER (WHERE status = 'pending') AS pending,
    count(*) FILTER (WHERE status = 'open') AS open,
    count(*) FILTER (WHERE status = 'intake_closed') AS intake_closed,
    count(*) FILTER (WHERE status = 'closed') AS closed,
    count(*) FILTER (WHERE status = 'rejected') AS rejected
FROM public.job_postings;

-- name: CountOverviewApplications :one
-- Status counts are all-time; submitted_in_window counts applications created inside the window.
SELECT
    count(*) FILTER (WHERE status = 'pending') AS pending,
    count(*) FILTER (WHERE status = 'interview_eligible') AS interview_eligible,
    count(*) FILTER (WHERE status = 'interviewed') AS interviewed,
    count(*) FILTER (WHERE status = 'approved') AS approved,
    count(*) FILTER (WHERE status = 'rejected') AS rejected,
    count(*) FILTER (WHERE created_at >= @window_from AND created_at < @window_to) AS submitted_in_window
FROM public.applications;

-- name: CountOverviewInterviews :one
-- *_started_in_window use started_at by type; completed_in_window uses completed_at.
-- evaluation_failed is all-time.
SELECT
    count(*) FILTER (
        WHERE type = 'practice' AND started_at >= @window_from AND started_at < @window_to
    ) AS practice_started_in_window,
    count(*) FILTER (
        WHERE type = 'recruitment' AND started_at >= @window_from AND started_at < @window_to
    ) AS recruitment_started_in_window,
    count(*) FILTER (WHERE completed_at >= @window_from AND completed_at < @window_to) AS completed_in_window,
    count(*) FILTER (WHERE evaluation_status = 'failed') AS evaluation_failed
FROM public.interviews;
