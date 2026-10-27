-- The balance after a purchase debited it through the ledger, read in the same transaction while the
-- ledger still holds the wallet row lock.
-- name: GetWalletBalance :one
SELECT balance
FROM public.wallets
WHERE user_id = @user_id;
