// The catalog filters live in the URL so reload, back and sharing keep them.
export const VOICE_PAGE_SIZE = 9;
// Matches domain.MaxVoiceQueryRunes in the Go API.
export const MAX_VOICE_QUERY = 100;
// Keeps the API offset inside int32 even for a hand-edited ?page=.
const MAX_PAGE = 100_000;
export const voiceStatuses = ["all", "active", "inactive"] as const;
export type VoiceStatus = (typeof voiceStatuses)[number];
export type VoiceFilters = Readonly<{
  status: VoiceStatus;
  q: string;
  page: number;
}>;
export type SearchParams = Record<string, string | string[] | undefined>;

export const defaultVoiceFilters: VoiceFilters = {
  status: "all",
  q: "",
  page: 1,
};

function single(value: string | string[] | undefined) {
  return typeof value === "string" ? value : null;
}

// Unknown or malformed values fall back to the default instead of failing the page.
export function parseVoiceFilters(params: SearchParams): VoiceFilters {
  const status = single(params.status);
  const rawPage = single(params.page) ?? "";
  const page = /^\d{1,6}$/.test(rawPage) ? Number(rawPage) : 1;
  const q = (single(params.q) ?? "").trim();
  return {
    status: voiceStatuses.includes(status as VoiceStatus)
      ? (status as VoiceStatus)
      : "all",
    q: Array.from(q).length <= MAX_VOICE_QUERY ? q : "",
    page: page >= 1 && page <= MAX_PAGE ? page : 1,
  };
}

// URL query for the catalog page; defaults are left out to keep links short.
export function voiceSearch(filters: VoiceFilters) {
  const params = new URLSearchParams();
  if (filters.status !== "all") params.set("status", filters.status);
  if (filters.q) params.set("q", filters.q);
  if (filters.page > 1) params.set("page", String(filters.page));
  const query = params.toString();
  return query ? `?${query}` : "";
}

export function voiceApiParams(filters: VoiceFilters) {
  const params: Record<string, string | number> = {
    status: filters.status,
    limit: VOICE_PAGE_SIZE,
    offset: (filters.page - 1) * VOICE_PAGE_SIZE,
  };
  if (filters.q) params.q = filters.q;
  return params;
}
