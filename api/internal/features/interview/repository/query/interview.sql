-- Interview sessions of the Candidate. A practice session belongs to the Candidate through its role profile
-- and a recruitment session (no role profile) through its Application, so every query that reads or
-- writes one session resolves the owner as COALESCE(rp.candidate_id, a.candidate_id), and another
-- Candidate's session is not found.

-- name: GetOwnedRoleProfile :one
SELECT id, jd_id, status
FROM public.role_profiles
WHERE id = @id AND candidate_id = @candidate_id;

-- A random sample of a Job Description's bank, at most @question_limit questions (Decision 14). The
-- session stores the sample in interview_questions in this order when it starts, and every later read
-- goes through those rows, so Get and Resume always return the same questions in the same order. A bank
-- is small, so sorting it by random() is cheap.
-- name: ListStartQuestions :many
SELECT id, content
FROM public.core_questions
WHERE jd_id = @jd_id
ORDER BY random()
LIMIT @question_limit;

-- A practice session may use only an avatar in the Candidate's own inventory (no system avatars exist),
-- and only an active voice. Another user's personal avatar is their likeness and is never usable.
-- name: AvatarOwnedBy :one
SELECT EXISTS (
    SELECT 1 FROM public.avatars av
    JOIN public.inventories inv ON inv.id = av.inventory_id
    WHERE av.id = @id AND inv.user_id = @user_id
)::boolean AS found;

-- name: VoiceActive :one
SELECT EXISTS (SELECT 1 FROM public.voices WHERE id = @id AND is_active)::boolean AS found;

-- Serializes the practice starts of one Candidate for one role profile (the key is the two ids), so two
-- concurrent starts cannot both find "no unfinished session" and both charge.
-- name: LockPracticeStart :exec
SELECT pg_advisory_xact_lock(hashtextextended(@lock_key::text, 0));

-- The unfinished practice session of the Candidate for a role profile, if any: resuming it is free.
-- name: FindUnfinishedPractice :one
SELECT i.id
FROM public.interviews i
WHERE i.type = 'practice' AND i.role_profile_id = @role_profile_id AND i.completed_at IS NULL
ORDER BY i.started_at DESC, i.id DESC
LIMIT 1;

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

-- The question at one position of a session: the text the interviewer spoke once it was delivered live,
-- otherwise the bank text as it is now. asked_at is NULL until the live delivery.
-- name: GetInterviewQuestion :one
SELECT iq.position, COALESCE(iq.spoken_text, cq.content)::text AS content, iq.asked_at
FROM public.interview_questions iq
JOIN public.core_questions cq ON cq.id = iq.core_question_id
WHERE iq.interview_id = @interview_id AND iq.position = @position;

-- Freezes the question text and the time of its first live delivery. A repeat delivery (a reconnect)
-- keeps the first text and time, so the stored turn always holds what was actually asked.
-- name: MarkInterviewQuestionAsked :one
UPDATE public.interview_questions iq
SET spoken_text = COALESCE(iq.spoken_text, cq.content),
    asked_at = COALESCE(iq.asked_at, statement_timestamp())
FROM public.core_questions cq
WHERE cq.id = iq.core_question_id AND iq.interview_id = @interview_id AND iq.position = @position
RETURNING iq.spoken_text::text AS spoken_text, iq.asked_at;

-- Freezes the provider voice of a session on its first live delivery: the session voice's provider id,
-- or the configured default when the session has none. Once set it never changes, so a later catalog
-- change does not rewrite how the session was spoken. It stays NULL when neither exists.
-- name: FreezeInterviewVoice :one
UPDATE public.interviews i
SET provider_voice_id = COALESCE(
        i.provider_voice_id,
        (SELECT v.provider_voice_id FROM public.voices v WHERE v.id = i.voice_id),
        NULLIF(@default_provider_voice_id::text, ''))
WHERE i.id = @id
RETURNING i.provider_voice_id;

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
LEFT JOIN public.role_profiles rp ON rp.id = i.role_profile_id
LEFT JOIN public.applications a ON a.id = i.application_id
WHERE i.id = @id AND COALESCE(rp.candidate_id, a.candidate_id) = @candidate_id
FOR UPDATE OF i;

-- name: GetOwnedInterview :one
SELECT i.id, i.type::text AS type, i.role_profile_id, i.application_id, i.avatar_id, i.voice_id,
       i.started_at, i.completed_at, i.evaluation_status
FROM public.interviews i
LEFT JOIN public.role_profiles rp ON rp.id = i.role_profile_id
LEFT JOIN public.applications a ON a.id = i.application_id
WHERE i.id = @id AND COALESCE(rp.candidate_id, a.candidate_id) = @candidate_id;

-- name: ListInterviewTurns :many
SELECT position, question, COALESCE(answer, '')::text AS answer
FROM public.conversation_turns
WHERE interview_id = @interview_id
ORDER BY position;

-- A core turn. asked_at is the live delivery time of its question (NULL for an answer to a question that
-- was never delivered live); answered_at is the statement time.
-- name: InsertInterviewTurn :exec
INSERT INTO public.conversation_turns (interview_id, position, question, answer, kind, asked_at, answered_at)
VALUES (@interview_id, @position, @question, @answer, 'core', sqlc.narg(asked_at), statement_timestamp());

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

-- Ends an evaluation whose run panicked, so the next Complete claims and retries it instead of waiting
-- for the claim to go stale. It is the same guard as MarkInterviewEvaluationFailed in the evaluation
-- feature (interview_evaluation.sql), copied here so this feature does not import it: a completed
-- evaluation is never turned back into a failed one.
-- name: MarkEvaluationFailed :exec
UPDATE public.interviews
SET evaluation_status = 'failed'
WHERE id = @id AND evaluation_status <> 'completed';
