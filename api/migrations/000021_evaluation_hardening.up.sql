-- Evaluation hardening (KAN-253, KAN-259). Additive only: no new table.
BEGIN;

-- The verbatim quote from a Candidate answer that supports a metric score. The server checks it
-- against the stored answers before it is trusted; NULL for evaluations written before this column.
ALTER TABLE public.score_details
    ADD COLUMN evidence text;

-- Set when the model reported a suspected prompt injection or a score's evidence quote could not be
-- found in the Candidate's answers. The evaluation still completes; the Recruiter sees the flag.
-- evaluation_attempts counts the claims of the evaluation and caps retries (see ClaimInterviewEvaluation).
ALTER TABLE public.interviews
    ADD COLUMN injection_flagged boolean NOT NULL DEFAULT false,
    ADD COLUMN evaluation_attempts integer NOT NULL DEFAULT 0;

COMMIT;
