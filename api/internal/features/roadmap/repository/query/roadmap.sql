-- Roadmap rows (KAN-342). The owner of a Roadmap is the candidate of its Role Profile; every read a candidate
-- can reach carries @candidate_id so a foreign profile looks exactly like a missing one.

-- name: CreatePendingRoadmap :one
INSERT INTO public.roadmaps (role_profile_id) VALUES (@role_profile_id)
RETURNING id, role_profile_id, generation_status, created_at, updated_at;

-- A profile always has its Roadmap (created with it, or by migration 000035); this repairs a row missing for a
-- profile that was written some other way. It never touches an existing row, and only the owner's profile.
-- name: EnsureOwnedRoadmap :one
INSERT INTO public.roadmaps (role_profile_id)
SELECT rp.id FROM public.role_profiles rp WHERE rp.id = @role_profile_id AND rp.candidate_id = @candidate_id
ON CONFLICT (role_profile_id) DO UPDATE SET role_profile_id = EXCLUDED.role_profile_id
RETURNING id, role_profile_id, generation_status, created_at, updated_at;

-- name: GetOwnedRoadmapByProfile :one
SELECT r.id, r.role_profile_id, r.generation_status, r.created_at, r.updated_at
FROM public.roadmaps r
JOIN public.role_profiles rp ON rp.id = r.role_profile_id
WHERE r.role_profile_id = @role_profile_id AND rp.candidate_id = @candidate_id;

-- The owner and profile of a Roadmap, which name the practice start lock of that profile.
-- name: GetRoadmapOwner :one
SELECT rp.candidate_id, rp.id AS role_profile_id
FROM public.roadmaps r JOIN public.role_profiles rp ON rp.id = r.role_profile_id
WHERE r.id = @id;

-- The same advisory lock a practice start takes (interview repository, LockPracticeStart) with the same key,
-- candidate id and profile id joined by a colon, so a start and a publish of the same profile exclude each
-- other. It is held until the transaction ends.
-- name: LockProfilePracticeStart :exec
SELECT pg_advisory_xact_lock(hashtextextended(@lock_key::text, 0));

-- name: GetRoadmapForUpdate :one
SELECT id, role_profile_id, generation_status, created_at, updated_at
FROM public.roadmaps WHERE id = @id FOR UPDATE;

-- Moves a Roadmap into generating when it is in one of the allowed states, and returns the status it had. A
-- generating Roadmap whose updated_at is older than @stale_before belongs to a worker that died, and may be
-- taken over. The new updated_at is the fencing token of this generation: only its holder may write the result.
-- name: ClaimRoadmapGeneration :one
WITH previous AS (
    SELECT rm.id, rm.generation_status FROM public.roadmaps rm WHERE rm.id = @id FOR UPDATE
)
UPDATE public.roadmaps r SET generation_status = 'generating', updated_at = clock_timestamp()
FROM previous
WHERE r.id = previous.id
  AND (previous.generation_status = ANY(@allowed_from::text[])
       OR (previous.generation_status = 'generating' AND r.updated_at < @stale_before::timestamptz))
RETURNING r.updated_at, previous.generation_status AS previous_status;

-- The facts about a candidate's profile that decide whether its Roadmap may be generated again.
-- name: GetOwnedProfileState :one
SELECT rp.jd_id, (rp.status = 'confirmed')::boolean AS confirmed,
       EXISTS (SELECT 1 FROM public.interviews i WHERE i.role_profile_id = rp.id)::boolean AS has_sessions
FROM public.role_profiles rp
WHERE rp.id = @role_profile_id AND rp.candidate_id = @candidate_id;

-- A result is written only while the fencing token is unchanged and the Roadmap is still generating.
-- name: FinishRoadmapGeneration :execrows
UPDATE public.roadmaps SET generation_status = @generation_status::text, updated_at = clock_timestamp()
WHERE id = @id AND generation_status = 'generating' AND updated_at = @fence::timestamptz;

-- name: DeleteRoadmapItems :exec
DELETE FROM public.roadmap_items WHERE roadmap_id = @roadmap_id;

-- name: InsertRoadmapItem :one
INSERT INTO public.roadmap_items (id, roadmap_id, title, description, position)
VALUES (@id, @roadmap_id, @title, sqlc.narg(description), @position)
RETURNING id, roadmap_id, title, description, position, created_at, updated_at;

-- name: ListRoadmapItems :many
SELECT id, roadmap_id, title, description, position, created_at, updated_at
FROM public.roadmap_items WHERE roadmap_id = @roadmap_id ORDER BY position, id;

-- name: ListOwnedRoadmapItemsByProfile :many
SELECT ri.id, ri.roadmap_id, ri.title, ri.description, ri.position, ri.created_at, ri.updated_at
FROM public.roadmap_items ri
JOIN public.roadmaps r ON r.id = ri.roadmap_id
JOIN public.role_profiles rp ON rp.id = r.role_profile_id
WHERE r.role_profile_id = @role_profile_id AND rp.candidate_id = @candidate_id
ORDER BY ri.position, ri.id;

-- name: GetOwnedRoadmapItem :one
SELECT ri.id, ri.roadmap_id, ri.title, ri.description, ri.position, ri.created_at, ri.updated_at
FROM public.roadmap_items ri
JOIN public.roadmaps r ON r.id = ri.roadmap_id
JOIN public.role_profiles rp ON rp.id = r.role_profile_id
WHERE ri.id = @id AND r.role_profile_id = @role_profile_id AND rp.candidate_id = @candidate_id;

-- Links one answered core question of a practice session to its topic. Nothing is written unless the
-- interview is a practice of the candidate, the topic belongs to the profile of that interview and the core
-- question of that assignment carries the same topic. A retry changes nothing (the unique key).
-- name: InsertRoadmapAssessment :execrows
INSERT INTO public.roadmap_assessments (roadmap_item_id, interview_id, question_position)
SELECT ri.id, iv.id, iq.position
FROM public.interviews iv
JOIN public.role_profiles rp ON rp.id = iv.role_profile_id
JOIN public.roadmaps r ON r.role_profile_id = rp.id
JOIN public.roadmap_items ri ON ri.roadmap_id = r.id
JOIN public.interview_questions iq ON iq.interview_id = iv.id AND iq.position = @question_position
JOIN public.core_questions cq ON cq.id = iq.core_question_id AND cq.roadmap_item_id = ri.id
WHERE iv.id = @interview_id AND iv.type = 'practice' AND ri.id = @roadmap_item_id AND rp.candidate_id = @candidate_id
ON CONFLICT (interview_id, question_position) DO NOTHING;

-- name: ListRoadmapAssessmentsByItem :many
SELECT id, roadmap_item_id, interview_id, question_position, created_at
FROM public.roadmap_assessments WHERE roadmap_item_id = @roadmap_item_id
ORDER BY interview_id, question_position;
