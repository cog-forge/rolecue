-- Per-core facts of one interview for the Learning Roadmap adapter (KAN-349, RM-08). Raw facts only: the
-- Roadmap decides what they mean. Nothing here is a hidden bank field: no expected answer, rubric or text.
--
-- delivered: the live engine froze the question for the Candidate (asked_at is set when it is delivered).
-- answered:  the core question has a stored turn, the same grouping the grader uses (question_number of a turn =
--            the count of core turns up to it = interview_questions.position), so a core with follow-ups is
--            still one question. A turn is stored only with a non-empty answer; the transcript retention pass
--            clears the answer text but keeps the turn, so expiring text never turns an answer into a skip.
-- score:     the Barem question total, set only for a graded core (numeric(5,2)).
-- name: ListCoreResultFacts :many
SELECT iq.position, iq.core_question_id, (iq.asked_at IS NOT NULL)::bool AS delivered,
       EXISTS (
           SELECT 1
           FROM (
               SELECT ct.answer,
                      (count(*) FILTER (WHERE ct.kind = 'core') OVER (ORDER BY ct.position))::int AS question_number
               FROM public.conversation_turns ct
               WHERE ct.interview_id = iq.interview_id
           ) t
           WHERE t.question_number = iq.position
       )::bool AS answered,
       iq.score
FROM public.interview_questions iq
WHERE iq.interview_id = @interview_id
ORDER BY iq.position;
