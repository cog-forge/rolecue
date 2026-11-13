-- Catch-up of Roadmap links (KAN-351, RM-10). The job reads its work from the database on every run: there is no
-- queue and no outbox, so a restart between a saved result and its link loses nothing.

-- Practice sessions that may feed a Roadmap and still have at least one countable, mapped core question without a
-- link, oldest evaluation first (interviews.completed_at, then id: the event time, never the time of any link).
--
-- The filter must be exact, or the same sessions would be listed first on every run and starve the rest. It
-- mirrors the Go rules and is tested against them (roadmap/tests): the session predicate is
-- domain.SessionFacts.Eligible (practice, completed, evaluation completed, not injection-flagged) and a core is
-- countable like domain.ClassifyCore (a graded core, or a delivered core that has no stored turn).
-- @exclude holds the sessions that are backing off after a failure.
-- name: ListSessionsMissingAssessments :many
SELECT i.id
FROM public.interviews i
JOIN public.role_profiles rp ON rp.id = i.role_profile_id
WHERE i.type = 'practice' AND i.completed_at IS NOT NULL AND i.evaluation_status = 'completed' AND NOT i.injection_flagged
  AND i.id <> ALL(@exclude::uuid[])
  AND EXISTS (
      SELECT 1
      FROM public.interview_questions iq
      JOIN public.core_questions cq ON cq.id = iq.core_question_id
      JOIN public.roadmap_items ri ON ri.id = cq.roadmap_item_id
      JOIN public.roadmaps r ON r.id = ri.roadmap_id AND r.role_profile_id = i.role_profile_id
      WHERE iq.interview_id = i.id
        AND NOT EXISTS (SELECT 1 FROM public.roadmap_assessments ra
                        WHERE ra.interview_id = iq.interview_id AND ra.question_position = iq.position)
        AND (iq.score IS NOT NULL
             OR (iq.asked_at IS NOT NULL
                 AND NOT EXISTS (
                     SELECT 1
                     FROM (
                         SELECT (count(*) FILTER (WHERE ct.kind = 'core') OVER (ORDER BY ct.position))::int AS question_number
                         FROM public.conversation_turns ct
                         WHERE ct.interview_id = iq.interview_id
                     ) t
                     WHERE t.question_number = iq.position)))
  )
ORDER BY i.completed_at, i.id
LIMIT @batch_size;
