-- name: EnsureWallet :exec
INSERT INTO public.wallets (user_id, balance)
VALUES (@user_id, 0)
ON CONFLICT (user_id) DO NOTHING;

-- name: GetWallet :one
SELECT id, balance
FROM public.wallets
WHERE user_id = @user_id
FOR UPDATE;

-- name: UpdateWalletBalance :exec
UPDATE public.wallets
SET balance = @balance
WHERE id = @id;
