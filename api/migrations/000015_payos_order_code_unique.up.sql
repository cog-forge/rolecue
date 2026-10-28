BEGIN;

-- One checkout is one PayOS order. The status guard under FOR UPDATE already makes a duplicate webhook
-- credit once; this index makes a second row for the same order impossible (KAN-245). Rows without an
-- order code (the internal COIN movements) and the empty string some readers treat as "none" are left out.
CREATE UNIQUE INDEX transactions_payos_order_code_key
    ON public.transactions (payos_order_code)
    WHERE payos_order_code IS NOT NULL AND payos_order_code <> '';

COMMIT;
