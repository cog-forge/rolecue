import { apiClient } from "@/lib/api/client";
import {
  adminTransactionPageEnvelopeSchema,
  adminTransactionItemEnvelopeSchema,
  billingSummaryEnvelopeSchema,
  type AdminTransactionQuery,
  type BillingSummary,
  type AdminTransactionPage,
  type AdminTransactionItem,
} from "../schemas/billing-schema";

export const adminBillingKeys = {
  all: ["admin", "billing"] as const,
  summary: (filters: Record<string, unknown>) =>
    [...adminBillingKeys.all, "summary", filters] as const,
  transactions: (query: AdminTransactionQuery) =>
    [...adminBillingKeys.all, "transactions", query] as const,
  transactionDetail: (id: string) =>
    [...adminBillingKeys.all, "transaction", id] as const,
};

export function buildBillingParams(
  query: Partial<AdminTransactionQuery>,
): Record<string, string | number> {
  const params: Record<string, string | number> = {};
  if (query.page != null) params.page = query.page;
  if (query.pageSize != null) params.page_size = query.pageSize;
  if (query.search && query.search.trim()) params.search = query.search.trim();
  if (query.status && query.status !== "all") params.status = query.status;

  const effectiveType =
    query.type && query.type !== "all"
      ? query.type
      : query.tab && query.tab !== "all"
        ? query.tab
        : undefined;
  if (effectiveType) params.type = effectiveType;

  if (query.package && query.package !== "all") params.package = query.package;
  if (query.amountMin != null) params.amount_min = query.amountMin;
  if (query.amountMax != null) params.amount_max = query.amountMax;
  if (query.sortBy) params.sort_by = query.sortBy;
  if (query.sortOrder) params.sort_order = query.sortOrder;

  return params;
}

export async function getBillingSummary(
  filter: Partial<AdminTransactionQuery> = {},
  signal?: AbortSignal,
): Promise<BillingSummary> {
  const params = buildBillingParams(filter);
  const response = await apiClient.get<unknown>("/admin/billing/summary", {
    params,
    signal,
  });
  return billingSummaryEnvelopeSchema.parse(response.data).data;
}

export async function listAdminTransactions(
  query: AdminTransactionQuery,
  signal?: AbortSignal,
): Promise<AdminTransactionPage> {
  const params = buildBillingParams({
    ...query,
    page: query.page || 1,
    pageSize: query.pageSize || 20,
  });
  const response = await apiClient.get<unknown>("/admin/billing/transactions", {
    params,
    signal,
  });
  return adminTransactionPageEnvelopeSchema.parse(response.data).data;
}

export async function getBillingTransactionByID(
  id: string,
  signal?: AbortSignal,
): Promise<AdminTransactionItem> {
  const response = await apiClient.get<unknown>(
    `/admin/billing/transactions/${id}`,
    { signal },
  );
  return adminTransactionItemEnvelopeSchema.parse(response.data).data;
}

export async function exportBillingCSV(
  filter: Partial<AdminTransactionQuery> = {},
): Promise<string> {
  const params = buildBillingParams({
    ...filter,
    page: 1,
    pageSize: 5000,
  });
  const response = await apiClient.get<string>("/admin/billing/export", {
    params,
    responseType: "text",
  });
  return typeof response.data === "string"
    ? response.data
    : String(response.data ?? "");
}
