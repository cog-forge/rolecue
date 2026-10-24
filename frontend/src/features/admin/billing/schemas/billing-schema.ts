import { z } from "zod";

// ─── Transaction Statuses & Enums ──────────────────────────────────────────

export const adminTransactionStatusSchema = z.enum([
  "pending",
  "success",
  "failed",
  "cancelled",
  "refunded",
  "expired",
]);

export type AdminTransactionStatus = z.infer<
  typeof adminTransactionStatusSchema
>;

export const transactionTypeSchema = z.enum([
  "all",
  "topup",
  "spend",
  "refund",
]);
export type TransactionType = z.infer<typeof transactionTypeSchema>;

// ─── User in Transaction ───────────────────────────────────────────────────

export const adminTransactionUserSchema = z.object({
  id: z.string(),
  name: z.string().default(""),
  email: z.string().default(""),
  role: z.string().default("candidate"),
});

export type AdminTransactionUser = z.infer<typeof adminTransactionUserSchema>;

// ─── Enriched Transaction Item (New API) ───────────────────────────────────

export const adminTransactionItemSchema = z.object({
  id: z.string().min(1),
  from: z.string().nullable().optional(),
  to: z.string().nullable().optional(),
  amount: z.number().int(),
  currency: z.string().default("COIN"),
  paid_vnd: z.number().nullable().optional(),
  coins: z.number().nullable().optional(),
  type: z.string().default(""),
  package_or_feature: z.string().default(""),
  description: z.string().nullable().optional(),
  status: adminTransactionStatusSchema,
  failure_reason: z.string().nullable().optional(),
  payos_order_code: z.number().nullable().optional(),
  payos_payment_link_id: z.string().nullable().optional(),
  balance_before: z.number().nullable().optional(),
  balance_after: z.number().nullable().optional(),
  original_transaction_id: z.string().nullable().optional(),
  coin_clawed_back: z.boolean().default(false),
  is_test: z.boolean().default(false),
  user: adminTransactionUserSchema.nullable().optional(),
  created_at: z.string().optional(),
  updated_at: z.string().optional(),
});

export type AdminTransactionItem = z.infer<typeof adminTransactionItemSchema>;

// Legacy type alias for backwards compatibility
export const adminTransactionSchema = adminTransactionItemSchema;
export type AdminTransaction = AdminTransactionItem;

export const adminTransactionPageSchema = z.object({
  items: z.array(adminTransactionItemSchema),
  total: z.number().int(),
  page: z.number().int(),
  page_size: z.number().int(),
});

export type AdminTransactionPage = z.infer<typeof adminTransactionPageSchema>;

export const adminTransactionPageEnvelopeSchema = z.object({
  success: z.boolean(),
  data: adminTransactionPageSchema,
});

// ─── Billing Summary (Unified One-Source-of-Truth) ─────────────────────────

export const billingReconciliationSchema = z.object({
  matched_count: z.number().default(0),
  matched_amount_vnd: z.number().default(0),
  unmatched_count: z.number().default(0),
  unmatched_amount_vnd: z.number().default(0),
  last_synced_at: z.string().nullable().optional(),
});

export type BillingReconciliation = z.infer<typeof billingReconciliationSchema>;

export const billingSummaryComparisonSchema = z.object({
  prev_net_revenue: z.number().default(0),
  net_revenue_change_pct: z.number().default(0),
  prev_successful_deposits: z.number().default(0),
  successful_deposits_change_pct: z.number().default(0),
  prev_pending_count: z.number().default(0),
  pending_count_change_pct: z.number().default(0),
  prev_failed_count: z.number().default(0),
  failed_count_change_pct: z.number().default(0),
  prev_other_count: z.number().default(0),
  other_count_change_pct: z.number().default(0),
  prev_average_deposit_vnd: z.number().default(0),
  average_deposit_change_pct: z.number().default(0),
  prev_unique_payers: z.number().default(0),
  unique_payers_change_pct: z.number().default(0),
});

export type BillingSummaryComparison = z.infer<
  typeof billingSummaryComparisonSchema
>;

export const billingSummarySchema = z.object({
  total_revenue: z.number().default(0),
  net_revenue: z.number().default(0),
  gross_revenue: z.number().default(0),
  total_refunds_vnd: z.number().default(0),
  refund_count: z.number().default(0),
  successful_deposits: z.number().default(0),
  pending_count: z.number().default(0),
  failed_count: z.number().default(0),
  cancelled_count: z.number().default(0),
  other_count: z.number().default(0),
  total_transactions: z.number().default(0),
  success_rate: z.number().default(0),
  average_deposit_vnd: z.number().default(0),
  unique_payers: z.number().default(0),
  new_payers: z.number().default(0),
  returning_payers: z.number().default(0),
  coins_sold: z.number().default(0),
  coins_spent: z.number().default(0),
  outstanding_coins: z.number().default(0),
  average_daily_revenue: z.number().default(0),
  peak_day_date: z.string().default(""),
  peak_day_revenue: z.number().default(0),
  reconciliation: billingReconciliationSchema.default({
    matched_count: 0,
    matched_amount_vnd: 0,
    unmatched_count: 0,
    unmatched_amount_vnd: 0,
  }),
  comparison: billingSummaryComparisonSchema.default({
    prev_net_revenue: 0,
    net_revenue_change_pct: 0,
    prev_successful_deposits: 0,
    successful_deposits_change_pct: 0,
    prev_pending_count: 0,
    pending_count_change_pct: 0,
    prev_failed_count: 0,
    failed_count_change_pct: 0,
    prev_other_count: 0,
    other_count_change_pct: 0,
    prev_average_deposit_vnd: 0,
    average_deposit_change_pct: 0,
    prev_unique_payers: 0,
    unique_payers_change_pct: 0,
  }),
});

export type BillingSummary = z.infer<typeof billingSummarySchema>;

export const billingSummaryEnvelopeSchema = z.object({
  success: z.boolean(),
  data: billingSummarySchema,
});

// ─── Daily Series (All Days in Period) ─────────────────────────────────────

export const billingDailyItemSchema = z.object({
  date: z.string(), // YYYY-MM-DD
  revenue: z.number().default(0),
  gross_revenue: z.number().default(0),
  refunds_vnd: z.number().default(0),
  deposit_count: z.number().default(0),
  unique_payers: z.number().default(0),
  coins_sold: z.number().default(0),
  coins_spent: z.number().default(0),
  packages_breakdown: z.record(z.string(), z.number()).default({}),
  features_breakdown: z.record(z.string(), z.number()).default({}),
  previous_period_revenue: z.number().nullable().optional(),
});

export type BillingDailyItem = z.infer<typeof billingDailyItemSchema>;

export const billingDailySeriesEnvelopeSchema = z.object({
  success: z.boolean(),
  data: z.array(billingDailyItemSchema),
});

// ─── Filter & Query Definitions ────────────────────────────────────────────

export const transactionStatusFilterSchema = z.enum([
  "all",
  "pending",
  "success",
  "failed",
  "cancelled",
]);

export type TransactionStatusFilter = z.infer<
  typeof transactionStatusFilterSchema
>;

export const transactionTypeFilterSchema = z.enum([
  "all",
  "topup",
  "spend",
  "refund",
]);

export type TransactionTypeFilter = z.infer<typeof transactionTypeFilterSchema>;

export type DatePreset =
  "today" | "7d" | "30d" | "90d" | "thisMonth" | "lastMonth" | "custom";

export type AdminTransactionQuery = {
  tab?: "all" | "topup" | "spend" | "refund";
  page: number;
  pageSize?: number;
  search?: string;
  status?: string;
  type?: string;
  package?: string;
  amountMin?: number;
  amountMax?: number;
  dateFrom?: string;
  dateTo?: string;
  includeTest?: boolean;
  sortBy?: string;
  sortOrder?: "asc" | "desc";
};

// ─── Backward-Compatible Revenue Types ─────────────────────────────────────

export const revenueStatsSchema = z.object({
  total_revenue: z.number(),
  successful_count: z.number(),
  pending_count: z.number(),
  failed_count: z.number(),
  cancelled_count: z.number(),
  total_count: z.number(),
});

export type RevenueStats = z.infer<typeof revenueStatsSchema>;

export const revenueSummaryItemSchema = z.object({
  period: z.string(),
  revenue: z.number(),
  successful_deposits: z.number(),
});

export type RevenueSummaryItem = z.infer<typeof revenueSummaryItemSchema>;

export const revenueReportSchema = z.object({
  stats: revenueStatsSchema,
  summary: z.array(revenueSummaryItemSchema),
});

export type RevenueReport = z.infer<typeof revenueReportSchema>;

export const revenueReportEnvelopeSchema = z.object({
  success: z.boolean(),
  data: revenueReportSchema,
});
