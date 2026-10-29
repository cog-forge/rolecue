BEGIN;

-- Publication begins at the first successful slot funding, not posting creation or moderation.
-- Existing rows keep NULL: their actual publication time cannot be reconstructed reliably.
ALTER TABLE public.job_postings ADD COLUMN published_at timestamptz;
CREATE INDEX job_postings_live_publication_idx ON public.job_postings (published_at DESC)
    WHERE status = 'open' AND interview_slot IS NOT NULL;

COMMIT;
