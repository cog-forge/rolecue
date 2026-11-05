BEGIN;

DROP INDEX public.transactions_from_idempotency_key_key;
ALTER TABLE public.transactions DROP COLUMN idempotency_key;

COMMIT;
