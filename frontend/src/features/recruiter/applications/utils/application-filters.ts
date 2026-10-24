import {
  applicationStatuses,
  type ApplicationStatus,
} from "../schemas/application-schema";

// The list filters live in the URL so reload, back and sharing keep them.
export const APPLICATION_PAGE_SIZE = 20;
export const SEARCH_MAX = 200;
// Go takes the offset as an int32, so the page stays within it.
const PAGE_MAX = Math.floor(2_147_483_647 / APPLICATION_PAGE_SIZE) + 1;
const UUID =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

export type ApplicationStatusFilter = ApplicationStatus | "all";
export type ApplicationFilters = Readonly<{
  status: ApplicationStatusFilter;
  search: string;
  page: number;
  // Only the list across all postings narrows to one posting this way.
  posting: string | null;
}>;
export type SearchParams = Record<string, string | string[] | undefined>;

export const defaultApplicationFilters: ApplicationFilters = {
  status: "all",
  search: "",
  page: 1,
  posting: null,
};

function single(value: string | string[] | undefined) {
  return typeof value === "string" ? value : null;
}

// Unknown or malformed values fall back to the default instead of failing the page.
export function parseApplicationFilters(
  params: SearchParams,
): ApplicationFilters {
  const status = single(params.status);
  const page = Number(single(params.page));
  const search = (single(params.q) ?? "").trim();
  const posting = single(params.posting);
  return {
    status: applicationStatuses.find((value) => value === status) ?? "all",
    search: Array.from(search).length <= SEARCH_MAX ? search : "",
    page: Number.isInteger(page) && page > 0 && page <= PAGE_MAX ? page : 1,
    posting: posting && UUID.test(posting) ? posting.toLowerCase() : null,
  };
}

// URL query for the page; defaults are left out to keep links short.
export function applicationSearch(filters: ApplicationFilters) {
  const params = new URLSearchParams();
  if (filters.status !== "all") params.set("status", filters.status);
  if (filters.search) params.set("q", filters.search);
  if (filters.posting) params.set("posting", filters.posting);
  if (filters.page > 1) params.set("page", String(filters.page));
  const query = params.toString();
  return query ? `?${query}` : "";
}

// The request query. The posting filter is sent only when the caller asks for it, because the list
// of one posting takes its posting from the path.
export function applicationApiParams(
  filters: ApplicationFilters,
  withPosting: boolean,
) {
  const params: Record<string, string | number> = {
    limit: APPLICATION_PAGE_SIZE,
    offset: (filters.page - 1) * APPLICATION_PAGE_SIZE,
  };
  if (filters.status !== "all") params.status = filters.status;
  if (filters.search) params.q = filters.search;
  if (withPosting && filters.posting) params.job_posting_id = filters.posting;
  return params;
}
