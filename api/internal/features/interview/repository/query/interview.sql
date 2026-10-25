-- Interview sessions of the Candidate. A session belongs to the Candidate through its role profile, so
-- every query that reads or writes one session joins role_profiles, and another Candidate's session is
-- not found.

-- name: GetOwnedRoleProfile :one
SELECT id, jd_id, status
FROM public.role_profiles
WHERE id = @id AND candidate_id = @candidate_id;

-- The most recent confirmed role profile of the Candidate, the default for a recruitment session.
-- name: LatestConfirmedRoleProfile :one
SELECT id
FROM public.role_profiles
WHERE candidate_id = @candidate_id AND status = 'confirmed'
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- The first core questions of a Job Description's bank, in a fixed order so a session always asks the
-- same questions.
-- name: ListStartQuestions :many
SELECT id, content
FROM public.core_questions
WHERE jd_id = @jd_id
ORDER BY id
LIMIT @question_limit;

-- name: AvatarExists :one
SELECT EXISTS (SELECT 1 FROM public.avatars WHERE id = @id)::boolean AS found;

-- name: VoiceExists :one
SELECT EXISTS (SELECT 1 FROM public.voices WHERE id = @id)::boolean AS found;

-- name: InsertInterview :one
INSERT INTO public.interviews (role_profile_id, type, application_id, avatar_id, voice_id, start_key)
VALUES (@role_profile_id, sqlc.arg(interview_type)::text::public.interview_type,
        sqlc.narg(application_id), sqlc.narg(avatar_id), sqlc.narg(voice_id), sqlc.narg(start_key))
RETURNING id;

-- The session a practice start key already created, if any.
-- name: FindInterviewByStartKey :one
SELECT id FROM public.interviews WHERE start_key = @start_key;

-- name: InsertInterviewQuestion :exec
INSERT INTO public.interview_questions (interview_id, position, core_question_id)
VALUES (@interview_id, @position, @core_question_id);

-- The question at one position of a session, with the bank text as it is now.
-- name: GetInterviewQuestion :one
SELECT iq.position, cq.content
FROM public.interview_questions iq
JOIN public.core_questions cq ON cq.id = iq.core_question_id
WHERE iq.interview_id = @interview_id AND iq.position = @position;

-- name: CountInterviewQuestions :one
SELECT count(*)::bigint AS questions
FROM public.interview_questions
WHERE interview_id = @interview_id;

-- Finds the posting of an Application the Candidate owns, without a lock. The posting row is locked next,
-- before the Application is read again under that lock.
-- name: GetApplicationPosting :one
SELECT job_posting_id
FROM public.applications
WHERE id = @id AND candidate_id = @candidate_id;

-- Locks the posting first, as every interview start does, so concurrent starts for one posting queue up.
-- name: LockPostingForInterview :one
SELECT jd_id, interview_slot, required_avatar_id, required_voice_id
FROM public.job_postings
WHERE id = @id
FOR UPDATE;

-- The Application as it is under the posting lock. within_deadline uses the database clock, the same
-- clock that wrote interview_deadline on CV approval. It reads statement_timestamp(), not now(): now()
-- is the transaction start, which is before the wait for the posting lock, so a start that queued behind
-- a long transaction would otherwise be judged against a stale time.
-- name: GetApplicationForInterview :one
SELECT status::text AS status,
       COALESCE(interview_deadline > statement_timestamp(), false)::boolean AS within_deadline
FROM public.applications
WHERE id = @id AND candidate_id = @candidate_id AND job_posting_id = @job_posting_id;

-- name: FindInterviewByApplication :one
SELECT id
FROM public.interviews
WHERE application_id = @application_id;

-- consumed counts the interviews that used a slot of the posting. It is the same query as the capacity
-- count in the application feature (capacity.sql), copied here so this feature does not import it.
-- name: CountConsumedInterviews :one
SELECT count(*)::bigint AS consumed
FROM public.interviews i
JOIN public.applications a ON a.id = i.application_id
WHERE a.job_posting_id = @job_posting_id;

-- name: LockOwnedInterview :one
SELECT i.id
FROM public.interviews i
JOIN public.role_profiles rp ON rp.id = i.role_profile_id
WHERE i.id = @id AND rp.candidate_id = @candidate_id
FOR UPDATE OF i;

-- name: GetOwnedInterview :one
SELECT i.id, i.type::text AS type, i.role_profile_id, i.application_id, i.avatar_id, i.voice_id,
       i.started_at, i.completed_at, i.evaluation_status
FROM public.interviews i
JOIN public.role_profiles rp ON rp.id = i.role_profile_id
WHERE i.id = @id AND rp.candidate_id = @candidate_id;

-- name: ListInterviewTurns :many
SELECT position, question, COALESCE(answer, '')::text AS answer
FROM public.conversation_turns
WHERE interview_id = @interview_id
ORDER BY position;

-- name: InsertInterviewTurn :exec
INSERT INTO public.conversation_turns (interview_id, position, question, answer)
VALUES (@interview_id, @position, @question, @answer);

-- The guard keeps the first completion's time; a repeat changes nothing.
-- name: SetInterviewCompleted :exec
UPDATE public.interviews
SET completed_at = statement_timestamp()
WHERE id = @id AND completed_at IS NULL;

-- Only an Application still waiting for its interview moves on. An Application already interviewed,
-- or one the Recruiter decided meanwhile, is left as it is.
-- name: MarkApplicationInterviewed :exec
UPDATE public.applications
SET status = 'interviewed', updated_at = statement_timestamp()
WHERE id = @id AND status = 'interview_eligible';

-- An Application takes answers and a completion only while it still waits for its interview and the
-- window is open. The statement clock matches the start check and the expiry pass.
-- name: ApplicationAcceptsInterview :one
SELECT (status = 'interview_eligible' AND interview_deadline > statement_timestamp())::boolean AS open
FROM public.applications
WHERE id = @id
FOR SHARE;

-- One run owns an evaluation at a time. A completed interview is claimed when its evaluation failed, or
-- when it is pending and was never claimed or its last claim is older than @stale_seconds (the run died
-- with its process). Exactly one concurrent caller gets a row back and starts the evaluation.
-- name: ClaimInterviewEvaluation :execrows
UPDATE public.interviews
SET evaluation_status = 'pending', evaluation_claimed_at = statement_timestamp()
WHERE id = @id
  AND completed_at IS NOT NULL
  AND (evaluation_status = 'failed'
       OR (evaluation_status = 'pending'
           AND (evaluation_claimed_at IS NULL
                OR evaluation_claimed_at < statement_timestamp() - make_interval(secs => @stale_seconds::float8))));
