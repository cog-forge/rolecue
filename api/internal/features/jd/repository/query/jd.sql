-- A Candidate's JD is owned through role_profiles.candidate_id; job_descriptions has no owner
-- column. Every read and write joins or locks role_profiles, so another user's JD is not found.

-- name: GetLevelIDByName :one
SELECT id FROM public.levels
WHERE lower(name) = @name::text
ORDER BY id
LIMIT 1;

-- name: InsertJobDescription :one
INSERT INTO public.job_descriptions (content_path, level_id, refinement_note)
VALUES (@content_path, @level_id, sqlc.narg(refinement_note))
RETURNING id;

-- name: InsertRoleProfile :one
INSERT INTO public.role_profiles (jd_id, candidate_id, title)
VALUES (@jd_id, @candidate_id, @title)
RETURNING created_at;

-- A blank evidence entry stands for "no evidence" (text[] elements can't be NULL from Go).
-- name: InsertRequirements :exec
INSERT INTO public.requirements (jd_id, name, target, type, evidence_text)
SELECT @jd_id::uuid, unnest(@names::text[]), unnest(@targets::text[]),
       unnest(@types::text[])::public.requirement_type, NULLIF(unnest(@evidence_texts::text[]), '');

-- name: ListJDs :many
SELECT rp.jd_id, rp.title, l.name AS level_name, rp.created_at
FROM public.role_profiles rp
JOIN public.job_descriptions jd ON jd.id = rp.jd_id
JOIN public.levels l ON l.id = jd.level_id
WHERE rp.candidate_id = @candidate_id
ORDER BY rp.created_at DESC, rp.jd_id DESC
LIMIT @page_limit OFFSET @page_offset;

-- name: CountJDs :one
SELECT count(*) FROM public.role_profiles
WHERE candidate_id = @candidate_id;

-- name: GetJD :one
SELECT jd.id, jd.content_path, jd.refinement_note, rp.title, l.name AS level_name, rp.created_at,
       EXISTS(SELECT 1 FROM public.core_questions cq WHERE cq.jd_id = jd.id) AS has_question_bank
FROM public.role_profiles rp
JOIN public.job_descriptions jd ON jd.id = rp.jd_id
JOIN public.levels l ON l.id = jd.level_id
WHERE rp.jd_id = @id AND rp.candidate_id = @candidate_id;

-- There is no position column, so rows come back by target, then name. COLLATE "C" sorts
-- byte-wise, which the service mirrors when it answers a Create.
-- name: ListRequirements :many
SELECT name, target, type::text AS type, evidence_text
FROM public.requirements
WHERE jd_id = @jd_id
ORDER BY target COLLATE "C", name COLLATE "C", id;

-- Locks the role profile row; question-bank generation locks the same row, so a bank can't
-- appear between this check and the write.
-- name: LockOwnedJD :one
SELECT jd.content_path,
       EXISTS(SELECT 1 FROM public.core_questions cq WHERE cq.jd_id = jd.id) AS has_question_bank
FROM public.role_profiles rp
JOIN public.job_descriptions jd ON jd.id = rp.jd_id
WHERE rp.jd_id = @id AND rp.candidate_id = @candidate_id
FOR UPDATE OF rp;

-- name: UpdateJobDescription :exec
UPDATE public.job_descriptions
SET level_id = @level_id, refinement_note = sqlc.narg(refinement_note)
WHERE id = @id;

-- name: UpdateRoleProfileTitle :one
UPDATE public.role_profiles SET title = @title
WHERE jd_id = @jd_id
RETURNING created_at;

-- name: DeleteRequirements :exec
DELETE FROM public.requirements WHERE jd_id = @jd_id;

-- name: DeleteCoreQuestions :exec
DELETE FROM public.core_questions WHERE jd_id = @jd_id;

-- name: DeleteRoleProfile :exec
DELETE FROM public.role_profiles WHERE jd_id = @jd_id;

-- name: DeleteJobDescription :exec
DELETE FROM public.job_descriptions WHERE id = @id;
