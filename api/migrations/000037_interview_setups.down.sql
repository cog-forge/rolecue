-- Reverts 000037: the persisted setups are dropped (they are drafts; nothing else references them).
BEGIN;

DROP TABLE public.interview_setups;

COMMIT;
