import { apiClient } from "@/lib/api/client";
import {
  listApiParams,
  type ListStatus,
} from "@/features/recruiter/job-postings/utils/list-filters";
import {
  countPageSchema,
  deadlinePageSchema,
  type Todo,
} from "../schemas/dashboard-schema";

export const dashboardKeys = {
  all: (userId: string) => ["recruiter-dashboard", userId] as const,
  postings: (userId: string, status: ListStatus) =>
    [...dashboardKeys.all(userId), "postings", status] as const,
  applications: (userId: string, status: "pending") =>
    [...dashboardKeys.all(userId), "applications", status] as const,
  deadlines: (userId: string) =>
    [...dashboardKeys.all(userId), "deadlines"] as const,
};

// An Application's interview deadline counts as soon when it falls inside this window.
export const SOON_MS = 24 * 60 * 60 * 1000;
// The most eligible Applications the API returns in one page.
export const DEADLINE_SCAN = 100;

const exact = (count: number): Todo => ({
  count,
  atLeast: false,
  detail: null,
});

// Counts postings the way the postings list filters them, so each row's link shows the same rows.
export async function countPostings(status: ListStatus, signal?: AbortSignal) {
  const response = await apiClient.get<unknown>("/job-postings", {
    params: {
      ...listApiParams({ status, search: "", page: 1 }),
      limit: 1,
      offset: 0,
    },
    signal,
  });
  return exact(countPageSchema.parse(response.data).data.pagination.total);
}

export async function countPendingApplications(signal?: AbortSignal) {
  const response = await apiClient.get<unknown>("/recruiter/applications", {
    params: { status: "pending", limit: 1, offset: 0 },
    signal,
  });
  return exact(countPageSchema.parse(response.data).data.pagination.total);
}

// The API cannot filter by deadline, so the first page of eligible Applications is counted here.
// When there are more than one page, the count is a lower bound and says so. detail is the
// earliest deadline in the window, as an ISO time.
export async function summarizeDeadlines(
  signal?: AbortSignal,
  now = Date.now(),
): Promise<Todo> {
  const response = await apiClient.get<unknown>("/recruiter/applications", {
    params: { status: "interview_eligible", limit: DEADLINE_SCAN, offset: 0 },
    signal,
  });
  const { items, pagination } = deadlinePageSchema.parse(response.data).data;
  const soon = items
    .filter((item) => item.interview_eligible && item.interview_deadline)
    .map((item) => Date.parse(item.interview_deadline as string))
    .filter((deadline) => deadline > now && deadline <= now + SOON_MS)
    .sort((a, b) => a - b);
  return {
    count: soon.length,
    atLeast: pagination.total > items.length,
    detail: soon.length > 0 ? new Date(soon[0]).toISOString() : null,
  };
}
