-- Interview Behavior Guidelines (KAN-276): append-only versions, the current one is the highest version.

-- name: GetCurrentGuideline :one
SELECT g.id, g.version, g.tone, g.question_length, g.follow_up_max, g.follow_up_focus, g.avoid_topics,
       g.updated_by, COALESCE(u.name, '')::text AS updated_by_name, g.created_at
FROM public.interview_behavior_guidelines g
LEFT JOIN public.users u ON u.id = g.updated_by
ORDER BY g.version DESC
LIMIT 1;

-- Serialises saves: the lock is held until the transaction ends, so the version read after it is the
-- version the insert follows.
-- name: LockGuidelineSaves :exec
SELECT pg_advisory_xact_lock(hashtextextended('interview_behavior_guidelines', 0));

-- name: CurrentGuidelineVersion :one
SELECT COALESCE(max(version), 0)::integer FROM public.interview_behavior_guidelines;

-- name: InsertGuideline :one
INSERT INTO public.interview_behavior_guidelines
    (version, tone, question_length, follow_up_max, follow_up_focus, avoid_topics, updated_by)
VALUES (@version, @tone, @question_length, @follow_up_max, @follow_up_focus, @avoid_topics, @updated_by)
RETURNING id, version, tone, question_length, follow_up_max, follow_up_focus, avoid_topics, updated_by, created_at;

-- name: GetGuidelineAuthorName :one
SELECT COALESCE((SELECT name FROM public.users WHERE id = @user_id), '')::text;
