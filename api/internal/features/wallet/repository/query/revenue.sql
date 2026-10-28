-- Revenue report (KAN-192). Revenue figures reuse the admin billing queries; these add what they lack.

-- name: CountNewPayers :one
-- Users whose first ever successful PayOS top-up falls in [date_from, date_to). A top-up credits "to".
SELECT COUNT(*)::bigint
FROM (
    SELECT t."to", MIN(t.created_at) AS first_paid_at
    FROM public.transactions t
    WHERE t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' AND t.status = 'success'
    GROUP BY t."to"
) payers
WHERE payers.first_paid_at >= @date_from::timestamptz
  AND payers.first_paid_at < @date_to::timestamptz;

-- name: ListUserRoles :many
SELECT u.id, COALESCE(u.role, '')::text AS role
FROM public.users u
WHERE u.id = ANY(@ids::uuid[]);

-- name: SumWalletBalances :one
SELECT COALESCE(SUM(w.balance), 0)::bigint
FROM public.wallets w;
