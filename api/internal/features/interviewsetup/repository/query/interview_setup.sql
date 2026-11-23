-- Persisted pre-interview setup (KAN-231). Every read is one statement, so the facts a readiness is computed
-- from are one snapshot. The queries are Candidate-scoped: a foreign target is "no rows".

-- The practice target the Candidate owns, its saved setup (if any) and the live state of what the setup
-- points at. bank_ready is whether the Job Description has at least one core question; the bank itself is
-- never returned. An avatar is usable when it is still in the Candidate's own inventory and its model is not
-- blocked. A deleted avatar or voice is already NULL in the setup row (ON DELETE SET NULL).
-- name: GetPracticeSetup :one
SELECT rp.title,
       rp.status,
       EXISTS (SELECT 1 FROM public.core_questions cq WHERE cq.jd_id = rp.jd_id)::boolean AS bank_ready,
       (st.id IS NOT NULL)::boolean AS saved,
       st.updated_at,
       st.environment,
       av.id AS avatar_id,
       av.name AS avatar_name,
       COALESCE(inv.user_id = rp.candidate_id AND s.status = 'ready', false)::boolean AS avatar_usable,
       v.id AS voice_id,
       v.name AS voice_name,
       COALESCE(v.is_active, false)::boolean AS voice_active
FROM public.role_profiles rp
LEFT JOIN public.interview_setups st ON st.role_profile_id = rp.id
LEFT JOIN public.avatars av ON av.id = st.avatar_id
LEFT JOIN public.inventories inv ON inv.id = av.inventory_id
LEFT JOIN public.avatar_assets s ON s.id = av.asset_id
LEFT JOIN public.voices v ON v.id = st.voice_id
WHERE rp.id = @role_profile_id AND rp.candidate_id = @candidate_id;

-- The recruitment target the Candidate owns, its saved environment and the avatar and voice the Recruiter
-- locked on the posting. The eligibility is the one the company interviewer read uses: CV approved, inside
-- the deadline on the database clock, a funded open or intake_closed posting. The locked avatar must still be
-- in the posting Recruiter's own inventory with a model that is not blocked. started is whether an interview
-- of the Application exists.
-- name: GetRecruitmentSetup :one
SELECT a.status::text AS status,
       a.interview_deadline,
       COALESCE(a.interview_deadline > statement_timestamp(), false)::boolean AS within_deadline,
       jp.title,
       (jp.status IN ('open', 'intake_closed') AND jp.interview_slot IS NOT NULL)::boolean AS posting_accepting,
       EXISTS (SELECT 1 FROM public.interviews i WHERE i.application_id = a.id)::boolean AS started,
       (st.id IS NOT NULL)::boolean AS saved,
       st.updated_at,
       st.environment,
       av.id AS avatar_id,
       av.name AS avatar_name,
       COALESCE(inv.user_id = jp.recruiter_id AND s.status = 'ready', false)::boolean AS avatar_usable,
       v.id AS voice_id,
       v.name AS voice_name,
       COALESCE(v.is_active, false)::boolean AS voice_active
FROM public.applications a
JOIN public.job_postings jp ON jp.id = a.job_posting_id
LEFT JOIN public.interview_setups st ON st.application_id = a.id
LEFT JOIN public.avatars av ON av.id = jp.required_avatar_id
LEFT JOIN public.inventories inv ON inv.id = av.inventory_id
LEFT JOIN public.avatar_assets s ON s.id = av.asset_id
LEFT JOIN public.voices v ON v.id = jp.required_voice_id
WHERE a.id = @application_id AND a.candidate_id = @candidate_id;

-- Locks the avatar against deletion while the setup is written, and says whether it may be chosen: only an
-- avatar in the Candidate's own inventory whose model is not blocked (another user's avatar is their
-- likeness). No row: not found.
-- name: LockChoosableAvatar :one
SELECT av.id
FROM public.avatars av
JOIN public.inventories inv ON inv.id = av.inventory_id
JOIN public.avatar_assets s ON s.id = av.asset_id
WHERE av.id = @id AND inv.user_id = @user_id AND s.status = 'ready'
FOR KEY SHARE OF av;

-- Locks the voice against deletion while the setup is written. Only an active voice may be chosen.
-- name: LockActiveVoice :one
SELECT id FROM public.voices WHERE id = @id AND is_active FOR KEY SHARE;

-- A full replacement: NULL is "not chosen yet". A repeat with the same values changes nothing, not even
-- updated_at, so a retried save is invisible.
-- name: UpsertPracticeSetup :exec
INSERT INTO public.interview_setups (type, role_profile_id, avatar_id, voice_id, environment)
VALUES ('practice', @role_profile_id, sqlc.narg(avatar_id), sqlc.narg(voice_id), sqlc.narg(environment))
ON CONFLICT (role_profile_id) DO UPDATE
SET avatar_id = EXCLUDED.avatar_id, voice_id = EXCLUDED.voice_id, environment = EXCLUDED.environment,
    updated_at = now()
WHERE (interview_setups.avatar_id, interview_setups.voice_id, interview_setups.environment)
      IS DISTINCT FROM (EXCLUDED.avatar_id, EXCLUDED.voice_id, EXCLUDED.environment);

-- name: UpsertRecruitmentSetup :exec
INSERT INTO public.interview_setups (type, application_id, environment)
VALUES ('recruitment', @application_id, sqlc.narg(environment))
ON CONFLICT (application_id) DO UPDATE
SET environment = EXCLUDED.environment, updated_at = now()
WHERE interview_setups.environment IS DISTINCT FROM EXCLUDED.environment;
