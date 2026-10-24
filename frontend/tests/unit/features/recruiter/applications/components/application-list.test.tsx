import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import { ApplicationList } from "@/features/recruiter/applications/components/application-list";
import {
  applicationSearch,
  defaultApplicationFilters,
  type ApplicationFilters,
} from "@/features/recruiter/applications/utils/application-filters";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  search: new URLSearchParams(),
  pathname: "/recruiter/applications",
  user: {
    id: "11111111-1111-4111-8111-111111111111",
    full_name: "Rec",
    email: "rec@example.com",
    role: "recruiter" as const,
    email_verified: true,
    is_locked: false,
    image: null,
  },
}));
vi.mock("@/lib/api/client", () => ({ apiClient: { get: mocks.get } }));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));
vi.mock("next/navigation", () => ({
  useSearchParams: () => mocks.search,
  usePathname: () => mocks.pathname,
}));

const postingId = "33333333-3333-4333-8333-333333333333";
const application = (i: number, over: Record<string, unknown> = {}) => ({
  id: `44444444-4444-4444-8444-${String(i).padStart(12, "0")}`,
  job_posting_id: postingId,
  job_posting_title: "Backend Engineer",
  status: "pending",
  submitted_at: "2026-10-20T08:00:00Z",
  candidate: {
    id: `55555555-5555-4555-8555-${String(i).padStart(12, "0")}`,
    name: `Candidate ${i}`,
    email: `candidate${i}@example.com`,
    image: null,
  },
  interview_eligible: false,
  interview_deadline: null,
  reject_reason: null,
  updated_at: "2026-10-20T08:00:00Z",
  ...over,
});
const page = (
  items: ReturnType<typeof application>[],
  total = items.length,
  offset = 0,
) => ({
  data: {
    success: true,
    data: { items, pagination: { limit: 20, offset, total } },
  },
});
function failure(status: number) {
  return new AxiosError("failure", "ERR_BAD_RESPONSE", undefined, undefined, {
    status,
    statusText: "Error",
    data: {
      success: false,
      error: { code: "INTERNAL_ERROR", message: "boom" },
    },
    headers: new AxiosHeaders(),
    config: { headers: new AxiosHeaders() },
  });
}
function mount(
  props: { postingId?: string } = {},
  filters: ApplicationFilters = defaultApplicationFilters,
) {
  mocks.search = new URLSearchParams(applicationSearch(filters));
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  });
  client.setQueryData(["auth", "me"], mocks.user);
  render(
    <QueryClientProvider client={client}>
      <ApplicationList {...props} />
    </QueryClientProvider>,
  );
  return client;
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.get.mockReset();
  mocks.pathname = "/recruiter/applications";
});

describe("ApplicationList", () => {
  it("lists one posting's applications with status, deadline and reject reason", async () => {
    mocks.pathname = `/recruiter/job-postings/${postingId}`;
    mocks.get.mockResolvedValue(
      page([
        application(1),
        application(2, {
          status: "interview_eligible",
          interview_eligible: true,
          interview_deadline: "2026-10-21T08:30:00Z",
        }),
        application(3, {
          status: "interview_eligible",
          interview_eligible: false,
          interview_deadline: "2026-10-19T08:30:00Z",
        }),
        application(4, { status: "rejected", reject_reason: "Not a fit" }),
      ]),
    );
    mount({ postingId });
    expect(screen.getByRole("status")).toHaveTextContent(
      /loading applications/i,
    );
    const table = await screen.findByRole("table");
    expect(within(table).getByText("Candidate 1")).toBeInTheDocument();
    expect(
      within(table).getByText("candidate1@example.com"),
    ).toBeInTheDocument();
    expect(within(table).getByText("Awaiting CV review")).toBeInTheDocument();
    expect(within(table).getByText(/Interview by/)).toHaveTextContent(
      "Oct 21, 2026",
    );
    expect(within(table).getByText(/Deadline passed/)).toBeInTheDocument();
    expect(within(table).getByText("Rejected")).toBeInTheDocument();
    expect(within(table).getByText("Reason: Not a fit")).toBeInTheDocument();
    // The posting is already known, and a row opens nothing until Application Detail exists.
    expect(
      within(table).queryByRole("columnheader", { name: "Job posting" }),
    ).not.toBeInTheDocument();
    expect(within(table).queryByRole("link")).not.toBeInTheDocument();
    expect(mocks.get).toHaveBeenCalledWith(
      `/job-postings/${postingId}/applications`,
      { params: { limit: 20, offset: 0 }, signal: expect.any(AbortSignal) },
    );
    expect(screen.getByText("Showing 1–4 of 4")).toBeInTheDocument();
  });

  it("lists every posting's applications with a link to each posting", async () => {
    mocks.get.mockResolvedValue(page([application(1)]));
    mount();
    const table = await screen.findByRole("table");
    expect(
      within(table).getByRole("columnheader", { name: "Job posting" }),
    ).toBeInTheDocument();
    expect(
      within(table).getByRole("link", { name: "Backend Engineer" }),
    ).toHaveAttribute("href", `/recruiter/job-postings/${postingId}`);
    expect(mocks.get).toHaveBeenCalledWith("/recruiter/applications", {
      params: { limit: 20, offset: 0 },
      signal: expect.any(AbortSignal),
    });
  });

  it("sends the URL filters to Go", async () => {
    mocks.get.mockResolvedValue(page([application(21)], 41, 20));
    mount(
      {},
      {
        status: "rejected",
        search: "linh",
        page: 2,
        posting: postingId,
      },
    );
    await screen.findByRole("table");
    expect(mocks.get).toHaveBeenCalledWith("/recruiter/applications", {
      params: {
        limit: 20,
        offset: 20,
        status: "rejected",
        q: "linh",
        job_posting_id: postingId,
      },
      signal: expect.any(AbortSignal),
    });
    expect(
      screen.getByText(/Showing the applications of one job posting/),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Show all my postings" }),
    ).toHaveAttribute("href", "/recruiter/applications?status=rejected&q=linh");
    expect(screen.getByText("Showing 21–40 of 41")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Previous" })).toHaveAttribute(
      "href",
      `/recruiter/applications?status=rejected&q=linh&posting=${postingId}`,
    );
    expect(screen.getByRole("link", { name: "Next" })).toHaveAttribute(
      "href",
      `/recruiter/applications?status=rejected&q=linh&posting=${postingId}&page=3`,
    );
  });

  it("ignores a posting filter in the URL on a posting's own list", async () => {
    mocks.pathname = `/recruiter/job-postings/${postingId}`;
    mocks.get.mockResolvedValue(page([application(1)]));
    mount(
      { postingId },
      {
        ...defaultApplicationFilters,
        posting: "99999999-9999-4999-8999-999999999999",
      },
    );
    await screen.findByRole("table");
    expect(mocks.get).toHaveBeenCalledWith(
      `/job-postings/${postingId}/applications`,
      { params: { limit: 20, offset: 0 }, signal: expect.any(AbortSignal) },
    );
    expect(
      screen.queryByText(/Showing the applications of one job posting/),
    ).not.toBeInTheDocument();
  });

  it("exposes the search form by role and name, with one Search button inside it", async () => {
    mocks.get.mockResolvedValue(page([application(1)]));
    mount();
    await screen.findByRole("table");
    // The E2E specs reach the button through this form: the workspace also has a "Search pages"
    // button, so a bare name match is ambiguous there.
    const form = screen.getByRole("search", { name: "Search applications" });
    expect(
      within(form).getByRole("button", { name: "Search" }),
    ).toHaveAttribute("type", "submit");
    expect(
      within(form).getByLabelText("Search by name or email"),
    ).toBeVisible();
  });

  it("marks the active status tab and links the others", async () => {
    mocks.get.mockResolvedValue(page([application(1)]));
    mount({}, { ...defaultApplicationFilters, status: "approved" });
    await screen.findByRole("table");
    const tabs = screen.getByRole("navigation", {
      name: "Filter applications by status",
    });
    expect(
      within(tabs).getByRole("link", { name: "Approved" }),
    ).toHaveAttribute("aria-current", "page");
    expect(
      within(tabs).getByRole("link", { name: "Interview eligible" }),
    ).toHaveAttribute(
      "href",
      "/recruiter/applications?status=interview_eligible",
    );
    expect(within(tabs).getByRole("link", { name: "All" })).toHaveAttribute(
      "href",
      "/recruiter/applications",
    );
  });

  it("explains an empty list, and offers a way back when it is filtered", async () => {
    mocks.get.mockResolvedValue(page([]));
    mount();
    expect(await screen.findByText("No applications yet.")).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Show all applications" }),
    ).not.toBeInTheDocument();
  });

  it("lets 'show all applications' drop the one-posting narrowing too", async () => {
    mocks.get.mockResolvedValue(page([]));
    mount(
      {},
      { ...defaultApplicationFilters, search: "zzz", posting: postingId },
    );
    expect(
      await screen.findByText("No applications match “zzz”."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Show all applications" }),
    ).toHaveAttribute("href", "/recruiter/applications");
  });

  it("names the search when nothing matches it", async () => {
    mocks.get.mockResolvedValue(page([]));
    mount({}, { ...defaultApplicationFilters, search: "zzz" });
    expect(
      await screen.findByText("No applications match “zzz”."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Show all applications" }),
    ).toHaveAttribute("href", "/recruiter/applications");
    expect(
      screen.getByRole("link", { name: "Clear search" }),
    ).toBeInTheDocument();
  });

  it("offers the first page when the page is past the end", async () => {
    mocks.get.mockResolvedValue(page([], 5, 40));
    mount({}, { ...defaultApplicationFilters, page: 3 });
    expect(
      await screen.findByText("This page is past the end of the list."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Go to the first page" }),
    ).toHaveAttribute("href", "/recruiter/applications");
  });

  it("shows an error with a retry", async () => {
    mocks.get.mockRejectedValueOnce(failure(500));
    mount();
    expect(
      await screen.findByText("We couldn't load the applications"),
    ).toBeInTheDocument();
    mocks.get.mockResolvedValue(page([application(1)]));
    screen.getByRole("button", { name: "Try again" }).click();
    expect(await screen.findByRole("table")).toBeInTheDocument();
  });

  it("treats a missing posting as not found, with no retry", async () => {
    mocks.get.mockRejectedValue(failure(404));
    mount({ postingId });
    expect(
      await screen.findByText("We couldn't find this job posting"),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Try again" }),
    ).not.toBeInTheDocument();
  });

  it("re-checks the session when access is denied", async () => {
    mocks.get.mockRejectedValue(failure(403));
    const client = mount();
    expect(
      await screen.findByText("You don't have access to applications"),
    ).toBeInTheDocument();
    expect(client.getQueryState(["auth", "me"])?.isInvalidated).toBe(true);
  });
});
