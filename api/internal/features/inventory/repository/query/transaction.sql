-- name: CreateSlotPurchaseTransaction :exec
INSERT INTO public.transactions ("from", "to", amount, currency, description, status)
VALUES ($1, 'system', $2, 'COIN', 'Inventory slot purchase', 'success');
