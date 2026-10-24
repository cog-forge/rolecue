-- Migration: 000013_billing_redesign.up.sql
-- Add fields to transactions for billing redesign, audit logging, and links.

ALTER TABLE public.transactions
  ADD COLUMN IF NOT EXISTS is_test BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS original_transaction_id UUID REFERENCES public.transactions(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS coin_clawed_back BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS failure_reason TEXT,
  ADD COLUMN IF NOT EXISTS payos_payment_link_id TEXT,
  ADD COLUMN IF NOT EXISTS balance_before INTEGER,
  ADD COLUMN IF NOT EXISTS balance_after INTEGER;

-- Flag legacy test duplicate orders as test transactions
UPDATE public.transactions
SET is_test = TRUE
WHERE payos_order_code IN ('100201', '100202');

-- Create billing audit logs table
CREATE TABLE IF NOT EXISTS public.billing_audit_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  admin_id UUID NOT NULL REFERENCES public.users(id),
  action TEXT NOT NULL,
  transaction_id UUID REFERENCES public.transactions(id) ON DELETE SET NULL,
  reason TEXT NOT NULL,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for billing filters and performance
CREATE INDEX IF NOT EXISTS idx_transactions_is_test ON public.transactions(is_test);
CREATE INDEX IF NOT EXISTS idx_transactions_orig_id ON public.transactions(original_transaction_id);
CREATE INDEX IF NOT EXISTS idx_transactions_payos_order_code ON public.transactions(payos_order_code);
CREATE INDEX IF NOT EXISTS idx_billing_audit_logs_tx ON public.billing_audit_logs(transaction_id);
CREATE INDEX IF NOT EXISTS idx_billing_audit_logs_created_at ON public.billing_audit_logs(created_at DESC);
