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
    count(*) FILTER (WHERE created_at >= @window_from AND created_at < @window_to) AS new_in_window,
    count(*) FILTER (WHERE created_at >= @previous_from AND created_at < @window_from) AS new_in_previous_window
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
    count(*) FILTER (WHERE created_at >= @window_from AND created_at < @window_to) AS submitted_in_window,
    count(*) FILTER (WHERE created_at >= @previous_from AND created_at < @window_from) AS submitted_in_previous_window
FROM public.applications;

-- name: CountOverviewInterviews :one
-- Start counts come from the activity series. Evaluation failures count current rows in every period.
SELECT
    count(*) FILTER (WHERE completed_at >= @window_from AND completed_at < @window_to) AS completed_in_window,
    count(*) FILTER (WHERE evaluation_status = 'failed') AS evaluation_failed
FROM public.interviews;

-- name: CountOverviewInterviewActivity :many
-- Each reporting window has its own origin. Both windows use half-open bounds.
WITH starts AS (
    SELECT type, started_at >= @window_from::timestamptz AS is_current,
           CASE WHEN started_at >= @window_from::timestamptz
             THEN started_at - @window_from::timestamptz
             ELSE started_at - @previous_from::timestamptz
           END AS elapsed
    FROM public.interviews
    WHERE started_at >= @previous_from::timestamptz AND started_at < @window_to::timestamptz
)
SELECT
    floor(extract(epoch FROM elapsed) / @bucket_seconds::int)::int AS bucket_index,
    count(*) FILTER (WHERE is_current AND type = 'practice') AS practice,
    count(*) FILTER (WHERE is_current AND type = 'recruitment') AS recruitment,
    count(*) FILTER (WHERE NOT is_current AND type = 'practice') AS previous_practice,
    count(*) FILTER (WHERE NOT is_current AND type = 'recruitment') AS previous_recruitment
FROM starts
GROUP BY bucket_index
ORDER BY bucket_index;
