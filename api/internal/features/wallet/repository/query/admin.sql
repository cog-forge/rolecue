-- Admin billing and revenue read-only queries (KAN-191, KAN-192, KAN-241).
-- Each query keeps its own copy of the enrichment CTE. Every filtered query applies the same optional
-- [date_from, date_to) window on t.created_at inside the CTE, before the user and package lookups.

-- name: ListAdminTransactions :many
WITH enriched AS (
    SELECT
        t.id,
        t.created_at,
        t.amount,
        t."from" AS tx_from,
        t."to" AS tx_to,
        COALESCE(t.description, '') AS description,
        t.currency,
        t.status::text AS status,
        COALESCE(t.payos_order_code, '') AS payos_order_code,
        CASE
            WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN 'topup'
            WHEN lower(t."from") = 'system' OR lower(COALESCE(t.description, '')) LIKE '%refund%' THEN 'refund'
            ELSE 'spend'
        END AS tx_type,
        COALESCE(u.id::text,
            CASE
                WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN t."to"
                WHEN t."from" = '' OR lower(t."from") = 'system' THEN t."to"
                ELSE t."from"
            END
        )::text AS user_id,
        COALESCE(u.name, '') AS user_name,
        COALESCE(u.email, '') AS user_email,
        (CASE
            WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN
                COALESCE(
                    NULLIF(btrim(regexp_replace(regexp_replace(COALESCE(t.description, ''), '^Deposit:\s*', ''), '\s*\([0-9]+\s*VND\)\s*$', '')), ''),
                    cp.name,
                    'Coin Top-up'
                )
            ELSE COALESCE(NULLIF(btrim(t.description), ''), 'Coin Transaction')
        END)::text AS package_or_feature,
        (CASE
            WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN
                COALESCE(
                    NULLIF(substring(COALESCE(t.description, '') from '\(([0-9]+)\s*VND\)'), '')::bigint,
                    cp.price::bigint,
                    t.amount::bigint
                )
            ELSE 0::bigint
        END)::bigint AS paid_vnd,
        (CASE
            WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN t.amount::bigint
            WHEN lower(t."from") = 'system' OR lower(COALESCE(t.description, '')) LIKE '%refund%' THEN abs(t.amount)::bigint
            ELSE -abs(t.amount)::bigint
        END)::bigint AS coin_amount
    FROM public.transactions t
    LEFT JOIN public.users u ON u.id = (
        CASE
            WHEN (
                CASE
                    WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN t."to"
                    WHEN t."from" = '' OR lower(t."from") = 'system' THEN t."to"
                    ELSE t."from"
                END
            ) ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
            THEN (
                CASE
                    WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN t."to"
                    WHEN t."from" = '' OR lower(t."from") = 'system' THEN t."to"
                    ELSE t."from"
                END
            )::uuid
            ELSE NULL
        END
    )
    LEFT JOIN LATERAL (
        SELECT cp_inner.name, cp_inner.price, cp_inner.amount
        FROM public.coin_packages cp_inner
        WHERE t.payos_order_code IS NOT NULL AND t.payos_order_code <> ''
          AND (
              cp_inner.name = btrim(regexp_replace(regexp_replace(COALESCE(t.description, ''), '^Deposit:\s*', ''), '\s*\([0-9]+\s*VND\)\s*$', ''))
              OR cp_inner.amount = t.amount
          )
        ORDER BY
            CASE
                WHEN cp_inner.name = btrim(regexp_replace(regexp_replace(COALESCE(t.description, ''), '^Deposit:\s*', ''), '\s*\([0-9]+\s*VND\)\s*$', '')) THEN 0
                ELSE 1
            END,
            cp_inner.price ASC
        LIMIT 1
    ) cp ON true
    WHERE (sqlc.narg(date_from)::timestamptz IS NULL OR t.created_at >= sqlc.narg(date_from)::timestamptz)
      AND (sqlc.narg(date_to)::timestamptz IS NULL OR t.created_at < sqlc.narg(date_to)::timestamptz)
)
SELECT
    id,
    amount,
    tx_from,
    tx_to,
    description,
    currency,
    status,
    payos_order_code,
    tx_type,
    user_id,
    user_name,
    user_email,
    package_or_feature,
    paid_vnd,
    coin_amount,
    created_at,
    COUNT(*) OVER()::bigint AS total_count
FROM enriched
WHERE (sqlc.narg(search)::text IS NULL OR sqlc.narg(search)::text = '' OR (
    id::text ILIKE '%' || sqlc.narg(search)::text || '%' ESCAPE '\'
    OR user_name ILIKE '%' || sqlc.narg(search)::text || '%' ESCAPE '\'
    OR user_email ILIKE '%' || sqlc.narg(search)::text || '%' ESCAPE '\'
    OR user_id ILIKE '%' || sqlc.narg(search)::text || '%' ESCAPE '\'
    OR description ILIKE '%' || sqlc.narg(search)::text || '%' ESCAPE '\'
    OR payos_order_code ILIKE '%' || sqlc.narg(search)::text || '%' ESCAPE '\'
))
  AND (sqlc.narg(status)::text IS NULL OR sqlc.narg(status)::text = '' OR status = sqlc.narg(status)::text)
  AND (sqlc.narg(tx_type)::text IS NULL OR sqlc.narg(tx_type)::text = '' OR tx_type = sqlc.narg(tx_type)::text)
  AND (sqlc.narg(package_name)::text IS NULL OR sqlc.narg(package_name)::text = '' OR package_or_feature ILIKE '%' || sqlc.narg(package_name)::text || '%' ESCAPE '\')
  AND (sqlc.narg(amount_min)::bigint IS NULL OR (CASE WHEN tx_type = 'topup' THEN paid_vnd ELSE abs(coin_amount) END) >= sqlc.narg(amount_min)::bigint)
  AND (sqlc.narg(amount_max)::bigint IS NULL OR (CASE WHEN tx_type = 'topup' THEN paid_vnd ELSE abs(coin_amount) END) <= sqlc.narg(amount_max)::bigint)
ORDER BY
    CASE WHEN @sort_by::text = 'created_at' AND @sort_order::text = 'asc' THEN created_at END ASC,
    CASE WHEN @sort_by::text = 'created_at' AND @sort_order::text = 'desc' THEN created_at END DESC,
    CASE WHEN @sort_by::text = 'amount' AND @sort_order::text = 'asc' THEN paid_vnd END ASC,
    CASE WHEN @sort_by::text = 'amount' AND @sort_order::text = 'desc' THEN paid_vnd END DESC,
    CASE WHEN @sort_by::text = 'coins' AND @sort_order::text = 'asc' THEN coin_amount END ASC,
    CASE WHEN @sort_by::text = 'coins' AND @sort_order::text = 'desc' THEN coin_amount END DESC,
    CASE WHEN @sort_by::text = 'status' AND @sort_order::text = 'asc' THEN status END ASC,
    CASE WHEN @sort_by::text = 'status' AND @sort_order::text = 'desc' THEN status END DESC,
    CASE WHEN @sort_by::text = 'id' AND @sort_order::text = 'asc' THEN id END ASC,
    CASE WHEN @sort_by::text = 'id' AND @sort_order::text = 'desc' THEN id END DESC,
    created_at DESC,
    id DESC
LIMIT @page_limit OFFSET @page_offset;

-- name: CountAdminTransactions :one
WITH enriched AS (
    SELECT
        t.id,
        t.amount,
        COALESCE(t.description, '') AS description,
        t.status::text AS status,
        COALESCE(t.payos_order_code, '') AS payos_order_code,
        CASE
            WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN 'topup'
            WHEN lower(t."from") = 'system' OR lower(COALESCE(t.description, '')) LIKE '%refund%' THEN 'refund'
            ELSE 'spend'
        END AS tx_type,
        COALESCE(u.id::text,
            CASE
                WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN t."to"
                WHEN t."from" = '' OR lower(t."from") = 'system' THEN t."to"
                ELSE t."from"
            END
        )::text AS user_id,
        COALESCE(u.name, '') AS user_name,
        COALESCE(u.email, '') AS user_email,
        (CASE
            WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN
                COALESCE(
                    NULLIF(btrim(regexp_replace(regexp_replace(COALESCE(t.description, ''), '^Deposit:\s*', ''), '\s*\([0-9]+\s*VND\)\s*$', '')), ''),
                    cp.name,
                    'Coin Top-up'
                )
            ELSE COALESCE(NULLIF(btrim(t.description), ''), 'Coin Transaction')
        END)::text AS package_or_feature,
        (CASE
            WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN
                COALESCE(
                    NULLIF(substring(COALESCE(t.description, '') from '\(([0-9]+)\s*VND\)'), '')::bigint,
                    cp.price::bigint,
                    t.amount::bigint
                )
            ELSE 0::bigint
        END)::bigint AS paid_vnd,
        (CASE
            WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN t.amount::bigint
            WHEN lower(t."from") = 'system' OR lower(COALESCE(t.description, '')) LIKE '%refund%' THEN abs(t.amount)::bigint
            ELSE -abs(t.amount)::bigint
        END)::bigint AS coin_amount
    FROM public.transactions t
    LEFT JOIN public.users u ON u.id = (
        CASE
            WHEN (
                CASE
                    WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN t."to"
                    WHEN t."from" = '' OR lower(t."from") = 'system' THEN t."to"
                    ELSE t."from"
                END
            ) ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
            THEN (
                CASE
                    WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN t."to"
                    WHEN t."from" = '' OR lower(t."from") = 'system' THEN t."to"
                    ELSE t."from"
                END
            )::uuid
            ELSE NULL
        END
    )
    LEFT JOIN LATERAL (
        SELECT cp_inner.name, cp_inner.price, cp_inner.amount
        FROM public.coin_packages cp_inner
        WHERE t.payos_order_code IS NOT NULL AND t.payos_order_code <> ''
          AND (
              cp_inner.name = btrim(regexp_replace(regexp_replace(COALESCE(t.description, ''), '^Deposit:\s*', ''), '\s*\([0-9]+\s*VND\)\s*$', ''))
              OR cp_inner.amount = t.amount
          )
        ORDER BY
            CASE
                WHEN cp_inner.name = btrim(regexp_replace(regexp_replace(COALESCE(t.description, ''), '^Deposit:\s*', ''), '\s*\([0-9]+\s*VND\)\s*$', '')) THEN 0
                ELSE 1
            END,
            cp_inner.price ASC
        LIMIT 1
    ) cp ON true
    WHERE (sqlc.narg(date_from)::timestamptz IS NULL OR t.created_at >= sqlc.narg(date_from)::timestamptz)
      AND (sqlc.narg(date_to)::timestamptz IS NULL OR t.created_at < sqlc.narg(date_to)::timestamptz)
)
SELECT COUNT(*)::bigint
FROM enriched
WHERE (sqlc.narg(search)::text IS NULL OR sqlc.narg(search)::text = '' OR (
    id::text ILIKE '%' || sqlc.narg(search)::text || '%' ESCAPE '\'
    OR user_name ILIKE '%' || sqlc.narg(search)::text || '%' ESCAPE '\'
    OR user_email ILIKE '%' || sqlc.narg(search)::text || '%' ESCAPE '\'
    OR user_id ILIKE '%' || sqlc.narg(search)::text || '%' ESCAPE '\'
    OR description ILIKE '%' || sqlc.narg(search)::text || '%' ESCAPE '\'
    OR payos_order_code ILIKE '%' || sqlc.narg(search)::text || '%' ESCAPE '\'
))
  AND (sqlc.narg(status)::text IS NULL OR sqlc.narg(status)::text = '' OR status = sqlc.narg(status)::text)
  AND (sqlc.narg(tx_type)::text IS NULL OR sqlc.narg(tx_type)::text = '' OR tx_type = sqlc.narg(tx_type)::text)
  AND (sqlc.narg(package_name)::text IS NULL OR sqlc.narg(package_name)::text = '' OR package_or_feature ILIKE '%' || sqlc.narg(package_name)::text || '%' ESCAPE '\')
  AND (sqlc.narg(amount_min)::bigint IS NULL OR (CASE WHEN tx_type = 'topup' THEN paid_vnd ELSE abs(coin_amount) END) >= sqlc.narg(amount_min)::bigint)
  AND (sqlc.narg(amount_max)::bigint IS NULL OR (CASE WHEN tx_type = 'topup' THEN paid_vnd ELSE abs(coin_amount) END) <= sqlc.narg(amount_max)::bigint);

-- name: GetAdminTransactionByID :one
SELECT
    t.id,
    t.amount,
    t."from" AS tx_from,
    t."to" AS tx_to,
    COALESCE(t.description, '') AS description,
    t.currency,
    t.status::text AS status,
    COALESCE(t.payos_order_code, '') AS payos_order_code,
    CASE
        WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN 'topup'
        WHEN lower(t."from") = 'system' OR lower(COALESCE(t.description, '')) LIKE '%refund%' THEN 'refund'
        ELSE 'spend'
    END AS tx_type,
    COALESCE(u.id::text,
        CASE
            WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN t."to"
            WHEN t."from" = '' OR lower(t."from") = 'system' THEN t."to"
            ELSE t."from"
        END
    )::text AS user_id,
    COALESCE(u.name, '') AS user_name,
    COALESCE(u.email, '') AS user_email,
    (CASE
        WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN
            COALESCE(
                NULLIF(btrim(regexp_replace(regexp_replace(COALESCE(t.description, ''), '^Deposit:\s*', ''), '\s*\([0-9]+\s*VND\)\s*$', '')), ''),
                cp.name,
                'Coin Top-up'
            )
        ELSE COALESCE(NULLIF(btrim(t.description), ''), 'Coin Transaction')
    END)::text AS package_or_feature,
    (CASE
        WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN
            COALESCE(
                NULLIF(substring(COALESCE(t.description, '') from '\(([0-9]+)\s*VND\)'), '')::bigint,
                cp.price::bigint,
                t.amount::bigint
            )
        ELSE 0::bigint
    END)::bigint AS paid_vnd,
    (CASE
        WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN t.amount::bigint
        WHEN lower(t."from") = 'system' OR lower(COALESCE(t.description, '')) LIKE '%refund%' THEN abs(t.amount)::bigint
        ELSE -abs(t.amount)::bigint
    END)::bigint AS coin_amount,
    t.created_at
FROM public.transactions t
LEFT JOIN public.users u ON u.id = (
    CASE
        WHEN (
            CASE
                WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN t."to"
                WHEN t."from" = '' OR lower(t."from") = 'system' THEN t."to"
                ELSE t."from"
            END
        ) ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
        THEN (
            CASE
                WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN t."to"
                WHEN t."from" = '' OR lower(t."from") = 'system' THEN t."to"
                ELSE t."from"
            END
        )::uuid
        ELSE NULL
    END
)
LEFT JOIN LATERAL (
    SELECT cp_inner.name, cp_inner.price, cp_inner.amount
    FROM public.coin_packages cp_inner
    WHERE t.payos_order_code IS NOT NULL AND t.payos_order_code <> ''
      AND (
          cp_inner.name = btrim(regexp_replace(regexp_replace(COALESCE(t.description, ''), '^Deposit:\s*', ''), '\s*\([0-9]+\s*VND\)\s*$', ''))
          OR cp_inner.amount = t.amount
      )
    ORDER BY
        CASE
            WHEN cp_inner.name = btrim(regexp_replace(regexp_replace(COALESCE(t.description, ''), '^Deposit:\s*', ''), '\s*\([0-9]+\s*VND\)\s*$', '')) THEN 0
            ELSE 1
        END,
        cp_inner.price ASC
    LIMIT 1
) cp ON true
WHERE t.id = @id;

-- name: GetAdminBillingSummary :one
WITH enriched AS (
    SELECT
        t.id,
        t.amount,
        COALESCE(t.description, '') AS description,
        t.status::text AS status,
        COALESCE(t.payos_order_code, '') AS payos_order_code,
        CASE
            WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN 'topup'
            WHEN lower(t."from") = 'system' OR lower(COALESCE(t.description, '')) LIKE '%refund%' THEN 'refund'
            ELSE 'spend'
        END AS tx_type,
        COALESCE(u.id::text,
            CASE
                WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN t."to"
                WHEN t."from" = '' OR lower(t."from") = 'system' THEN t."to"
                ELSE t."from"
            END
        )::text AS user_id,
        COALESCE(u.name, '') AS user_name,
        COALESCE(u.email, '') AS user_email,
        (CASE
            WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN
                COALESCE(
                    NULLIF(btrim(regexp_replace(regexp_replace(COALESCE(t.description, ''), '^Deposit:\s*', ''), '\s*\([0-9]+\s*VND\)\s*$', '')), ''),
                    cp.name,
                    'Coin Top-up'
                )
            ELSE COALESCE(NULLIF(btrim(t.description), ''), 'Coin Transaction')
        END)::text AS package_or_feature,
        (CASE
            WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN
                COALESCE(
                    NULLIF(substring(COALESCE(t.description, '') from '\(([0-9]+)\s*VND\)'), '')::bigint,
                    cp.price::bigint,
                    t.amount::bigint
                )
            ELSE 0::bigint
        END)::bigint AS paid_vnd,
        (CASE
            WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN t.amount::bigint
            WHEN lower(t."from") = 'system' OR lower(COALESCE(t.description, '')) LIKE '%refund%' THEN abs(t.amount)::bigint
            ELSE -abs(t.amount)::bigint
        END)::bigint AS coin_amount
    FROM public.transactions t
    LEFT JOIN public.users u ON u.id = (
        CASE
            WHEN (
                CASE
                    WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN t."to"
                    WHEN t."from" = '' OR lower(t."from") = 'system' THEN t."to"
                    ELSE t."from"
                END
            ) ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
            THEN (
                CASE
                    WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN t."to"
                    WHEN t."from" = '' OR lower(t."from") = 'system' THEN t."to"
                    ELSE t."from"
                END
            )::uuid
            ELSE NULL
        END
    )
    LEFT JOIN LATERAL (
        SELECT cp_inner.name, cp_inner.price, cp_inner.amount
        FROM public.coin_packages cp_inner
        WHERE t.payos_order_code IS NOT NULL AND t.payos_order_code <> ''
          AND (
              cp_inner.name = btrim(regexp_replace(regexp_replace(COALESCE(t.description, ''), '^Deposit:\s*', ''), '\s*\([0-9]+\s*VND\)\s*$', ''))
              OR cp_inner.amount = t.amount
          )
        ORDER BY
            CASE
                WHEN cp_inner.name = btrim(regexp_replace(regexp_replace(COALESCE(t.description, ''), '^Deposit:\s*', ''), '\s*\([0-9]+\s*VND\)\s*$', '')) THEN 0
                ELSE 1
            END,
            cp_inner.price ASC
        LIMIT 1
    ) cp ON true
    WHERE (sqlc.narg(date_from)::timestamptz IS NULL OR t.created_at >= sqlc.narg(date_from)::timestamptz)
      AND (sqlc.narg(date_to)::timestamptz IS NULL OR t.created_at < sqlc.narg(date_to)::timestamptz)
),
filtered AS (
    SELECT *
    FROM enriched
    WHERE (sqlc.narg(search)::text IS NULL OR sqlc.narg(search)::text = '' OR (
        id::text ILIKE '%' || sqlc.narg(search)::text || '%' ESCAPE '\'
        OR user_name ILIKE '%' || sqlc.narg(search)::text || '%' ESCAPE '\'
        OR user_email ILIKE '%' || sqlc.narg(search)::text || '%' ESCAPE '\'
        OR user_id ILIKE '%' || sqlc.narg(search)::text || '%' ESCAPE '\'
        OR description ILIKE '%' || sqlc.narg(search)::text || '%' ESCAPE '\'
        OR payos_order_code ILIKE '%' || sqlc.narg(search)::text || '%' ESCAPE '\'
    ))
      AND (sqlc.narg(status)::text IS NULL OR sqlc.narg(status)::text = '' OR status = sqlc.narg(status)::text)
      AND (sqlc.narg(tx_type)::text IS NULL OR sqlc.narg(tx_type)::text = '' OR tx_type = sqlc.narg(tx_type)::text)
      AND (sqlc.narg(package_name)::text IS NULL OR sqlc.narg(package_name)::text = '' OR package_or_feature ILIKE '%' || sqlc.narg(package_name)::text || '%' ESCAPE '\')
      AND (sqlc.narg(amount_min)::bigint IS NULL OR (CASE WHEN tx_type = 'topup' THEN paid_vnd ELSE abs(coin_amount) END) >= sqlc.narg(amount_min)::bigint)
      AND (sqlc.narg(amount_max)::bigint IS NULL OR (CASE WHEN tx_type = 'topup' THEN paid_vnd ELSE abs(coin_amount) END) <= sqlc.narg(amount_max)::bigint)
)
SELECT
    COALESCE(SUM(paid_vnd) FILTER (WHERE status = 'success' AND tx_type = 'topup'), 0)::bigint AS total_revenue,
    COUNT(*) FILTER (WHERE status = 'success' AND tx_type = 'topup')::bigint AS successful_deposits,
    COUNT(*) FILTER (WHERE tx_type = 'topup')::bigint AS total_deposits,
    COUNT(*) FILTER (WHERE status = 'success')::bigint AS success_count,
    COUNT(*) FILTER (WHERE status = 'pending')::bigint AS pending_count,
    COUNT(*) FILTER (WHERE status = 'failed')::bigint AS failed_count,
    COUNT(*) FILTER (WHERE status = 'cancelled')::bigint AS cancelled_count,
    COUNT(*)::bigint AS total_transactions,
    COUNT(DISTINCT NULLIF(user_id, '')) FILTER (WHERE status = 'success' AND tx_type = 'topup')::bigint AS unique_payers,
    COALESCE(SUM(coin_amount) FILTER (WHERE status = 'success' AND tx_type = 'topup'), 0)::bigint AS coins_sold,
    COALESCE(SUM(abs(coin_amount)) FILTER (WHERE status = 'success' AND tx_type = 'spend'), 0)::bigint AS coins_spent,
    COALESCE(SUM(abs(coin_amount)) FILTER (WHERE status = 'success' AND tx_type = 'refund'), 0)::bigint AS coins_refunded
FROM filtered;

-- name: GetAdminPackageRevenueBreakdown :many
WITH enriched AS (
    SELECT
        t.id,
        t.amount,
        COALESCE(t.description, '') AS description,
        t.status::text AS status,
        COALESCE(t.payos_order_code, '') AS payos_order_code,
        CASE
            WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN 'topup'
            WHEN lower(t."from") = 'system' OR lower(COALESCE(t.description, '')) LIKE '%refund%' THEN 'refund'
            ELSE 'spend'
        END AS tx_type,
        COALESCE(u.id::text,
            CASE
                WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN t."to"
                WHEN t."from" = '' OR lower(t."from") = 'system' THEN t."to"
                ELSE t."from"
            END
        )::text AS user_id,
        COALESCE(u.name, '') AS user_name,
        COALESCE(u.email, '') AS user_email,
        (CASE
            WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN
                COALESCE(
                    NULLIF(btrim(regexp_replace(regexp_replace(COALESCE(t.description, ''), '^Deposit:\s*', ''), '\s*\([0-9]+\s*VND\)\s*$', '')), ''),
                    cp.name,
                    'Coin Top-up'
                )
            ELSE COALESCE(NULLIF(btrim(t.description), ''), 'Coin Transaction')
        END)::text AS package_or_feature,
        (CASE
            WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN
                COALESCE(
                    NULLIF(substring(COALESCE(t.description, '') from '\(([0-9]+)\s*VND\)'), '')::bigint,
                    cp.price::bigint,
                    t.amount::bigint
                )
            ELSE 0::bigint
        END)::bigint AS paid_vnd,
        (CASE
            WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN t.amount::bigint
            WHEN lower(t."from") = 'system' OR lower(COALESCE(t.description, '')) LIKE '%refund%' THEN abs(t.amount)::bigint
            ELSE -abs(t.amount)::bigint
        END)::bigint AS coin_amount
    FROM public.transactions t
    LEFT JOIN public.users u ON u.id = (
        CASE
            WHEN (
                CASE
                    WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN t."to"
                    WHEN t."from" = '' OR lower(t."from") = 'system' THEN t."to"
                    ELSE t."from"
                END
            ) ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
            THEN (
                CASE
                    WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN t."to"
                    WHEN t."from" = '' OR lower(t."from") = 'system' THEN t."to"
                    ELSE t."from"
                END
            )::uuid
            ELSE NULL
        END
    )
    LEFT JOIN LATERAL (
        SELECT cp_inner.name, cp_inner.price, cp_inner.amount
        FROM public.coin_packages cp_inner
        WHERE t.payos_order_code IS NOT NULL AND t.payos_order_code <> ''
          AND (
              cp_inner.name = btrim(regexp_replace(regexp_replace(COALESCE(t.description, ''), '^Deposit:\s*', ''), '\s*\([0-9]+\s*VND\)\s*$', ''))
              OR cp_inner.amount = t.amount
          )
        ORDER BY
            CASE
                WHEN cp_inner.name = btrim(regexp_replace(regexp_replace(COALESCE(t.description, ''), '^Deposit:\s*', ''), '\s*\([0-9]+\s*VND\)\s*$', '')) THEN 0
                ELSE 1
            END,
            cp_inner.price ASC
        LIMIT 1
    ) cp ON true
    WHERE (sqlc.narg(date_from)::timestamptz IS NULL OR t.created_at >= sqlc.narg(date_from)::timestamptz)
      AND (sqlc.narg(date_to)::timestamptz IS NULL OR t.created_at < sqlc.narg(date_to)::timestamptz)
)
SELECT
    package_or_feature AS package_name,
    COALESCE(SUM(paid_vnd), 0)::bigint AS revenue_vnd,
    COUNT(*)::bigint AS deposit_count,
    COALESCE(SUM(coin_amount), 0)::bigint AS coins_sold
FROM enriched
WHERE status = 'success'
  AND tx_type = 'topup'
  AND (sqlc.narg(search)::text IS NULL OR sqlc.narg(search)::text = '' OR (
      id::text ILIKE '%' || sqlc.narg(search)::text || '%' ESCAPE '\'
      OR user_name ILIKE '%' || sqlc.narg(search)::text || '%' ESCAPE '\'
      OR user_email ILIKE '%' || sqlc.narg(search)::text || '%' ESCAPE '\'
      OR user_id ILIKE '%' || sqlc.narg(search)::text || '%' ESCAPE '\'
      OR description ILIKE '%' || sqlc.narg(search)::text || '%' ESCAPE '\'
      OR payos_order_code ILIKE '%' || sqlc.narg(search)::text || '%' ESCAPE '\'
  ))
  AND (sqlc.narg(package_name)::text IS NULL OR sqlc.narg(package_name)::text = '' OR package_or_feature ILIKE '%' || sqlc.narg(package_name)::text || '%' ESCAPE '\')
  AND (sqlc.narg(amount_min)::bigint IS NULL OR paid_vnd >= sqlc.narg(amount_min)::bigint)
  AND (sqlc.narg(amount_max)::bigint IS NULL OR paid_vnd <= sqlc.narg(amount_max)::bigint)
GROUP BY package_or_feature
ORDER BY revenue_vnd DESC, package_name ASC;

-- name: GetAdminFeatureCoinBreakdown :many
WITH enriched AS (
    SELECT
        t.id,
        t.amount,
        COALESCE(t.description, '') AS description,
        t.status::text AS status,
        COALESCE(t.payos_order_code, '') AS payos_order_code,
        CASE
            WHEN t.payos_order_code IS NOT NULL AND t.payos_order_code <> '' THEN 'topup'
            WHEN lower(t."from") = 'system' OR lower(COALESCE(t.description, '')) LIKE '%refund%' THEN 'refund'
            ELSE 'spend'
        END AS tx_type,
        COALESCE(u.id::text, t."from")::text AS user_id,
        COALESCE(u.name, '') AS user_name,
        COALESCE(u.email, '') AS user_email,
        COALESCE(NULLIF(btrim(t.description), ''), 'Coin Transaction')::text AS feature_name,
        abs(t.amount)::bigint AS coins_spent
    FROM public.transactions t
    LEFT JOIN public.users u ON u.id = (
        CASE
            WHEN t."from" ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
            THEN t."from"::uuid
            ELSE NULL
        END
    )
    WHERE (sqlc.narg(date_from)::timestamptz IS NULL OR t.created_at >= sqlc.narg(date_from)::timestamptz)
      AND (sqlc.narg(date_to)::timestamptz IS NULL OR t.created_at < sqlc.narg(date_to)::timestamptz)
)
SELECT
    feature_name,
    COALESCE(SUM(coins_spent), 0)::bigint AS coins_spent,
    COUNT(*)::bigint AS spend_count
FROM enriched
WHERE status = 'success'
  AND tx_type = 'spend'
  AND (sqlc.narg(search)::text IS NULL OR sqlc.narg(search)::text = '' OR (
      id::text ILIKE '%' || sqlc.narg(search)::text || '%' ESCAPE '\'
      OR user_name ILIKE '%' || sqlc.narg(search)::text || '%' ESCAPE '\'
      OR user_email ILIKE '%' || sqlc.narg(search)::text || '%' ESCAPE '\'
      OR user_id ILIKE '%' || sqlc.narg(search)::text || '%' ESCAPE '\'
      OR description ILIKE '%' || sqlc.narg(search)::text || '%' ESCAPE '\'
      OR payos_order_code ILIKE '%' || sqlc.narg(search)::text || '%' ESCAPE '\'
  ))
  AND (sqlc.narg(package_name)::text IS NULL OR sqlc.narg(package_name)::text = '' OR feature_name ILIKE '%' || sqlc.narg(package_name)::text || '%' ESCAPE '\')
  AND (sqlc.narg(amount_min)::bigint IS NULL OR coins_spent >= sqlc.narg(amount_min)::bigint)
  AND (sqlc.narg(amount_max)::bigint IS NULL OR coins_spent <= sqlc.narg(amount_max)::bigint)
GROUP BY feature_name
ORDER BY coins_spent DESC, feature_name ASC;
