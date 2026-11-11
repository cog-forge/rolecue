-- Rubric management (KAN-326). Visibility is decided here, in SQL, so a rubric the caller may not see is never
-- returned, whatever id was asked for:
--   admin     sees the system rubrics (any status) and never a Recruiter's private rubric;
--   recruiter sees the ACTIVE system rubrics and the rubrics they own.
-- The predicate is repeated in each query on purpose (sqlc has no shared fragments); keep the copies identical.

-- name: ListVisibleRubrics :many
SELECT r.id, r.name, r.description, r.question_type, r.scope, r.owner_id, r.is_active, r.created_at, r.updated_at,
       (SELECT count(*) FROM public.rubric_criteria c WHERE c.rubric_id = r.id)::int AS criteria_count,
       (SELECT COALESCE(sum(c.max_points), 0) FROM public.rubric_criteria c WHERE c.rubric_id = r.id)::int AS max_points,
       (SELECT count(*) FROM public.core_questions cq WHERE cq.rubric_id = r.id)::int AS reference_count
FROM public.rubrics r
WHERE (
        (sqlc.arg('viewer_role')::text = 'admin' AND r.scope = 'system')
     OR (sqlc.arg('viewer_role')::text = 'recruiter' AND ((r.scope = 'system' AND r.is_active) OR r.owner_id = sqlc.arg('viewer_id')::uuid))
      )
  AND (sqlc.narg('scope_filter')::text IS NULL
       OR (sqlc.narg('scope_filter')::text = 'system' AND r.scope = 'system')
       OR (sqlc.narg('scope_filter')::text = 'own' AND r.owner_id = sqlc.arg('viewer_id')::uuid))
  AND (sqlc.narg('question_type')::text IS NULL OR r.question_type = sqlc.narg('question_type')::text)
  AND (sqlc.arg('status')::text = 'all' OR (sqlc.arg('status')::text = 'active' AND r.is_active) OR (sqlc.arg('status')::text = 'archived' AND NOT r.is_active))
ORDER BY r.name, r.id
LIMIT @page_limit OFFSET @page_offset;

-- name: CountVisibleRubrics :one
SELECT count(*)
FROM public.rubrics r
WHERE (
        (sqlc.arg('viewer_role')::text = 'admin' AND r.scope = 'system')
     OR (sqlc.arg('viewer_role')::text = 'recruiter' AND ((r.scope = 'system' AND r.is_active) OR r.owner_id = sqlc.arg('viewer_id')::uuid))
      )
  AND (sqlc.narg('scope_filter')::text IS NULL
       OR (sqlc.narg('scope_filter')::text = 'system' AND r.scope = 'system')
       OR (sqlc.narg('scope_filter')::text = 'own' AND r.owner_id = sqlc.arg('viewer_id')::uuid))
  AND (sqlc.narg('question_type')::text IS NULL OR r.question_type = sqlc.narg('question_type')::text)
  AND (sqlc.arg('status')::text = 'all' OR (sqlc.arg('status')::text = 'active' AND r.is_active) OR (sqlc.arg('status')::text = 'archived' AND NOT r.is_active));

-- name: GetVisibleRubric :one
SELECT r.id, r.name, r.description, r.question_type, r.scope, r.owner_id, r.is_active, r.created_at, r.updated_at,
       (SELECT count(*) FROM public.rubric_criteria c WHERE c.rubric_id = r.id)::int AS criteria_count,
       (SELECT COALESCE(sum(c.max_points), 0) FROM public.rubric_criteria c WHERE c.rubric_id = r.id)::int AS max_points,
       (SELECT count(*) FROM public.core_questions cq WHERE cq.rubric_id = r.id)::int AS reference_count
FROM public.rubrics r
WHERE r.id = @id
  AND (
        (sqlc.arg('viewer_role')::text = 'admin' AND r.scope = 'system')
     OR (sqlc.arg('viewer_role')::text = 'recruiter' AND ((r.scope = 'system' AND r.is_active) OR r.owner_id = sqlc.arg('viewer_id')::uuid))
      );

-- name: ListRubricCriteria :many
SELECT c.id, c.metric_id, m.name AS metric_name, c.name, c.max_points, c.scoring_guide, c.position
FROM public.rubric_criteria c
JOIN public.metrics m ON m.id = c.metric_id
WHERE c.rubric_id = @rubric_id
ORDER BY c.position;

-- Locks the rubric's row for the whole transaction. A question that is being pointed at the rubric holds a
-- FOR KEY SHARE lock on this row through its foreign key, which conflicts with FOR UPDATE: so the reference
-- count read right after this lock cannot change until the transaction ends.
-- name: LockRubric :one
SELECT id, scope, owner_id, is_active, updated_at
FROM public.rubrics
WHERE id = @id
FOR UPDATE;

-- Copy: the source is read under a shared lock, so it cannot be edited or deleted between this read and the
-- insert of the copy. The same visibility rule applies as for a plain read.
-- name: ShareVisibleRubric :one
SELECT r.id, r.name, r.description, r.question_type
FROM public.rubrics r
WHERE r.id = @id
  AND (
        (sqlc.arg('viewer_role')::text = 'admin' AND r.scope = 'system')
     OR (sqlc.arg('viewer_role')::text = 'recruiter' AND ((r.scope = 'system' AND r.is_active) OR r.owner_id = sqlc.arg('viewer_id')::uuid))
      )
FOR SHARE;

-- name: CountRubricReferences :one
SELECT count(*)::int FROM public.core_questions WHERE rubric_id = @rubric_id;

-- name: InsertRubric :one
INSERT INTO public.rubrics (name, description, question_type, scope, owner_id)
VALUES (@name, @description, @question_type, @scope, sqlc.narg('owner_id'))
RETURNING id;

-- name: InsertRubricCriterion :exec
INSERT INTO public.rubric_criteria (rubric_id, metric_id, name, max_points, scoring_guide, position)
VALUES (@rubric_id, @metric_id, @name, @max_points, @scoring_guide, @position);

-- The copy gets new criterion ids; metric, text, points and order are taken from the source.
-- name: CopyRubricCriteria :exec
INSERT INTO public.rubric_criteria (rubric_id, metric_id, name, max_points, scoring_guide, position)
SELECT @new_rubric_id, c.metric_id, c.name, c.max_points, c.scoring_guide, c.position
FROM public.rubric_criteria c
WHERE c.rubric_id = @source_rubric_id;

-- Only the given fields change. updated_at is the optimistic-concurrency token and always moves forward:
-- clock_timestamp() is the real time, not the transaction start.
-- name: UpdateRubricFields :exec
UPDATE public.rubrics
SET name = COALESCE(sqlc.narg('name')::text, name),
    description = COALESCE(sqlc.narg('description')::text, description),
    question_type = COALESCE(sqlc.narg('question_type')::text, question_type),
    is_active = COALESCE(sqlc.narg('is_active')::boolean, is_active),
    updated_at = GREATEST(clock_timestamp(), updated_at + interval '1 microsecond')
WHERE id = @id;

-- Keeps the listed criteria of a rubric and deletes the others.
-- name: DeleteRubricCriteriaExcept :exec
DELETE FROM public.rubric_criteria
WHERE rubric_id = @rubric_id AND NOT (id = ANY(sqlc.arg('keep')::text[]::uuid[]));

-- name: UpdateRubricCriterion :execrows
UPDATE public.rubric_criteria
SET metric_id = @metric_id, name = @name, max_points = @max_points, scoring_guide = @scoring_guide, position = @position
WHERE id = @id AND rubric_id = @rubric_id;

-- name: DeleteRubric :exec
DELETE FROM public.rubrics WHERE id = @id;

-- Reads a rubric without the visibility rule. Used only inside a transaction that has just written it, to
-- return the stored result; every read for a caller goes through GetVisibleRubric.
-- name: GetRubricByID :one
SELECT r.id, r.name, r.description, r.question_type, r.scope, r.owner_id, r.is_active, r.created_at, r.updated_at,
       (SELECT count(*) FROM public.rubric_criteria c WHERE c.rubric_id = r.id)::int AS criteria_count,
       (SELECT COALESCE(sum(c.max_points), 0) FROM public.rubric_criteria c WHERE c.rubric_id = r.id)::int AS max_points,
       (SELECT count(*) FROM public.core_questions cq WHERE cq.rubric_id = r.id)::int AS reference_count
FROM public.rubrics r
WHERE r.id = @id;
