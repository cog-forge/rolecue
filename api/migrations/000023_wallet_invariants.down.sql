BEGIN;

ALTER TABLE public.inventories DROP CONSTRAINT inventories_slot_max;

ALTER TABLE public.coin_packages
    DROP CONSTRAINT coin_packages_amount_positive,
    DROP CONSTRAINT coin_packages_price_positive,
    DROP COLUMN is_active;

ALTER TABLE public.transactions
    DROP CONSTRAINT transactions_to_party_check,
    DROP CONSTRAINT transactions_from_party_check,
    DROP CONSTRAINT transactions_amount_positive;

ALTER TABLE public.wallets DROP CONSTRAINT wallets_balance_nonnegative;

COMMIT;
