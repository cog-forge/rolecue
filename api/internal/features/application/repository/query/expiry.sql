-- Auto-reject of Applications whose interview deadline has passed (KAN-214). One pass runs in one
-- transaction. It first takes a transaction-scoped advisory lock, so concurrent API instances do not
-- run the pass twice. Then it locks each affected posting (LockPostingByID, the same NO KEY UPDATE lock
-- as screening) and rejects the expired Applications of that posting. The posting lock comes before the
-- Application rows, the order every screening and interview start uses.

-- Serializes expiry passes across instances. A busy lock means another instance is running the pass,
-- so this one does nothing. Released at commit or rollback.
-- name: TryApplicationExpiryLock :one
SELECT pg_try_advisory_xact_lock(hashtextextended(@lock_key::text, 0))::boolean AS acquired;

-- The postings that hold at least one expired Application. An Application is expired when it is still
-- interview_eligible, its deadline is before now, and no interview of it has completed. A started but
-- not completed interview is expired too (the Candidate did not finish in time).
-- name: ListPostingsWithExpiredApplications :many
SELECT DISTINCT a.job_posting_id
FROM public.applications a
WHERE a.status = 'interview_eligible'
  AND a.interview_deadline < sqlc.arg(now)::timestamptz
  AND NOT EXISTS (
      SELECT 1 FROM public.interviews i
      WHERE i.application_id = a.id AND i.completed_at IS NOT NULL
  )
ORDER BY a.job_posting_id;

-- Rejects the expired Applications of one posting. The predicate is repeated in the UPDATE so a row that
-- changed after the list was read (an interview completed, a decision made) is left alone. The caller
-- holds the posting lock.
-- name: RejectExpiredApplications :execrows
UPDATE public.applications a
SET status = 'rejected', reject_reason = sqlc.narg(reject_reason), updated_at = statement_timestamp()
WHERE a.job_posting_id = sqlc.arg(job_posting_id)
  AND a.status = 'interview_eligible'
  AND a.interview_deadline < sqlc.arg(now)::timestamptz
  AND NOT EXISTS (
      SELECT 1 FROM public.interviews i
      WHERE i.application_id = a.id AND i.completed_at IS NOT NULL
  );
