-- A Job Posting is the Recruiter's company JD: job_descriptions has no owner column, so every
-- read and write goes through job_postings.recruiter_id. Another Recruiter's posting is not found.

-- name: GetLevelIDByName :one
SELECT id FROM public.levels
WHERE lower(name) = @name::text
ORDER BY id
LIMIT 1;

-- content_path is the storage key of the JD PDF; raw_text is the text read from it.
-- name: InsertJobDescription :one
INSERT INTO public.job_descriptions (content_path, raw_text, level_id, refinement_note)
VALUES (@content_path, @raw_text, @level_id, @refinement_note)
RETURNING id;

-- name: InsertRequirements :exec
INSERT INTO public.requirements (jd_id, name, target, type)
SELECT @jd_id::uuid, unnest(@names::text[]), unnest(@targets::text[]),
       unnest(@types::text[])::public.requirement_type;

-- name: DeleteRequirements :exec
DELETE FROM public.requirements WHERE jd_id = @jd_id;

-- name: UpdateJDLevel :exec
UPDATE public.job_descriptions SET level_id = @level_id WHERE id = @id;

-- name: InsertJobPosting :one
INSERT INTO public.job_postings (
    jd_id, recruiter_id, title, location, employment_type, salary_min, salary_max,
    interview_duration, required_avatar_id, required_voice_id
)
VALUES (
    @jd_id, @recruiter_id, @title, sqlc.narg(location), sqlc.narg(employment_type), sqlc.narg(salary_min),
    sqlc.narg(salary_max), sqlc.narg(interview_duration), @required_avatar_id, @required_voice_id
)
RETURNING id, created_at;

-- An avatar belongs to its owner through inventories.user_id. No row means not owned.
-- name: OwnedAvatarName :one
SELECT a.name
FROM public.avatars a
JOIN public.inventories i ON i.id = a.inventory_id
WHERE a.id = @avatar_id AND i.user_id = @user_id;

-- No row means the voice is missing or inactive. FOR SHARE holds the voice until the posting is
-- written: an Admin deactivating or deleting it locks the row FOR UPDATE, so either the Admin waits
-- and then counts this posting as a reference, or this check waits and sees the voice inactive.
-- name: ActiveVoiceName :one
SELECT name FROM public.voices WHERE id = @id AND is_active FOR SHARE;

-- name: GetOwnedPosting :one
SELECT jp.id, jp.jd_id, jp.title, jp.status::text AS status, jp.reject_reason, jp.location, jp.employment_type,
       jp.salary_min, jp.salary_max, jp.interview_slot, jp.interview_duration, jp.created_at,
       jp.required_avatar_id, a.name AS avatar_name, jp.required_voice_id, v.name AS voice_name,
       jd.content_path, jd.raw_text, COALESCE(jd.refinement_note, '')::text AS refinement_note, lower(l.name) AS level_name,
       jp.review_submitted,
       (SELECT count(*) FROM public.core_questions cq WHERE cq.jd_id = jp.jd_id) AS core_question_count
FROM public.job_postings jp
JOIN public.job_descriptions jd ON jd.id = jp.jd_id
JOIN public.levels l ON l.id = jd.level_id
LEFT JOIN public.avatars a ON a.id = jp.required_avatar_id
LEFT JOIN public.voices v ON v.id = jp.required_voice_id
WHERE jp.id = @id AND jp.recruiter_id = @recruiter_id;

-- name: ListRequirements :many
SELECT name, target, type::text AS type
FROM public.requirements
WHERE jd_id = @jd_id
ORDER BY name, id;

-- Every filter is optional: a NULL argument disables it. The search matches the title.
-- name: ListOwnedPostings :many
SELECT jp.id, jp.title, jp.status::text AS status, jp.reject_reason, jp.location, jp.employment_type,
       jp.interview_slot, jp.created_at, jp.review_submitted,
       (SELECT count(*) FROM public.core_questions cq WHERE cq.jd_id = jp.jd_id) AS core_question_count
FROM public.job_postings jp
WHERE jp.recruiter_id = @recruiter_id
  AND (sqlc.narg(status)::text IS NULL OR jp.status::text = sqlc.narg(status)::text)
  AND (sqlc.narg(search)::text IS NULL OR jp.title ILIKE '%' || sqlc.narg(search)::text || '%')
ORDER BY jp.created_at DESC, jp.id DESC
LIMIT @page_limit OFFSET @page_offset;

-- name: CountOwnedPostings :one
SELECT count(*)
FROM public.job_postings jp
WHERE jp.recruiter_id = @recruiter_id
  AND (sqlc.narg(status)::text IS NULL OR jp.status::text = sqlc.narg(status)::text)
  AND (sqlc.narg(search)::text IS NULL OR jp.title ILIKE '%' || sqlc.narg(search)::text || '%');

-- The row lock serializes an edit with an Admin decision and with question-bank generation, which
-- locks the same row before saving. Anything read from other tables must be read by a later
-- statement: one that waited for the lock still sees those tables as they were when it started.
-- name: LockOwnedPosting :one
SELECT jp.jd_id, jp.status::text AS status, jp.salary_min, jp.salary_max, jp.required_voice_id, jp.required_avatar_id,
       jp.review_submitted
FROM public.job_postings jp
WHERE jp.id = @id AND jp.recruiter_id = @recruiter_id
FOR UPDATE;

-- name: GetJDLevelName :one
SELECT lower(l.name) AS level_name
FROM public.job_descriptions jd
JOIN public.levels l ON l.id = jd.level_id
WHERE jd.id = @id;

-- name: CountCoreQuestions :one
SELECT count(*) FROM public.core_questions WHERE jd_id = @jd_id;

-- A rejected posting never opened, so no interview references its questions.
-- name: DeleteCoreQuestions :exec
DELETE FROM public.core_questions WHERE jd_id = @jd_id;

-- name: UpdateJDDocument :exec
UPDATE public.job_descriptions SET content_path = @content_path, raw_text = @raw_text WHERE id = @id;

-- name: UpdatePosting :exec
UPDATE public.job_postings SET
    title = CASE WHEN sqlc.arg(set_title)::boolean THEN sqlc.arg(title)::text ELSE title END,
    location = CASE WHEN sqlc.arg(set_location)::boolean THEN sqlc.narg(location)::text ELSE location END,
    employment_type = CASE WHEN sqlc.arg(set_employment_type)::boolean THEN sqlc.narg(employment_type)::text ELSE employment_type END,
    salary_min = CASE WHEN sqlc.arg(set_salary_min)::boolean THEN sqlc.narg(salary_min)::integer ELSE salary_min END,
    salary_max = CASE WHEN sqlc.arg(set_salary_max)::boolean THEN sqlc.narg(salary_max)::integer ELSE salary_max END,
    interview_duration = CASE WHEN sqlc.arg(set_interview_duration)::boolean THEN sqlc.narg(interview_duration)::integer ELSE interview_duration END,
    required_avatar_id = CASE WHEN sqlc.arg(set_avatar)::boolean THEN sqlc.narg(required_avatar_id)::uuid ELSE required_avatar_id END,
    required_voice_id = CASE WHEN sqlc.arg(set_voice)::boolean THEN sqlc.narg(required_voice_id)::uuid ELSE required_voice_id END
WHERE id = @id AND recruiter_id = @recruiter_id AND status IN ('pending', 'rejected');

-- Sends a prepared posting to the Admin queue; a rejected posting's old feedback no longer applies.
-- Called only after LockOwnedPosting and the bank and presentation checks.
-- name: SubmitPosting :exec
UPDATE public.job_postings SET status = 'pending', reject_reason = NULL, review_submitted = true
WHERE id = @id AND recruiter_id = @recruiter_id;

-- name: GetJDRefinementNote :one
SELECT COALESCE(refinement_note, '')::text AS refinement_note FROM public.job_descriptions WHERE id = @id;

-- name: UpdateJDRefinementNote :exec
UPDATE public.job_descriptions SET refinement_note = @refinement_note WHERE id = @id;
