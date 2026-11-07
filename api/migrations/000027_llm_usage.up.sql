-- Per-user LLM budget (KAN-278). One row per user and UTC day counts the LLM-backed operations the
-- user started; the daily budget is enforced against it with an atomic upsert (see
-- internal/pkg/llmbudget). Additive only. Rows hold counts, never prompts or model output.
BEGIN;

CREATE TABLE public.llm_usage (
    user_id uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    day date NOT NULL,
    calls integer NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, day),
    CONSTRAINT llm_usage_calls_check CHECK (calls >= 0)
);

COMMIT;
