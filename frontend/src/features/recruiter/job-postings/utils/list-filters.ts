import {
  postingStatuses,
  type PostingStatus,
} from "../schemas/job-posting-schema";

// The list filters live in the URL so reload, back and sharing keep them.
export const LIST_PAGE_SIZE = 20;
export const SEARCH_MAX = 200;
// Go takes the offset as an int32, so the page stays within it.
const PAGE_MAX = Math.floor(2_147_483_647 / LIST_PAGE_SIZE) + 1;
export type ListStatus = PostingStatus | "all";
export type ListFilters = Readonly<{
  status: ListStatus;
  search: string;
  page: number;
}>;
export type SearchParams = Record<string, string | string[] | undefined>;

export const defaultListFilters: ListFilters = {
  status: "all",
  search: "",
  page: 1,
};

function single(value: string | string[] | undefined) {
  return typeof value === "string" ? value : null;
}

// Unknown or malformed values fall back to the default instead of failing the page.
export function parseListFilters(params: SearchParams): ListFilters {
  const status = single(params.status);
  const page = Number(single(params.page));
  const search = (single(params.q) ?? "").trim();
  return {
    status: postingStatuses.includes(status as PostingStatus)
      ? (status as PostingStatus)
      : "all",
    search: Array.from(search).length <= SEARCH_MAX ? search : "",
    page: Number.isInteger(page) && page > 0 && page <= PAGE_MAX ? page : 1,
  };
}

// URL query for the list page; defaults are left out to keep links short.
export function listSearch(filters: ListFilters) {
  const params = new URLSearchParams();
  if (filters.status !== "all") params.set("status", filters.status);
  if (filters.search) params.set("q", filters.search);
  if (filters.page > 1) params.set("page", String(filters.page));
  const query = params.toString();
  return query ? `?${query}` : "";
}

export function listApiParams(filters: ListFilters) {
  const params: Record<string, string | number> = {
    limit: LIST_PAGE_SIZE,
    offset: (filters.page - 1) * LIST_PAGE_SIZE,
  };
  if (filters.status !== "all") params.status = filters.status;
  if (filters.search) params.search = filters.search;
  return params;
}
