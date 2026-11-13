-- Interview sessions of the Candidate. A practice session belongs to the Candidate through its role profile
-- and a recruitment session (no role profile) through its Application, so every query that reads or
-- writes one session resolves the owner as COALESCE(rp.candidate_id, a.candidate_id), and another
-- Candidate's session is not found.

-- name: GetOwnedRoleProfile :one
SELECT id, jd_id, status
FROM public.role_profiles
WHERE id = @id AND candidate_id = @candidate_id;

-- The bank a session samples from (Decision 14, KAN-333): every core question of the Job Description with
-- the distinct metrics its rubric has criteria for. metric_ids is empty for a question without a rubric
-- and for a rubric without criteria. It joins the rubric as it is, whether or not the rubric is active: a
-- question keeps the rubric it was authored with and grading uses it. The order is fixed (by id) so the
-- sampler's injected random source is the only randomness. The sample is stored in interview_questions
-- when the session starts, and every later read goes through those rows, so Get and Resume always return
-- the same questions in the same order. A bank is small (at most about 50 questions).
-- name: ListBankQuestions :many
SELECT cq.id,
       COALESCE(array_agg(DISTINCT rc.metric_id) FILTER (WHERE rc.metric_id IS NOT NULL), '{}')::uuid[] AS metric_ids
FROM public.core_questions cq
LEFT JOIN public.rubric_criteria rc ON rc.rubric_id = cq.rubric_id
WHERE cq.jd_id = @jd_id
GROUP BY cq.id
ORDER BY cq.id;

-- The metrics a posting is scored on, as evaluation reads them: when the posting has saved weights the
-- active metrics are the ones with a positive weight, and when it has none the equal-split defaults apply,
-- which is the whole metric catalog. "No rows" and "only zero weights" are different cases. It is the
-- same rule as the evaluation feature's weights (copied here so this feature does not import it).
-- name: ListPostingActiveMetrics :many
SELECT m.id
FROM public.metrics m
WHERE NOT EXISTS (SELECT 1 FROM public.metrics_percentage w WHERE w.job_posting_id = @job_posting_id)
   OR EXISTS (SELECT 1 FROM public.metrics_percentage w
              WHERE w.job_posting_id = @job_posting_id AND w.metric_id = m.id AND w.percentage > 0)
ORDER BY m.id;

-- The metric catalog: the active metrics of a practice session, which has no posting and so no weights.
-- name: ListCatalogMetrics :many
SELECT id FROM public.metrics ORDER BY id;

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

-- The session records the Interview Behavior Guideline version current when it starts (KAN-276); NULL when
-- none was ever saved, which means the defaults. A later save never changes it.
-- name: InsertInterview :one
INSERT INTO public.interviews (role_profile_id, type, application_id, avatar_id, voice_id, start_key, focus_item_id, behavior_guideline_id)
VALUES (@role_profile_id, sqlc.arg(interview_type)::text::public.interview_type,
        sqlc.narg(application_id), sqlc.narg(avatar_id), sqlc.narg(voice_id), sqlc.narg(start_key), sqlc.narg(focus_item_id),
        (SELECT g.id FROM public.interview_behavior_guidelines g ORDER BY g.version DESC LIMIT 1))
RETURNING id;

-- The guideline the session started under. No row means the session has none: the defaults apply.
-- name: GetInterviewGuideline :one
SELECT g.tone, g.question_length, g.follow_up_max, g.follow_up_focus, g.avoid_topics
FROM public.interviews i
JOIN public.interview_behavior_guidelines g ON g.id = i.behavior_guideline_id
WHERE i.id = @interview_id;

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
SELECT (count(*) + (SELECT jp.erased_interviews FROM public.job_postings jp WHERE jp.id = @job_posting_id))::bigint AS consumed
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
       i.started_at, i.completed_at, i.end_reason, i.evaluation_status, i.focus_item_id
FROM public.interviews i
LEFT JOIN public.role_profiles rp ON rp.id = i.role_profile_id
LEFT JOIN public.applications a ON a.id = i.application_id
WHERE i.id = @id AND COALESCE(rp.candidate_id, a.candidate_id) = @candidate_id;

-- The stored turns in conversation order. question_number is the core question a turn belongs to: a
-- follow-up always comes right after its core question (or an earlier follow-up of it), so it is the
-- number of core turns up to and including the row (KAN-56). It is derived, not stored.
-- name: ListInterviewTurns :many
SELECT position,
       (count(*) FILTER (WHERE kind = 'core') OVER (ORDER BY position))::int AS question_number,
       kind, question, COALESCE(answer, '')::text AS answer
FROM public.conversation_turns
WHERE interview_id = @interview_id
ORDER BY position;

-- Each stored turn with the core question of its session it belongs to (KAN-333). A session stores its
-- assigned core questions as interview_questions positions 1..n and answers them in that order, so the
-- n-th core turn is the answer to position n; a follow-up shares the core question it probes. The turn's
-- core question number is derived the same way as in ListInterviewTurns (the count of core turns up to
-- and including the row) and joined to interview_questions.position, which gives the core question id.
-- A follow-up that was asked but never answered is not stored, so it leaves no gap. This is an internal
-- read for grading; no Candidate response carries core_question_id.
-- name: ListTurnCoreQuestions :many
SELECT t.position, t.kind, t.question_number, iq.core_question_id
FROM (
    SELECT ct.position, ct.kind,
           (count(*) FILTER (WHERE ct.kind = 'core') OVER (ORDER BY ct.position))::int AS question_number
    FROM public.conversation_turns ct
    WHERE ct.interview_id = @interview_id
) t
JOIN public.interview_questions iq ON iq.interview_id = @interview_id AND iq.position = t.question_number
ORDER BY t.position;

-- A core turn, or a follow-up the live interviewer asked. asked_at is the live delivery time of its
-- question (NULL for an answer to a core question that was never delivered live); answered_at is the
-- statement time.
-- name: InsertInterviewTurn :exec
INSERT INTO public.conversation_turns (interview_id, position, kind, question, answer, asked_at, answered_at)
VALUES (@interview_id, @position, @kind, @question, @answer, sqlc.narg(asked_at), statement_timestamp());

-- The requirements of the Job Description a session was started from: the role profile's JD for practice,
-- the posting's JD for recruitment. A follow-up decider reads them; the Candidate never does.
-- name: ListInterviewRequirements :many
SELECT r.name, r.target, COALESCE(r.type::text, '')::text AS type
FROM public.interviews i
LEFT JOIN public.role_profiles rp ON rp.id = i.role_profile_id
LEFT JOIN public.applications a ON a.id = i.application_id
LEFT JOIN public.job_postings jp ON jp.id = a.job_posting_id
JOIN public.requirements r ON r.jd_id = COALESCE(rp.jd_id, jp.jd_id)
WHERE i.id = @interview_id
ORDER BY r.name, r.id;

-- Ends a session: the one write every end path makes (completion, leaving, abandonment). The guard keeps
-- the first ending's time and reason; a repeat changes nothing.
-- name: SetInterviewEnded :exec
UPDATE public.interviews
SET completed_at = statement_timestamp(), end_reason = @end_reason
WHERE id = @id AND completed_at IS NULL;

-- The last activity of a session, derived from what is stored (no activity column): the start, the latest
-- question the interviewer delivered and the latest stored answer. GREATEST skips the NULLs of a session
-- that has no delivery or answer yet. The socket being open is not activity: an answer lasts at most 3
-- minutes, so a session with nothing delivered or stored for the inactivity timeout is abandoned.
-- name: InterviewLastActivity :one
SELECT GREATEST(
           i.started_at,
           (SELECT max(iq.asked_at) FROM public.interview_questions iq WHERE iq.interview_id = i.id),
           (SELECT max(ct.answered_at) FROM public.conversation_turns ct WHERE ct.interview_id = i.id)
       )::timestamptz AS last_activity
FROM public.interviews i
WHERE i.id = @id;

-- Unfinished sessions with no activity since @idle_before (see InterviewLastActivity), oldest first, with
-- their owner. A recruitment session is listed only while its Application still accepts the interview:
-- once the window closed the expiry pass rejects the Application, and ending the session then would
-- make that pass skip it (it leaves Applications with a completed interview alone).
-- name: ListAbandonedInterviews :many
SELECT i.id, COALESCE(rp.candidate_id, a.candidate_id)::uuid AS candidate_id
FROM public.interviews i
LEFT JOIN public.role_profiles rp ON rp.id = i.role_profile_id
LEFT JOIN public.applications a ON a.id = i.application_id
WHERE i.completed_at IS NULL
  AND (i.application_id IS NULL
       OR (a.status = 'interview_eligible' AND a.interview_deadline > @now::timestamptz))
  AND GREATEST(
          i.started_at,
          (SELECT max(iq.asked_at) FROM public.interview_questions iq WHERE iq.interview_id = i.id),
          (SELECT max(ct.answered_at) FROM public.conversation_turns ct WHERE ct.interview_id = i.id)
      ) < @idle_before::timestamptz
ORDER BY i.started_at, i.id
LIMIT @batch_size;

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
-- with its process). Exactly one concurrent caller gets a row back and starts the evaluation. Each claim
-- counts as an attempt and none is granted once @max_attempts have been used (KAN-259), so a poison
-- interview cannot burn LLM calls forever.
-- It returns the attempt number the claim used (1 for the first run), and no row when nothing was claimed.
-- name: ClaimInterviewEvaluation :one
UPDATE public.interviews
SET evaluation_status = 'pending', evaluation_claimed_at = statement_timestamp(),
    evaluation_attempts = evaluation_attempts + 1
WHERE id = @id
  AND completed_at IS NOT NULL
  AND evaluation_attempts < @max_attempts::int
  AND (evaluation_status = 'failed'
       OR (evaluation_status = 'pending'
           AND (evaluation_claimed_at IS NULL
                OR evaluation_claimed_at < statement_timestamp() - make_interval(secs => @stale_seconds::float8))))
RETURNING evaluation_attempts;

-- Gives a claim back when its run was refused before it started (the daily budget did not cover a retry),
-- so the refusal does not use up one of the allowed attempts. Only an unfinished pending claim is undone.
-- name: ReleaseInterviewEvaluationClaim :exec
UPDATE public.interviews
SET evaluation_status = 'failed', evaluation_attempts = GREATEST(evaluation_attempts - 1, 0)
WHERE id = @id AND evaluation_status = 'pending';

-- True when a completed interview needs another evaluation run (failed, or pending past its stale claim)
-- but has used all @max_attempts. A completed or in-flight evaluation is not blocked.
-- name: EvaluationAttemptsExhausted :one
SELECT (completed_at IS NOT NULL
        AND evaluation_attempts >= @max_attempts::int
        AND (evaluation_status = 'failed'
             OR (evaluation_status = 'pending'
                 AND evaluation_claimed_at < statement_timestamp() - make_interval(secs => @stale_seconds::float8))))::boolean AS exhausted
FROM public.interviews
WHERE id = @id;

-- Ends an evaluation whose run panicked, so the next Complete claims and retries it instead of waiting
-- for the claim to go stale. It is the same guard as MarkInterviewEvaluationFailed in the evaluation
-- feature (interview_evaluation.sql), copied here so this feature does not import it: a completed
-- evaluation is never turned back into a failed one.
-- name: MarkEvaluationFailed :exec
UPDATE public.interviews
SET evaluation_status = 'failed'
WHERE id = @id AND evaluation_status <> 'completed';

-- A completed interview whose last allowed evaluation claim went stale will never run again, so it is
-- failed here instead of staying pending for ever. A fresh (running) claim and a completed or already
-- failed evaluation are not touched.
-- name: FailExhaustedStaleEvaluation :execrows
UPDATE public.interviews
SET evaluation_status = 'failed'
WHERE id = @id
  AND completed_at IS NOT NULL
  AND evaluation_attempts >= @max_attempts::int
  AND evaluation_status = 'pending'
  AND evaluation_claimed_at < statement_timestamp() - make_interval(secs => @stale_seconds::float8);

-- The Roadmap topic a session practises, NULL for general practice (KAN-348): the stored intent of the session.
-- name: GetInterviewFocus :one
SELECT focus_item_id FROM public.interviews WHERE id = @id;

-- A topic of the profile's Roadmap and the Roadmap's generation status (KAN-347). No row: the topic does not
-- belong to this profile. The caller has already checked that the profile belongs to the Candidate.
-- name: GetFocusTopic :one
SELECT ri.id, r.generation_status
FROM public.roadmap_items ri
JOIN public.roadmaps r ON r.id = ri.roadmap_id
WHERE ri.id = @item_id AND r.role_profile_id = @role_profile_id;

-- How many questions of the profile's bank belong to a topic.
-- name: CountTopicBankQuestions :one
SELECT count(*)::int FROM public.core_questions cq WHERE cq.jd_id = @jd_id AND cq.roadmap_item_id = @item_id;

-- The bank of a practice start with its Roadmap topic: the same rows and metrics as ListBankQuestions plus
-- roadmap_item_id (NULL for a bank written before the Roadmap existed).
-- name: ListRoadmapBank :many
SELECT cq.id,
       COALESCE(array_agg(DISTINCT rc.metric_id) FILTER (WHERE rc.metric_id IS NOT NULL), '{}')::uuid[] AS metric_ids,
       cq.roadmap_item_id
FROM public.core_questions cq
LEFT JOIN public.rubric_criteria rc ON rc.rubric_id = cq.rubric_id
WHERE cq.jd_id = @jd_id
GROUP BY cq.id
ORDER BY cq.id;

-- The valid practice results of a profile that feed its Roadmap topics: one row per assessed core question of
-- a completed, successfully evaluated, not injection-flagged practice session (the Roadmap eligibility rule).
-- A core that was shown and skipped has no stored score and counts 0, as the Roadmap read model counts it. The
-- topic is the one the assessment was recorded for; the time is the session's completion, never the link time.
-- name: ListProfileRoadmapEvidence :many
SELECT iq.core_question_id, a.roadmap_item_id AS item_id, a.interview_id, a.question_position,
       COALESCE(iq.score, 0)::float8 AS score, i.completed_at
FROM public.roadmap_assessments a
JOIN public.interviews i ON i.id = a.interview_id
JOIN public.interview_questions iq ON iq.interview_id = a.interview_id AND iq.position = a.question_position
WHERE i.role_profile_id = @role_profile_id
  AND i.type = 'practice'
  AND i.completed_at IS NOT NULL
  AND i.evaluation_status = 'completed'
  AND NOT i.injection_flagged;
