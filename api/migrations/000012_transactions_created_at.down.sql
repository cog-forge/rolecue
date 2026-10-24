BEGIN;

DROP INDEX IF EXISTS idx_transactions_created_at;

ALTER TABLE public.transactions
DROP COLUMN IF EXISTS updated_at,
DROP COLUMN IF EXISTS created_at;

COMMIT;
