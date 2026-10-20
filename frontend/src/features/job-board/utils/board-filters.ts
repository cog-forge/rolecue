// The board filters live in the URL so reload, back and sharing keep them.
export const BOARD_PAGE_SIZE = 20;
// Go rejects a filter longer than 200 characters.
export const FILTER_MAX = 200;
// Go takes the offset as an int32, so the page stays within it.
const PAGE_MAX = Math.floor(2_147_483_647 / BOARD_PAGE_SIZE) + 1;
export type BoardFilters = Readonly<{
  search: string;
  location: string;
  type: string;
  page: number;
}>;
export type SearchParams = Record<string, string | string[] | undefined>;

export const defaultBoardFilters: BoardFilters = {
  search: "",
  location: "",
  type: "",
  page: 1,
};

function single(value: string | string[] | undefined) {
  return typeof value === "string" ? value : null;
}

export function isFilterTooLong(value: string) {
  return Array.from(value.trim()).length > FILTER_MAX;
}

function text(value: string | string[] | undefined) {
  const trimmed = (single(value) ?? "").trim();
  return isFilterTooLong(trimmed) ? "" : trimmed;
}

// Unknown or malformed values fall back to the default instead of failing the page.
export function parseBoardFilters(params: SearchParams): BoardFilters {
  const page = Number(single(params.page));
  return {
    search: text(params.q),
    location: text(params.location),
    type: text(params.type),
    page: Number.isInteger(page) && page > 0 && page <= PAGE_MAX ? page : 1,
  };
}

export function hasTextFilters(filters: BoardFilters) {
  return (
    filters.search !== "" || filters.location !== "" || filters.type !== ""
  );
}

// URL query for the board; defaults are left out to keep links short.
export function boardSearch(filters: BoardFilters) {
  const params = new URLSearchParams();
  if (filters.search) params.set("q", filters.search);
  if (filters.location) params.set("location", filters.location);
  if (filters.type) params.set("type", filters.type);
  if (filters.page > 1) params.set("page", String(filters.page));
  const query = params.toString();
  return query ? `?${query}` : "";
}

export function boardApiParams(filters: BoardFilters) {
  const params: Record<string, string | number> = {
    limit: BOARD_PAGE_SIZE,
    offset: (filters.page - 1) * BOARD_PAGE_SIZE,
  };
  if (filters.search) params.search = filters.search;
  if (filters.location) params.location = filters.location;
  if (filters.type) params.employment_type = filters.type;
  return params;
}
