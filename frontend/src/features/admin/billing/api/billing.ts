import { apiClient } from "@/lib/api/client";
import {
  adminTransactionPageEnvelopeSchema,
  billingSummaryEnvelopeSchema,
  billingDailySeriesEnvelopeSchema,
  revenueReportEnvelopeSchema,
  adminTransactionItemSchema,
  type AdminTransactionQuery,
  type BillingSummary,
  type BillingDailyItem,
  type AdminTransactionPage,
  type AdminTransactionItem,
} from "../schemas/billing-schema";

export const adminBillingKeys = {
  all: ["admin", "billing"] as const,
  summary: (filters: Record<string, unknown>) =>
    [...adminBillingKeys.all, "summary", filters] as const,
  daily: (filters: Record<string, unknown>) =>
    [...adminBillingKeys.all, "daily", filters] as const,
  transactions: (query: AdminTransactionQuery) =>
    [...adminBillingKeys.all, "transactions", query] as const,
  transactionDetail: (id: string) =>
    [...adminBillingKeys.all, "transaction", id] as const,
  revenue: (dateFrom: string, dateTo: string) =>
    [...adminBillingKeys.all, "revenue", dateFrom, dateTo] as const,
};

function buildParams(
  query: AdminTransactionQuery,
): Record<string, string | number | boolean> {
  const params: Record<string, string | number | boolean> = {
    page: query.page || 1,
    page_size: query.pageSize || 20,
  };
  if (query.search) params.search = query.search;
  if (query.status && query.status !== "all") params.status = query.status;
  if (query.type && query.type !== "all") params.type = query.type;
  if (query.tab && query.tab !== "all") params.type = query.tab;
  if (query.package && query.package !== "all") params.package = query.package;
  if (query.amountMin != null) params.amount_min = query.amountMin;
  if (query.amountMax != null) params.amount_max = query.amountMax;
  if (query.dateFrom) params.date_from = query.dateFrom;
  if (query.dateTo) params.date_to = query.dateTo;
  if (query.includeTest) params.include_test = true;
  if (query.sortBy) params.sort_by = query.sortBy;
  if (query.sortOrder) params.sort_order = query.sortOrder;

  return params;
}

export async function getBillingSummary(
  filter: Partial<AdminTransactionQuery> = {},
  signal?: AbortSignal,
): Promise<BillingSummary> {
  const params = buildParams(filter as AdminTransactionQuery);
  const response = await apiClient.get<unknown>("/admin/billing/summary", {
    params,
    signal,
  });
  return billingSummaryEnvelopeSchema.parse(response.data).data;
}

export async function getBillingDailySeries(
  filter: Partial<AdminTransactionQuery> = {},
  signal?: AbortSignal,
): Promise<BillingDailyItem[]> {
  const params = buildParams(filter as AdminTransactionQuery);
  const response = await apiClient.get<unknown>("/admin/billing/daily", {
    params,
    signal,
  });
  return billingDailySeriesEnvelopeSchema.parse(response.data).data;
}

export async function listAdminTransactions(
  query: AdminTransactionQuery,
  signal?: AbortSignal,
): Promise<AdminTransactionPage> {
  const params = buildParams(query);
  const response = await apiClient.get<unknown>("/admin/transactions", {
    params,
    signal,
  });
  return adminTransactionPageEnvelopeSchema.parse(response.data).data;
}

export async function getBillingTransactionByID(
  id: string,
  signal?: AbortSignal,
): Promise<AdminTransactionItem> {
  const response = await apiClient.get<{
    success: boolean;
    data: AdminTransactionItem;
  }>(`/admin/billing/transactions/${id}`, { signal });
  return adminTransactionItemSchema.parse(response.data.data);
}

export async function syncPayOSTransaction(
  id: string,
): Promise<AdminTransactionItem> {
  const response = await apiClient.post<{
    success: boolean;
    data: AdminTransactionItem;
  }>(`/admin/billing/transactions/${id}/sync`);
  return adminTransactionItemSchema.parse(response.data.data);
}

export async function cancelTransaction(
  id: string,
  reason: string,
): Promise<AdminTransactionItem> {
  const response = await apiClient.post<{
    success: boolean;
    data: AdminTransactionItem;
  }>(`/admin/billing/transactions/${id}/cancel`, { reason });
  return adminTransactionItemSchema.parse(response.data.data);
}

export async function issueRefund(
  id: string,
  data: {
    user_id: string;
    amount: number;
    currency: string;
    reason: string;
    clawback_coins: boolean;
  },
): Promise<AdminTransactionItem> {
  const response = await apiClient.post<{
    success: boolean;
    data: AdminTransactionItem;
  }>(`/admin/billing/transactions/${id}/refund`, data);
  return adminTransactionItemSchema.parse(response.data.data);
}

export function getExportCSVUrl(
  filter: Partial<AdminTransactionQuery>,
): string {
  const params = new URLSearchParams();
  const built = buildParams(filter as AdminTransactionQuery);
  Object.entries(built).forEach(([key, val]) => {
    params.set(key, String(val));
  });
  return `/api/v1/admin/billing/export?${params.toString()}`;
}

// Backward-compatible method
export async function getAdminRevenueReport(
  dateFrom: string,
  dateTo: string,
  signal?: AbortSignal,
) {
  const params: Record<string, string> = {};
  if (dateFrom) params.date_from = dateFrom;
  if (dateTo) params.date_to = dateTo;

  const response = await apiClient.get<unknown>("/admin/revenue", {
    params,
    signal,
  });
  return revenueReportEnvelopeSchema.parse(response.data).data;
}
