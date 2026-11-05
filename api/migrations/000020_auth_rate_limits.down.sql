BEGIN;

-- Counters are transient: dropping them only resets the current windows.
DROP TABLE IF EXISTS public.rate_limits;

COMMIT;
