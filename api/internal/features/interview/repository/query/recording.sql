-- Recruitment recordings and the Recruiter's evidence reads. A Candidate owns an interview through
-- role_profiles.candidate_id, the same rule as the session queries. A Recruiter reaches an Application
-- only through a Job Posting they own (job_postings.recruiter_id). Anything else is not found.

-- name: GetCandidateRecordingState :one
SELECT i.id, i.type::text AS type, i.record_status, i.record_path,
       (i.completed_at IS NOT NULL)::boolean AS completed
FROM public.interviews i
LEFT JOIN public.role_profiles rp ON rp.id = i.role_profile_id
LEFT JOIN public.applications a ON a.id = i.application_id
WHERE i.id = @id AND COALESCE(rp.candidate_id, a.candidate_id) = @candidate_id;

-- The upload grant is one conditional UPDATE. A recording that is processing or ready is never replaced
-- by a new upload, so the status checked in the service cannot change under the write.
-- name: SetRecordingUpload :execrows
UPDATE public.interviews i
SET record_path = @record_path, record_status = 'uploading'
WHERE i.id = @id
  AND i.type = 'recruitment'
  AND i.record_status IN ('none', 'uploading', 'failed')
  AND i.application_id IN (SELECT a.id FROM public.applications a WHERE a.candidate_id = @candidate_id);

-- Finalization moves only the upload that was verified: the status and the key must still be the ones
-- the Candidate uploaded to, so a newer grant is never overwritten by an older check.
-- name: SetRecordingResult :execrows
UPDATE public.interviews i
SET record_status = @record_status
WHERE i.id = @id
  AND i.record_path = @record_path
  AND i.record_status = 'uploading'
  AND i.application_id IN (SELECT a.id FROM public.applications a WHERE a.candidate_id = @candidate_id);

-- name: RecruiterOwnsApplication :one
SELECT EXISTS (
    SELECT 1
    FROM public.applications a
    JOIN public.job_postings jp ON jp.id = a.job_posting_id
    WHERE a.id = @id AND a.job_posting_id = @job_posting_id AND jp.recruiter_id = @recruiter_id
)::boolean AS owned;

-- The interview of an Application, if one was started. Practice sessions never carry an application_id.
-- name: GetApplicationRecordingState :one
SELECT i.id, i.type::text AS type, i.record_status, i.record_path,
       (i.completed_at IS NOT NULL)::boolean AS completed
FROM public.interviews i
WHERE i.application_id = @application_id;

-- name: ListRecruitmentTurns :many
SELECT position,
       (count(*) FILTER (WHERE kind = 'core') OVER (ORDER BY position))::int AS question_number,
       kind, question, answer
FROM public.conversation_turns
WHERE interview_id = @interview_id
ORDER BY position;

-- Recording upkeep of the System Handler (KAN-272). Practice interviews never have a recording and are
-- never touched: every query below is restricted to recruitment interviews.

-- Marks failed the recording of a completed recruitment interview that never started uploading
-- (record_status none since before @none_before) or whose upload never finished (uploading since before
-- @uploading_before, past the upload grant's lifetime). Both are measured from the completion: there is no
-- column for the time of the grant. A failed recording can still take a new upload grant, so a late
-- upload from the Candidate's browser is not lost. The previous status is returned for the log.
-- name: FailStaleRecordings :many
WITH stale AS (
    SELECT i.id, i.record_status
    FROM public.interviews i
    WHERE i.type = 'recruitment'
      AND i.completed_at IS NOT NULL
      AND ((i.record_status = 'none' AND i.completed_at < @none_before::timestamptz)
           OR (i.record_status = 'uploading' AND i.completed_at < @uploading_before::timestamptz))
    FOR UPDATE SKIP LOCKED
)
UPDATE public.interviews i
SET record_status = 'failed'
FROM stale
WHERE i.id = stale.id AND i.record_status = stale.record_status
RETURNING i.id, stale.record_status AS previous_status;

-- Recordings past retention: a stored object of a recruitment interview whose Application reached its
-- final decision before @decided_before. applications has no decision timestamp; updated_at is the time
-- of the final decision because a terminal Application (approved or rejected) is never updated again.
-- Undecided Applications are never listed.
-- name: ListExpiredRecordings :many
SELECT i.id, i.record_path::text AS record_path
FROM public.interviews i
JOIN public.applications a ON a.id = i.application_id
WHERE i.type = 'recruitment'
  AND i.record_path IS NOT NULL
  AND a.status IN ('approved', 'rejected')
  AND a.updated_at < @decided_before::timestamptz
ORDER BY a.updated_at, i.id
LIMIT @batch_size;

-- Clears a purged recording: the key is gone and the status says it was deleted after retention. The key
-- must still be the one whose object was deleted.
-- name: ClearDeletedRecording :execrows
UPDATE public.interviews
SET record_path = NULL, record_status = 'deleted'
WHERE id = @id AND type = 'recruitment' AND record_path = @record_path;
