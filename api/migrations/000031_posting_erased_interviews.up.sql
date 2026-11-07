-- Interview slots used by Candidates whose account was later erased (KAN-280). The erasure deletes the
-- interview and application rows, which are what the capacity and refund counts read; this counter keeps
-- the number, without any personal data, so an erased interview still counts as a used slot.
BEGIN;

ALTER TABLE public.job_postings
    ADD COLUMN erased_interviews integer NOT NULL DEFAULT 0 CHECK (erased_interviews >= 0);

COMMIT;
