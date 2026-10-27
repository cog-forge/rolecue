import { routes } from "@/config/routes";
import type { ReportType } from "@/features/report/schemas/report-schema";

export type HistoryTypeFilter = ReportType | "all";

export interface HistoryFilters {
  type: ReportType | undefined;
  page: number;
}

export const historyTypeFilters: readonly HistoryTypeFilter[] = [
  "all",
  "practice",
  "recruitment",
];

export const historyTypeLabel: Record<HistoryTypeFilter, string> = {
  all: "All interviews",
  practice: "Practice",
  recruitment: "Recruitment",
};

// Anything not a known type or a positive whole page number falls back to "all" and page 1.
export function parseHistoryFilters(params: URLSearchParams): HistoryFilters {
  const type = params.get("type");
  const page = Number(params.get("page"));
  return {
    type: type === "practice" || type === "recruitment" ? type : undefined,
    page: Number.isSafeInteger(page) && page > 0 ? page : 1,
  };
}

export function historyHref(filters: HistoryFilters) {
  const search = new URLSearchParams();
  if (filters.type) search.set("type", filters.type);
  if (filters.page > 1) search.set("page", String(filters.page));
  const query = search.toString();
  return query ? `${routes.history}?${query}` : routes.history;
}
