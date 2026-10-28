BEGIN;

-- Transactions had no timestamp, so the admin billing screens could not filter by date and the wallet
-- history was sorted by a random UUID (KAN-191, KAN-192, KAN-206). Both writers (ledger.insertRow and
-- the PayOS checkout InsertTransaction) omit the column and get the default. Rows that exist when this
-- runs all get the migration time; that is acceptable because only dev data exists.
ALTER TABLE public.transactions
    ADD COLUMN created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP;

-- Every admin billing query takes an optional [date_from, date_to) window on this column.
CREATE INDEX transactions_created_at_idx ON public.transactions (created_at);

COMMIT;
