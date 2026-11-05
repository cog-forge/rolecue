BEGIN;

-- Database invariants for coins and money (KAN-266). Every CHECK is added NOT VALID, which enforces it
-- for new and updated rows at once without scanning, and is then validated only when no existing row
-- breaks it. A deployment with a legacy bad row therefore keeps starting; the row stays visible to
-- an operator because the constraint is left unvalidated.

ALTER TABLE public.wallets
    ADD CONSTRAINT wallets_balance_nonnegative CHECK (balance >= 0) NOT VALID;

-- Every coin and VND movement is a positive amount; the direction is in "from" and "to".
ALTER TABLE public.transactions
    ADD CONSTRAINT transactions_amount_positive CHECK (amount > 0) NOT VALID;

-- "from" and "to" hold a user id, the system side of the ledger ('system'), or the empty string that the
-- admin billing queries read as the system side for rows written before the ledger existed.
ALTER TABLE public.transactions
    ADD CONSTRAINT transactions_from_party_check CHECK (
        "from" IN ('', 'system')
        OR "from" ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
    ) NOT VALID,
    ADD CONSTRAINT transactions_to_party_check CHECK (
        "to" IN ('', 'system')
        OR "to" ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
    ) NOT VALID;

ALTER TABLE public.coin_packages
    ADD COLUMN is_active boolean NOT NULL DEFAULT true,
    ADD CONSTRAINT coin_packages_price_positive CHECK (price > 0) NOT VALID,
    ADD CONSTRAINT coin_packages_amount_positive CHECK (amount > 0) NOT VALID;

-- MaxInventorySlots in the inventory domain package is the same number.
ALTER TABLE public.inventories
    ADD CONSTRAINT inventories_slot_max CHECK (slot <= 100) NOT VALID;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM public.wallets WHERE NOT (balance >= 0)) THEN
        ALTER TABLE public.wallets VALIDATE CONSTRAINT wallets_balance_nonnegative;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM public.transactions WHERE NOT (amount > 0)) THEN
        ALTER TABLE public.transactions VALIDATE CONSTRAINT transactions_amount_positive;
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM public.transactions
        WHERE NOT ("from" IN ('', 'system') OR "from" ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$')
    ) THEN
        ALTER TABLE public.transactions VALIDATE CONSTRAINT transactions_from_party_check;
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM public.transactions
        WHERE NOT ("to" IN ('', 'system') OR "to" ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$')
    ) THEN
        ALTER TABLE public.transactions VALIDATE CONSTRAINT transactions_to_party_check;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM public.coin_packages WHERE NOT (price > 0)) THEN
        ALTER TABLE public.coin_packages VALIDATE CONSTRAINT coin_packages_price_positive;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM public.coin_packages WHERE NOT (amount > 0)) THEN
        ALTER TABLE public.coin_packages VALIDATE CONSTRAINT coin_packages_amount_positive;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM public.inventories WHERE NOT (slot <= 100)) THEN
        ALTER TABLE public.inventories VALIDATE CONSTRAINT inventories_slot_max;
    END IF;
END $$;

COMMIT;
