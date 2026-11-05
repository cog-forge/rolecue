BEGIN;

-- A slot purchase repeated with the same Idempotency-Key (a double click, a retry after a timeout)
-- finds its first ledger row and does not debit again (KAN-267). The key is scoped to the payer.
ALTER TABLE public.transactions ADD COLUMN idempotency_key text;

CREATE UNIQUE INDEX transactions_from_idempotency_key_key
    ON public.transactions ("from", idempotency_key)
    WHERE idempotency_key IS NOT NULL;

COMMIT;
