-- name: CreateProfileJD :one
INSERT INTO job_descriptions(raw_text) VALUES (@raw_text) RETURNING id;

-- name: CreateDocumentJD :one
INSERT INTO job_descriptions(raw_text, content_path) VALUES(@raw_text, @content_path) RETURNING id;

-- name: CreateRoleProfile :one
INSERT INTO role_profiles(jd_id, candidate_id, title)
VALUES (@jd_id, @candidate_id, 'Untitled role') RETURNING id;

-- name: GetRoleProfile :one
SELECT rp.id, rp.jd_id, rp.candidate_id, rp.title, rp.status, rp.created_at,
       COALESCE(jd.raw_text, '')::text AS raw_text,
       COALESCE(jd.content_path, '')::text AS content_path,
       COALESCE(jd.refinement_note, '')::text AS refinement_note, l.name AS seniority_level
FROM role_profiles rp JOIN job_descriptions jd ON jd.id = rp.jd_id
LEFT JOIN levels l ON l.id = jd.level_id
WHERE rp.id = @id AND rp.candidate_id = @candidate_id AND jd.raw_text IS NOT NULL;

-- name: LockOwnedProfile :one
SELECT rp.id FROM role_profiles rp JOIN job_descriptions jd ON jd.id = rp.jd_id WHERE rp.id = @id AND rp.candidate_id = @candidate_id AND jd.raw_text IS NOT NULL FOR UPDATE OF rp;

-- name: ListRoleProfileSummaries :many
SELECT rp.id, rp.title, rp.status, rp.created_at, l.name AS seniority_level
FROM role_profiles rp JOIN job_descriptions jd ON jd.id = rp.jd_id
LEFT JOIN levels l ON l.id = jd.level_id
WHERE rp.candidate_id = @candidate_id AND jd.raw_text IS NOT NULL
ORDER BY rp.created_at DESC, rp.id DESC LIMIT @page_limit OFFSET @page_offset;

-- name: CountRoleProfiles :one
SELECT count(*) FROM role_profiles rp JOIN job_descriptions jd ON jd.id = rp.jd_id WHERE rp.candidate_id = @candidate_id AND jd.raw_text IS NOT NULL;

-- name: ListProfileRequirements :many
SELECT name, target, kind, category, type, evidence_text FROM requirements
WHERE jd_id = @jd_id ORDER BY name, id;

-- name: SaveProfileJD :exec
UPDATE job_descriptions SET refinement_note = @refinement_note,
level_id = (SELECT id FROM levels WHERE name = sqlc.narg(seniority_level)) WHERE job_descriptions.id = @id;

-- name: DeleteProfileRequirements :exec
DELETE FROM requirements WHERE jd_id = @jd_id;

-- name: InsertProfileRequirement :exec
INSERT INTO requirements(jd_id, name, target, kind, category, type, evidence_text)
VALUES (@jd_id, @name, @target, @kind, sqlc.narg(category), sqlc.narg(type), sqlc.narg(evidence_text));

-- name: CompleteProfileReview :exec
UPDATE role_profiles SET title = @title, status = 'review'
WHERE id = @id;

-- name: ConfirmRoleProfile :exec
UPDATE role_profiles SET status = 'confirmed' WHERE id = @id;

-- name: ClaimProfileAnalysis :exec
UPDATE role_profiles SET status = 'analyzing' WHERE id = @id;

-- name: ReleaseProfileAnalysis :exec
UPDATE role_profiles SET status = 'draft' WHERE id = @id AND status = 'analyzing';

-- name: ProfileHasInterviews :one
SELECT EXISTS (SELECT 1 FROM interviews WHERE role_profile_id = @role_profile_id);

-- name: DeleteOwnedRoleProfile :exec
DELETE FROM role_profiles WHERE id = @id;

-- name: DeleteProfileQuestions :exec
DELETE FROM core_questions WHERE jd_id = @jd_id;

-- name: DeleteProfileJD :exec
DELETE FROM job_descriptions WHERE id = @id;
