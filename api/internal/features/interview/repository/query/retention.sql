-- Personal-data retention of the System Handler (KAN-281). Both passes are batch-limited and idempotent:
-- they list only what still holds data, and the update that follows touches only what still holds it.

-- CV files of Applications rejected at the CV stage (rejected, no interview was ever started), decided
-- before @decided_before. TODO(KAN-281): applications.updated_at is the decision time only because a
-- rejected Application is never updated again; switch to applications.decided_at (migration 000028)
-- when it lands.
-- name: ListExpiredCVs :many
SELECT a.id, a.cv_path::text AS cv_path
FROM public.applications a
WHERE a.status = 'rejected'
  AND a.cv_path IS NOT NULL
  AND a.updated_at < @decided_before::timestamptz
  AND NOT EXISTS (SELECT 1 FROM public.interviews i WHERE i.application_id = a.id)
ORDER BY a.updated_at, a.id
LIMIT @batch_size;

-- Clears the key of a CV whose object was deleted. It does not touch updated_at, which stands in for
-- the decision time. The key must still be the one whose object was deleted.
-- name: ClearExpiredCV :execrows
UPDATE public.applications
SET cv_path = NULL
WHERE id = @id AND cv_path = @cv_path AND status = 'rejected';

-- Transcripts of interviews completed before @completed_before: the answers and the per-turn feedback
-- are cleared, the questions, the positions and every score stay. A cleared turn is not selected again.
-- Only the interviews of the batch are touched, and the count is the turns cleared.
-- name: ClearExpiredTranscripts :execrows
WITH batch AS (
    SELECT i.id
    FROM public.interviews i
    WHERE i.completed_at IS NOT NULL
      AND i.completed_at < @completed_before::timestamptz
      AND EXISTS (SELECT 1 FROM public.conversation_turns t
                  WHERE t.interview_id = i.id AND (t.answer IS NOT NULL OR t.feedback IS NOT NULL))
    ORDER BY i.completed_at, i.id
    LIMIT @batch_size
)
UPDATE public.conversation_turns t
SET answer = NULL, feedback = NULL
FROM batch
WHERE t.interview_id = batch.id AND (t.answer IS NOT NULL OR t.feedback IS NOT NULL);
