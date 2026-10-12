import { postingStatuses, type PostingStatus } from "../schemas/moderation-schema";

// The queue filters live in the URL so reload, back and sharing keep them.
export const QUEUE_PAGE_SIZE = 20;
export type QueueStatus = PostingStatus | "all";
export type QueueFilters = Readonly<{
  status: QueueStatus;
  recruiterId: string | null;
  from: string | null; // YYYY-MM-DD, inclusive
  to: string | null; // YYYY-MM-DD, inclusive
  page: number;
}>;
export type SearchParams = Record<string, string | string[] | undefined>;

export const defaultQueueFilters: QueueFilters = {
  status: "pending",
  recruiterId: null,
  from: null,
  to: null,
  page: 1,
};

const datePattern = /^\d{4}-\d{2}-\d{2}$/;
const uuidPattern =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

function single(value: string | string[] | undefined) {
  return typeof value === "string" ? value : null;
}
function validDate(value: string | null) {
  if (!value || !datePattern.test(value)) return null;
  const date = new Date(`${value}T00:00:00Z`);
  return Number.isNaN(date.getTime()) || date.toISOString().slice(0, 10) !== value
    ? null
    : value;
}

// Unknown or malformed values fall back to the default instead of failing the page.
export function parseQueueFilters(params: SearchParams): QueueFilters {
  const status = single(params.status);
  const page = Number(single(params.page));
  const recruiterId = single(params.recruiter_id);
  return {
    status:
      status === "all" || postingStatuses.includes(status as PostingStatus)
        ? (status as QueueStatus)
        : "pending",
    recruiterId: recruiterId && uuidPattern.test(recruiterId) ? recruiterId : null,
    from: validDate(single(params.from)),
    to: validDate(single(params.to)),
    page: Number.isInteger(page) && page > 0 ? page : 1,
  };
}

// URL query for the queue page; defaults are left out to keep links short.
export function queueSearch(filters: QueueFilters) {
  const params = new URLSearchParams();
  if (filters.status !== "pending") params.set("status", filters.status);
  if (filters.recruiterId) params.set("recruiter_id", filters.recruiterId);
  if (filters.from) params.set("from", filters.from);
  if (filters.to) params.set("to", filters.to);
  if (filters.page > 1) params.set("page", String(filters.page));
  const query = params.toString();
  return query ? `?${query}` : "";
}

// Go expects RFC 3339 and a [from, to) window, so an inclusive "to" date ends at the next midnight.
export function queueApiParams(filters: QueueFilters) {
  const params: Record<string, string | number> = {
    status: filters.status,
    limit: QUEUE_PAGE_SIZE,
    offset: (filters.page - 1) * QUEUE_PAGE_SIZE,
  };
  if (filters.recruiterId) params.recruiter_id = filters.recruiterId;
  if (filters.from) params.from = `${filters.from}T00:00:00Z`;
  if (filters.to) {
    const next = new Date(`${filters.to}T00:00:00Z`);
    next.setUTCDate(next.getUTCDate() + 1);
    params.to = next.toISOString().replace(".000Z", "Z");
  }
  return params;
}
