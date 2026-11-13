-- Writes the links between answered practice questions and Roadmap topics (KAN-350, RM-09).

-- Links the listed assignment positions of one practice interview to the primary topic of their core question,
-- in ONE statement: every row is written or none is, whatever happens to the connection. Nothing is written
-- unless every condition holds for that position:
--   * the interview is a completed practice session of @candidate_id (a recruitment interview never matches:
--     it has no role profile, and the type is checked too),
--   * the core question carries a topic (core_questions.roadmap_item_id),
--   * that topic belongs to the Roadmap of the interview's own Role Profile (a foreign key alone cannot say so),
--   * the assignment exists (interview_questions).
-- The score, the answer and the evaluation state are not copied: the link only names the topic. A position
-- that is already linked is skipped by the UNIQUE (interview_id, question_position) key, so a retry, a second
-- worker or the catch-up job never adds a second row. The count is the rows newly written.
-- name: LinkInterviewCores :execrows
INSERT INTO public.roadmap_assessments (roadmap_item_id, interview_id, question_position)
SELECT ri.id, iv.id, iq.position
FROM public.interviews iv
JOIN public.role_profiles rp ON rp.id = iv.role_profile_id
JOIN public.interview_questions iq ON iq.interview_id = iv.id AND iq.position = ANY(@positions::int[])
JOIN public.core_questions cq ON cq.id = iq.core_question_id
JOIN public.roadmap_items ri ON ri.id = cq.roadmap_item_id
JOIN public.roadmaps r ON r.id = ri.roadmap_id AND r.role_profile_id = iv.role_profile_id
WHERE iv.id = @interview_id AND iv.type = 'practice' AND iv.completed_at IS NOT NULL AND rp.candidate_id = @candidate_id
ON CONFLICT (interview_id, question_position) DO NOTHING;

-- After LinkInterviewCores: of the listed positions, how many have a link now (whoever wrote it: a retry or a
-- concurrent worker) and how many could have one, because their core question has a topic in the Roadmap of the
-- interview's own Role Profile (the join the link rules use). A core without a topic is neither: it is simply not
-- part of any Roadmap. present < mappable means the link rules refused a core that does have a topic.
-- name: CountInterviewLinks :one
SELECT
    (SELECT count(*) FROM public.roadmap_assessments ra
      WHERE ra.interview_id = @interview_id AND ra.question_position = ANY(@positions::int[]))::bigint AS present,
    (SELECT count(*) FROM public.interview_questions iq
       JOIN public.interviews iv ON iv.id = iq.interview_id
       JOIN public.core_questions cq ON cq.id = iq.core_question_id
       JOIN public.roadmap_items ri ON ri.id = cq.roadmap_item_id
       JOIN public.roadmaps r ON r.id = ri.roadmap_id AND r.role_profile_id = iv.role_profile_id
      WHERE iq.interview_id = @interview_id AND iq.position = ANY(@positions::int[]))::bigint AS mappable;
