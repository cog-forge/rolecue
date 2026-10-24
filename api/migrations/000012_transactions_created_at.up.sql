BEGIN;

ALTER TABLE public.transactions
ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE INDEX IF NOT EXISTS idx_transactions_created_at ON public.transactions (created_at DESC);

UPDATE public.transactions
SET "from" = 'Banking (PayOS)'
WHERE "from" = '' OR "from" IS NULL;

UPDATE public.transactions
SET "to" = 'System'
WHERE "to" = '' OR "to" IS NULL;

COMMIT;
