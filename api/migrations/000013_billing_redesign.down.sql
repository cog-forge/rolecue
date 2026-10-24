-- Migration: 000013_billing_redesign.down.sql

DROP TABLE IF EXISTS public.billing_audit_logs;

ALTER TABLE public.transactions
  DROP COLUMN IF EXISTS balance_after,
  DROP COLUMN IF EXISTS balance_before,
  DROP COLUMN IF EXISTS payos_payment_link_id,
  DROP COLUMN IF EXISTS failure_reason,
  DROP COLUMN IF EXISTS coin_clawed_back,
  DROP COLUMN IF EXISTS original_transaction_id,
  DROP COLUMN IF EXISTS is_test;
