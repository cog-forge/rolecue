BEGIN;
DROP INDEX public.transactions_created_at_idx;
ALTER TABLE public.transactions DROP COLUMN created_at;
COMMIT;
