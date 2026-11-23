-- name: EnsureWallet :exec
-- Creates the user's wallet at zero when it is missing. An existing wallet is left untouched.
INSERT INTO public.wallets (user_id, balance) VALUES (@user_id, 0)
ON CONFLICT (user_id) DO NOTHING;

-- name: LockWalletByUser :one
-- Reads and row-locks the user's wallet. The caller picks the lock order; a wallet is always the last lock.
SELECT id, balance FROM public.wallets WHERE user_id = @user_id FOR UPDATE;

-- name: SetWalletBalance :exec
UPDATE public.wallets SET balance = @balance WHERE id = @id;

-- name: InsertCoinTransaction :exec
-- One COIN row without an idempotency key.
INSERT INTO public.transactions ("from", "to", amount, currency, description, status)
VALUES (@from_party, @to_party, @amount, @currency, @description, 'success');

-- name: InsertKeyedCoinTransaction :exec
-- One COIN row that carries an idempotency key, scoped to the payer by a unique index.
INSERT INTO public.transactions ("from", "to", amount, currency, description, status, idempotency_key)
VALUES (@from_party, @to_party, @amount, @currency, @description, 'success', @idempotency_key);

-- name: InsertKeyedCoinTransactionReturning :one
-- The Marketplace transfer row; the receipt is built from the returned id and created_at.
INSERT INTO public.transactions ("from", "to", amount, currency, description, status, idempotency_key)
VALUES (@from_party, @to_party, @amount, @currency, @description, 'success', @idempotency_key)
RETURNING id, created_at;

-- name: FindKeyedTransaction :one
-- The payer's row for an idempotency key. The unique index is on ("from", idempotency_key), so only the
-- payer's own rows can match.
SELECT id, "to", amount, currency, status::text AS status, description, created_at
FROM public.transactions
WHERE "from" = @from_party AND idempotency_key = @idempotency_key;
