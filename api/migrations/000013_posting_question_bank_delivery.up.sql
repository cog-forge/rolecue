BEGIN;

-- A pending posting is prepared by the Recruiter (question bank generated and reviewed) and only
-- reaches the Admin queue once explicitly submitted.
ALTER TABLE public.job_postings
    ADD COLUMN review_submitted boolean NOT NULL DEFAULT false;

-- Before this column, every pending posting was in the queue. Keep those with a bank there; a
-- pending posting without questions goes back to preparation.
UPDATE public.job_postings jp
SET review_submitted = true
WHERE jp.status = 'pending'
  AND EXISTS (SELECT 1 FROM public.core_questions cq WHERE cq.jd_id = jp.jd_id);

COMMIT;
