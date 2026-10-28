import { z } from "zod";

// ─── Transaction Statuses & Enums ──────────────────────────────────────────

export const adminTransactionStatusSchema = z.enum([
  "pending",
  "success",
  "failed",
  "cancelled",
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

// ─── Enriched Read-Only Transaction Item ───────────────────────────────────

export const adminTransactionItemSchema = z.object({
  id: z.string().min(1),
  from: z.string().default(""),
  to: z.string().default(""),
  user_id: z.string().default(""),
  user_name: z.string().default(""),
  user_email: z.string().default(""),
  type: z.string().default(""),
  package_or_feature: z.string().default(""),
  paid_vnd: z.number().default(0),
  coin_amount: z.number().default(0),
  amount: z.number().int(),
  currency: z.string().default("VND"),
  description: z.string().default(""),
  status: adminTransactionStatusSchema,
  payos_order_code: z.number().nullable().optional(),
  created_at: z.string().default(""),
});

export type AdminTransactionItem = z.infer<typeof adminTransactionItemSchema>;

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

export const adminTransactionItemEnvelopeSchema = z.object({
  success: z.boolean(),
  data: adminTransactionItemSchema,
});

// ─── Package & Feature Breakdowns (Real Ledger Data Only) ──────────────────

export const packageRevenueItemSchema = z.object({
  package_name: z.string(),
  revenue_vnd: z.number().default(0),
  deposit_count: z.number().default(0),
  coins_sold: z.number().default(0),
});

export type PackageRevenueItem = z.infer<typeof packageRevenueItemSchema>;

export const featureCoinItemSchema = z.object({
  feature_name: z.string(),
  coins_spent: z.number().default(0),
  spend_count: z.number().default(0),
});

export type FeatureCoinItem = z.infer<typeof featureCoinItemSchema>;

// ─── Shared Billing & Revenue Summary (One Source of Truth) ────────────────

export const billingSummarySchema = z.object({
  total_revenue: z.number().default(0),
  successful_deposits: z.number().default(0),
  total_deposits: z.number().default(0),
  success_count: z.number().default(0),
  pending_count: z.number().default(0),
  failed_count: z.number().default(0),
  cancelled_count: z.number().default(0),
  total_transactions: z.number().default(0),
  average_deposit: z.number().default(0),
  unique_payers: z.number().default(0),
  coins_sold: z.number().default(0),
  coins_spent: z.number().default(0),
  coins_refunded: z.number().default(0),
  coins_net: z.number().default(0),
  success_rate: z.number().default(0),
  package_breakdown: z.array(packageRevenueItemSchema).default([]),
  feature_breakdown: z.array(featureCoinItemSchema).default([]),
});

export type BillingSummary = z.infer<typeof billingSummarySchema>;

export const billingSummaryEnvelopeSchema = z.object({
  success: z.boolean(),
  data: billingSummarySchema,
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
  dateFrom?: string; // RFC 3339, inclusive
  dateTo?: string; // RFC 3339, exclusive
  sortBy?: string;
  sortOrder?: "asc" | "desc";
};

// ─── Revenue report with a period comparison ───────────────────────────────

export const revenueCompareSchema = z.enum([
  "previous_year",
  "previous_period",
]);
export type RevenueCompare = z.infer<typeof revenueCompareSchema>;

const revenueWindowSchema = z.object({ from: z.string(), to: z.string() });

export const revenuePeriodMetricsSchema = z.object({
  revenue_vnd: z.number(),
  successful_topups: z.number(),
  topup_attempts: z.number(),
  payment_success_rate: z.number(),
  paying_users: z.number(),
  new_payers: z.number(),
  average_order_vnd: z.number(),
  coins_sold: z.number(),
  coins_spent: z.number(),
  coins_refunded: z.number(),
});
export type RevenuePeriodMetrics = z.infer<typeof revenuePeriodMetricsSchema>;

const revenueMixItemSchema = z.object({
  key: z.string(),
  revenue_vnd: z.number(),
  comparison_revenue_vnd: z.number(),
  topups: z.number(),
});
export type RevenueMixItem = z.infer<typeof revenueMixItemSchema>;

export const revenueReportSchema = z.object({
  compare: revenueCompareSchema,
  granularity: z.enum(["day", "month"]),
  window: revenueWindowSchema,
  comparison_window: revenueWindowSchema,
  current: revenuePeriodMetricsSchema,
  comparison: revenuePeriodMetricsSchema,
  series: z.array(
    z.object({
      start: z.string(),
      comparison_start: z.string(),
      revenue_vnd: z.number(),
      comparison_revenue_vnd: z.number(),
    }),
  ),
  by_role: z.array(revenueMixItemSchema),
  by_package: z.array(revenueMixItemSchema),
  spend_by_feature: z.array(
    z.object({
      feature: z.string(),
      coins: z.number(),
      comparison_coins: z.number(),
      charges: z.number(),
    }),
  ),
  outstanding_coins: z.number(),
  outstanding_value_vnd: z.number(),
});
export type RevenueReport = z.infer<typeof revenueReportSchema>;

export const revenueReportEnvelopeSchema = z.object({
  success: z.boolean(),
  data: revenueReportSchema,
});
