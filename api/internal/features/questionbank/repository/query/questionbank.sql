-- name: GetRecruiterPosting :one
SELECT jd_id, status::text AS status
FROM public.job_postings
WHERE id = @id AND recruiter_id = @recruiter_id;

-- Ids are random UUIDs, so this order is stable between reads but unrelated to generation order;
-- core_questions has no position column and the frozen schema must not change.
-- name: ListCoreQuestions :many
SELECT id, content
FROM public.core_questions
WHERE jd_id = @jd_id
ORDER BY id;

-- name: CountCoreQuestions :one
SELECT count(*) FROM public.core_questions WHERE jd_id = @jd_id;

-- name: ListRequirementsForJD :many
SELECT name, target, type::text AS type, evidence_text
FROM public.requirements
WHERE jd_id = @jd_id
ORDER BY name, id;

-- name: GetJDRefinementNote :one
SELECT refinement_note FROM public.job_descriptions WHERE id = @id;

-- name: AddCoreQuestion :one
INSERT INTO public.core_questions (jd_id, content)
SELECT jp.jd_id, @content::text
FROM public.job_postings jp
WHERE jp.id = @posting_id AND jp.recruiter_id = @recruiter_id AND jp.status = 'pending'
RETURNING id, content;

-- name: UpdateCoreQuestion :one
UPDATE public.core_questions cq
SET content = @content::text
FROM public.job_postings jp
WHERE cq.id = @id
  AND cq.jd_id = jp.jd_id
  AND jp.id = @posting_id
  AND jp.recruiter_id = @recruiter_id
  AND jp.status = 'pending'
RETURNING cq.id, cq.content;

-- name: DeleteCoreQuestion :execrows
DELETE FROM public.core_questions cq
USING public.job_postings jp
WHERE cq.id = @id
  AND cq.jd_id = jp.jd_id
  AND jp.id = @posting_id
  AND jp.recruiter_id = @recruiter_id
  AND jp.status = 'pending';

-- name: CandidateOwnsJD :one
SELECT EXISTS(
    SELECT 1 FROM public.role_profiles
    WHERE jd_id = @jd_id AND candidate_id = @candidate_id
);

-- name: LockPendingPosting :one
SELECT jd_id
FROM public.job_postings
WHERE id = @id AND recruiter_id = @recruiter_id AND status = 'pending'
FOR UPDATE;

-- name: LockCandidateRoleProfile :one
SELECT jd_id
FROM public.role_profiles
WHERE jd_id = @jd_id AND candidate_id = @candidate_id
FOR UPDATE;

-- name: InsertCoreQuestions :many
INSERT INTO public.core_questions (jd_id, content)
SELECT @jd_id::uuid, unnest(@contents::text[])
RETURNING id, content;
