-- name: GetWalletByUserID :one
SELECT id, user_id, balance
FROM public.wallets
WHERE user_id = @user_id;

-- name: GetWalletByUserIDForUpdate :one
SELECT id, user_id, balance
FROM public.wallets
WHERE user_id = @user_id
FOR UPDATE;

-- name: CreateWallet :one
INSERT INTO public.wallets (user_id, balance)
VALUES (@user_id, 0)
RETURNING id, user_id, balance;

-- name: UpdateWalletBalance :one
UPDATE public.wallets
SET balance = @balance
WHERE id = @id
RETURNING id, user_id, balance;

-- name: InsertTransaction :one
INSERT INTO public.transactions ("from", "to", amount, currency, description, status, payos_order_code)
VALUES (@tx_from, @tx_to, @amount, @currency, @description, @status, @payos_order_code)
RETURNING id, "from", "to", amount, currency, description, status, payos_order_code;

-- name: GetTransactionByID :one
SELECT id, "from", "to", amount, currency, description, status, payos_order_code
FROM public.transactions
WHERE id = @id;

-- name: GetTransactionByPayOSOrderCode :one
SELECT id, "from", "to", amount, currency, description, status, payos_order_code
FROM public.transactions
WHERE payos_order_code = @payos_order_code;

-- name: GetTransactionByPayOSOrderCodeForUpdate :one
SELECT id, "from", "to", amount, currency, description, status, payos_order_code
FROM public.transactions
WHERE payos_order_code = @payos_order_code
FOR UPDATE;

-- name: UpdateTransactionStatus :one
UPDATE public.transactions
SET status = @status
WHERE id = @id
RETURNING id, "from", "to", amount, currency, description, status, payos_order_code;

-- name: ListTransactionsByUserID :many
SELECT id, "from", "to", amount, currency, description, status, payos_order_code
FROM public.transactions
WHERE "from" = @tx_user_id OR "to" = @tx_user_id
ORDER BY id DESC
LIMIT @page_limit OFFSET @page_offset;

-- name: CountTransactionsByUserID :one
SELECT COUNT(*)
FROM public.transactions
WHERE "from" = @tx_user_id OR "to" = @tx_user_id;

-- name: ListActiveCoinPackages :many
SELECT id, name, price, description, amount, currency
FROM public.coin_packages
ORDER BY price ASC;

-- name: GetCoinPackageByID :one
SELECT id, name, price, description, amount, currency
FROM public.coin_packages
WHERE id = @id;
