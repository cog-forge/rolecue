import { apiClient } from "@/lib/api/client";
import {
  adminTransactionPageEnvelopeSchema,
  adminTransactionItemEnvelopeSchema,
  billingSummaryEnvelopeSchema,
  revenueReportEnvelopeSchema,
  type RevenueCompare,
  type RevenueReport,
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
  revenueReport: (query: RevenueReportQuery) =>
    [...adminBillingKeys.all, "revenue-report", query] as const,
};

export type RevenueReportQuery = {
  dateFrom: string;
  dateTo: string;
  compare: RevenueCompare;
};

export async function getRevenueReport(
  query: RevenueReportQuery,
  signal?: AbortSignal,
): Promise<RevenueReport> {
  const response = await apiClient.get<unknown>(
    "/admin/billing/revenue-report",
    {
      params: {
        date_from: query.dateFrom,
        date_to: query.dateTo,
        compare: query.compare,
      },
      signal,
    },
  );
  return revenueReportEnvelopeSchema.parse(response.data).data;
}

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
  if (query.dateFrom) params.date_from = query.dateFrom;
  if (query.dateTo) params.date_to = query.dateTo;
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

export type BillingCSVExportResult = {
  csv: string;
  totalCount: number;
  truncated: boolean;
};

export async function exportBillingCSV(
  filter: Partial<AdminTransactionQuery> = {},
): Promise<BillingCSVExportResult> {
  const params = buildBillingParams({
    ...filter,
    page: 1,
    pageSize: 5000,
  });
  const response = await apiClient.get<string>("/admin/billing/export", {
    params,
    responseType: "text",
  });
  const headers = (response.headers ?? {}) as Record<string, unknown>;
  const rawTruncated = String(
    headers["x-export-truncated"] ?? "",
  ).toLowerCase();
  const rawTotal = Number(headers["x-total-count"] ?? 0);
  const csv =
    typeof response.data === "string"
      ? response.data
      : String(response.data ?? "");
  return {
    csv,
    totalCount: Number.isFinite(rawTotal) ? rawTotal : 0,
    truncated: rawTruncated === "true",
  };
}
