-- name: GetRecruiterPosting :one
SELECT jd_id, status::text AS status, review_submitted
FROM public.job_postings
WHERE id = @id AND recruiter_id = @recruiter_id;

-- Ids are random UUIDs, so this order is stable between reads but unrelated to generation order;
-- core_questions has no position column and the frozen schema must not change.
-- name: ListCoreQuestions :many
SELECT id, content
FROM public.core_questions
WHERE jd_id = @jd_id
ORDER BY id;

-- The Recruiter's view of the same bank, with each question's grading setup and whether its rubric can still
-- be used (KAN-329, KAN-331). The interview runtime keeps using ListCoreQuestions, which never selects
-- rubric_id or expected_answer. A rubric is permitted when it is a system rubric or one the Recruiter owns.
-- name: ListBankQuestions :many
SELECT cq.id, cq.content, cq.rubric_id, cq.expected_answer,
       COALESCE(r.is_active, false)::boolean AS rubric_active,
       (r.id IS NOT NULL AND (r.scope = 'system' OR r.owner_id = @recruiter_id::uuid))::boolean AS rubric_permitted
FROM public.core_questions cq
LEFT JOIN public.rubrics r ON r.id = cq.rubric_id
WHERE cq.jd_id = @jd_id
ORDER BY cq.id;

-- name: GetBankQuestion :one
SELECT cq.id, cq.content, cq.rubric_id, cq.expected_answer,
       COALESCE(r.is_active, false)::boolean AS rubric_active,
       (r.id IS NOT NULL AND (r.scope = 'system' OR r.owner_id = @recruiter_id::uuid))::boolean AS rubric_permitted
FROM public.core_questions cq
LEFT JOIN public.rubrics r ON r.id = cq.rubric_id
WHERE cq.id = @id;

-- The readiness rule (KAN-331): every question needs a rubric that is active and permitted for the posting's
-- owner, and an expected answer that is not blank and within the bounds. The same predicate is repeated in
-- jobposting/repository/query/jobposting.sql and admin/moderation/repository/query/moderation.sql (sqlc has no
-- shared fragments); keep the three identical.
-- name: CountIncompleteGrading :one
SELECT count(*)::int
FROM public.core_questions cq
LEFT JOIN public.rubrics r ON r.id = cq.rubric_id
WHERE cq.jd_id = @jd_id
  AND NOT (r.id IS NOT NULL AND r.is_active AND (r.scope = 'system' OR r.owner_id = @recruiter_id::uuid)
           AND cq.expected_answer IS NOT NULL AND cq.expected_answer ~ '\S'
           AND char_length(cq.expected_answer) <= @max_runes::int
           AND cardinality(regexp_split_to_array(btrim(cq.expected_answer), '\s+')) <= @max_words::int);

-- A change to any question after the posting went to the Admin queue withdraws the submission, so the Admin
-- cannot approve content that changed after the Recruiter submitted it (the Admin's decision waits for the
-- posting lock these writes hold and then finds the posting no longer submitted).
-- name: WithdrawSubmission :exec
UPDATE public.job_postings SET review_submitted = false WHERE id = @id AND review_submitted;

-- name: LockQuestionContent :one
SELECT cq.content
FROM public.core_questions cq
JOIN public.job_postings jp ON jp.jd_id = cq.jd_id
WHERE cq.id = @id AND jp.id = @posting_id
FOR UPDATE OF cq;

-- The rubrics a generation may choose from: active system templates first, then the Recruiter's own active
-- rubrics, newest change first, bounded by page_limit. The visibility rule is the one of the /rubrics API
-- (evaluation/repository/query/rubric.sql): system AND active, or owned. An archived rubric is never offered.
-- name: ListGenerationRubrics :many
SELECT r.id, r.name, r.description, r.question_type
FROM public.rubrics r
WHERE r.is_active AND (r.scope = 'system' OR r.owner_id = @recruiter_id::uuid)
ORDER BY (r.scope = 'system') DESC, CASE WHEN r.scope = 'system' THEN r.name END, r.updated_at DESC, r.id
LIMIT @page_limit;

-- name: ListGenerationRubricCriteria :many
SELECT c.rubric_id, c.name, m.name AS metric_name, c.max_points
FROM public.rubric_criteria c
JOIN public.metrics m ON m.id = c.metric_id
WHERE c.rubric_id = ANY(@rubric_ids::uuid[])
ORDER BY c.rubric_id, c.position;

-- name: CountCoreQuestions :one
SELECT count(*) FROM public.core_questions WHERE jd_id = @jd_id;

-- name: ListRequirementsForJD :many
SELECT name, target, COALESCE(type::text, '')::text AS type, evidence_text
FROM public.requirements
WHERE jd_id = @jd_id
ORDER BY name, id;

-- name: GetJDGenerationContext :one
SELECT jd.refinement_note, l.name AS level_name, l.description AS level_description
FROM public.job_descriptions jd
JOIN public.levels l ON l.id = jd.level_id
WHERE jd.id = @id;

-- A question is added only to a generated bank, so manual questions cannot skip generation.
-- name: AddCoreQuestion :one
INSERT INTO public.core_questions (jd_id, content)
SELECT jp.jd_id, @content::text
FROM public.job_postings jp
WHERE jp.id = @posting_id AND jp.recruiter_id = @recruiter_id AND jp.status IN ('pending', 'rejected')
  AND EXISTS (SELECT 1 FROM public.core_questions cq WHERE cq.jd_id = jp.jd_id)
RETURNING id, content, rubric_id, expected_answer;

-- name: UpdateCoreQuestion :one
UPDATE public.core_questions cq
SET content = @content::text
FROM public.job_postings jp
WHERE cq.id = @id
  AND cq.jd_id = jp.jd_id
  AND jp.id = @posting_id
  AND jp.recruiter_id = @recruiter_id
  AND jp.status IN ('pending', 'rejected')
RETURNING cq.id, cq.content, cq.rubric_id, cq.expected_answer;

-- name: DeleteCoreQuestion :execrows
DELETE FROM public.core_questions cq
USING public.job_postings jp
WHERE cq.id = @id
  AND cq.jd_id = jp.jd_id
  AND jp.id = @posting_id
  AND jp.recruiter_id = @recruiter_id
  AND jp.status IN ('pending', 'rejected');

-- name: CandidateOwnsJD :one
SELECT EXISTS(
    SELECT 1 FROM public.role_profiles
    WHERE jd_id = @jd_id AND candidate_id = @candidate_id
);

-- name: LockEditablePosting :one
SELECT jd_id
FROM public.job_postings
WHERE id = @id AND recruiter_id = @recruiter_id AND status IN ('pending', 'rejected')
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

-- Locks the generated rubric bindings that are still usable by this Recruiter (active, system or owned), so an
-- archive (which takes FOR UPDATE on the rubric) cannot land between this check and the insert.
-- name: LockUsableRubrics :many
SELECT r.id
FROM public.rubrics r
WHERE r.id = ANY(@rubric_ids::uuid[]) AND r.is_active AND (r.scope = 'system' OR r.owner_id = @recruiter_id::uuid)
ORDER BY r.id
FOR SHARE;

-- A generated Recruiter bank: each question with its rubric (NULL when none was chosen or it stopped being
-- usable) and expected answer (NULL when none). The arrays are parallel; an empty rubric id means none.
-- name: InsertGradedCoreQuestions :many
INSERT INTO public.core_questions (jd_id, content, rubric_id, expected_answer)
SELECT @jd_id::uuid, (@contents::text[])[i],
       (SELECT r.id FROM public.rubrics r
        WHERE r.id = NULLIF((@rubric_ids::text[])[i], '')::uuid AND r.is_active AND (r.scope = 'system' OR r.owner_id = @recruiter_id::uuid)),
       NULLIF((@expected_answers::text[])[i], '')
FROM generate_series(1, cardinality(@contents::text[])) AS i
ORDER BY i
RETURNING id, content, rubric_id, expected_answer;
