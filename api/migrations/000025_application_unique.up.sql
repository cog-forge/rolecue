BEGIN;

-- One Application per Candidate and posting, enforced by the database (KAN-268). The advisory lock in the
-- apply path already serializes the common race; this index makes a second row impossible whatever
-- path writes it.
--
-- Rows that already break the rule are removed deterministically before the index is built. In each
-- (posting, Candidate) group the Application that has an interview is kept, otherwise the earliest by
-- created_at and then id. A duplicate that has an interview of its own is never deleted.
DELETE FROM public.applications a
USING (
    SELECT ranked.id
    FROM (
        SELECT x.id,
               row_number() OVER (
                   PARTITION BY x.job_posting_id, x.candidate_id
                   ORDER BY EXISTS (SELECT 1 FROM public.interviews i WHERE i.application_id = x.id) DESC,
                            x.created_at ASC, x.id ASC
               ) AS rn
        FROM public.applications x
    ) ranked
    WHERE ranked.rn > 1
) dup
WHERE a.id = dup.id
  AND NOT EXISTS (SELECT 1 FROM public.interviews i WHERE i.application_id = a.id);

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM public.applications
        GROUP BY job_posting_id, candidate_id
        HAVING count(*) > 1
    ) THEN
        RAISE EXCEPTION 'applications still has duplicate (job_posting_id, candidate_id) pairs that each own an interview; merge them by hand, then rerun this migration';
    END IF;
END $$;

CREATE UNIQUE INDEX applications_posting_candidate_key
    ON public.applications (job_posting_id, candidate_id);

COMMIT;
