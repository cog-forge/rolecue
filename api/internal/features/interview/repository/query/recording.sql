-- Recruitment recordings and the Recruiter's evidence reads. A Candidate owns an interview through
-- role_profiles.candidate_id, the same rule as the session queries. A Recruiter reaches an Application
-- only through a Job Posting they own (job_postings.recruiter_id). Anything else is not found.

-- name: GetCandidateRecordingState :one
SELECT i.id, i.type::text AS type, i.record_status, i.record_path,
       (i.completed_at IS NOT NULL)::boolean AS completed
FROM public.interviews i
JOIN public.role_profiles rp ON rp.id = i.role_profile_id
WHERE i.id = @id AND rp.candidate_id = @candidate_id;

-- The upload grant is one conditional UPDATE. A recording that is processing or ready is never replaced
-- by a new upload, so the status checked in the service cannot change under the write.
-- name: SetRecordingUpload :execrows
UPDATE public.interviews i
SET record_path = @record_path, record_status = 'uploading'
WHERE i.id = @id
  AND i.type = 'recruitment'
  AND i.record_status IN ('none', 'uploading', 'failed')
  AND i.role_profile_id IN (SELECT rp.id FROM public.role_profiles rp WHERE rp.candidate_id = @candidate_id);

-- Finalization moves only the upload that was verified: the status and the key must still be the ones
-- the Candidate uploaded to, so a newer grant is never overwritten by an older check.
-- name: SetRecordingResult :execrows
UPDATE public.interviews i
SET record_status = @record_status
WHERE i.id = @id
  AND i.record_path = @record_path
  AND i.record_status = 'uploading'
  AND i.role_profile_id IN (SELECT rp.id FROM public.role_profiles rp WHERE rp.candidate_id = @candidate_id);

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
SELECT position, question, answer
FROM public.conversation_turns
WHERE interview_id = @interview_id
ORDER BY position;
