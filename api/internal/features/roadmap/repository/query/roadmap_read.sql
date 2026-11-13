-- Read model of the Learning Roadmap (KAN-352). Nothing here stores or caches a score: the rows below are the
-- raw facts the domain turns into scores, statuses and the topic history. Every read a candidate can reach
-- carries @candidate_id, so a foreign profile looks exactly like a missing one.

-- The Roadmap of an owned profile and whether the profile has any practice session, finished or not.
-- name: GetOwnedRoadmapRead :one
SELECT r.id, r.role_profile_id, r.generation_status,
       EXISTS (
           SELECT 1 FROM public.interviews i
           WHERE i.role_profile_id = r.role_profile_id AND i.type = 'practice'
       )::bool AS practice_started
FROM public.roadmaps r
JOIN public.role_profiles rp ON rp.id = r.role_profile_id
WHERE r.role_profile_id = @role_profile_id AND rp.candidate_id = @candidate_id;

-- Every assigned core question of a session of this profile whose bank question maps to a topic of the Roadmap,
-- with the stored facts the Roadmap rules need (see roadmap/domain/practice_result.go). The domain decides
-- which rows are evidence; this query only reads. It selects no expected answer, rubric, feedback or criterion
-- text, and recruitment sessions never match (they have no role profile).
--
-- delivered:     the live engine froze the question for the Candidate (asked_at).
-- answered:      the core question has a stored turn (the n-th core turn answers position n; a follow-up belongs
--                to its core). A turn is stored only with a non-empty answer, and the transcript retention pass
--                clears the answer text but keeps the turn, so the existence of the turn, not the text, says the
--                question was answered: expiring text must never turn an answer into a skip. Same grouping as
--                evaluation/query/practice_result.sql.
-- question_text: the core turn as it was asked, which the practice report already shows. Empty when unanswered.
-- linked:        a roadmap_assessments row exists for this answer and topic.
-- name: ListRoadmapResultRows :many
SELECT ri.id AS item_id, cq.id AS core_question_id, iq.interview_id, iq.position,
       i.type::text AS interview_type, i.completed_at, i.evaluation_status, i.injection_flagged,
       (iq.asked_at IS NOT NULL)::bool AS delivered,
       turns.answered::bool AS answered,
       iq.score,
       COALESCE(turns.question_text, '')::text AS question_text,
       (ra.id IS NOT NULL)::bool AS linked
FROM public.roadmap_items ri
JOIN public.core_questions cq ON cq.roadmap_item_id = ri.id
JOIN public.interview_questions iq ON iq.core_question_id = cq.id
JOIN public.interviews i ON i.id = iq.interview_id AND i.role_profile_id = @role_profile_id
LEFT JOIN public.roadmap_assessments ra
       ON ra.interview_id = iq.interview_id AND ra.question_position = iq.position AND ra.roadmap_item_id = ri.id
CROSS JOIN LATERAL (
    SELECT (count(*) > 0)::bool AS answered,
           max(t.question) FILTER (WHERE t.kind = 'core') AS question_text
    FROM (
        SELECT ct.kind, ct.question, ct.answer,
               (count(*) FILTER (WHERE ct.kind = 'core') OVER (ORDER BY ct.position))::int AS question_number
        FROM public.conversation_turns ct
        WHERE ct.interview_id = iq.interview_id
    ) t
    WHERE t.question_number = iq.position
) turns
WHERE ri.roadmap_id = @roadmap_id
ORDER BY ri.id, iq.interview_id, iq.position;
