import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import { JobPostingList } from "@/features/recruiter/job-postings/components/job-posting-list";
import {
  defaultListFilters,
  listSearch,
} from "@/features/recruiter/job-postings/utils/list-filters";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  search: new URLSearchParams(),
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
}));

const summary = (
  i: number,
  status = "pending",
  reject_reason: string | null = null,
) => ({
  id: `33333333-3333-4333-8333-${String(i).padStart(12, "0")}`,
  title: `Posting ${i + 1}`,
  status,
  reject_reason,
  location: "Hanoi",
  employment_type: "Full-time",
  interview_slot: null,
  core_question_count: 3,
  review_submitted: true,
  created_at: "2026-10-12T08:00:00Z",
});
const page = (items: ReturnType<typeof summary>[], total = items.length) => ({
  data: {
    success: true,
    data: { items, pagination: { limit: 20, offset: 0, total } },
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
function mount(filters = defaultListFilters) {
  mocks.search = new URLSearchParams(listSearch(filters));
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  });
  client.setQueryData(["auth", "me"], mocks.user);
  render(
    <QueryClientProvider client={client}>
      <JobPostingList />
    </QueryClientProvider>,
  );
  return client;
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.get.mockReset();
});

describe("JobPostingList", () => {
  it("loads, then lists own postings with status, reject reason and links", async () => {
    mocks.get.mockResolvedValue(
      page([summary(0), summary(1, "rejected", "Salary is missing")]),
    );
    mount();
    expect(screen.getByRole("status")).toHaveTextContent(
      /loading your job postings/i,
    );
    const table = await screen.findByRole("table");
    expect(within(table).getByText("Pending review")).toBeInTheDocument();
    expect(within(table).getByText("Rejected")).toBeInTheDocument();
    expect(
      within(table).getByText("Reason: Salary is missing"),
    ).toBeInTheDocument();
    expect(
      within(table).getByRole("link", { name: "Edit Posting 1" }),
    ).toHaveAttribute(
      "href",
      "/recruiter/job-postings/33333333-3333-4333-8333-000000000000",
    );
    expect(
      within(table).getByRole("link", { name: "Question bank for Posting 2" }),
    ).toHaveAttribute(
      "href",
      "/recruiter/job-postings/33333333-3333-4333-8333-000000000001/questions",
    );
    expect(mocks.get).toHaveBeenCalledWith("/job-postings", {
      params: { limit: 20, offset: 0 },
      signal: expect.any(AbortSignal),
    });
    expect(screen.getByText("Showing 1–2 of 2")).toBeInTheDocument();
  });

  it("offers View instead of Edit once a posting is approved", async () => {
    mocks.get.mockResolvedValue(page([summary(0, "open")]));
    mount();
    expect(
      await screen.findByRole("link", { name: "View Posting 1" }),
    ).toBeInTheDocument();
  });

  it("sends status, search and page to the API and keeps them in links", async () => {
    mocks.get.mockResolvedValue(page([summary(0)], 45));
    mount({ status: "pending", search: "Go", page: 2 });
    await screen.findByRole("table");
    expect(mocks.get).toHaveBeenCalledWith("/job-postings", {
      params: { limit: 20, offset: 20, status: "pending", search: "Go" },
      signal: expect.any(AbortSignal),
    });
    expect(screen.getByRole("link", { name: "Rejected" })).toHaveAttribute(
      "href",
      "/recruiter/job-postings?status=rejected&q=Go",
    );
    expect(screen.getByRole("link", { name: "Next" })).toHaveAttribute(
      "href",
      "/recruiter/job-postings?status=pending&q=Go&page=3",
    );
    expect(screen.getByRole("link", { name: "Pending" })).toHaveAttribute(
      "aria-current",
      "page",
    );
  });

  it("submits a trimmed search to the URL", async () => {
    mocks.get.mockResolvedValue(page([]));
    mount();
    fireEvent.change(await screen.findByLabelText("Search by title"), {
      target: { value: "  Go dev " },
    });
    const push = vi.spyOn(window.history, "pushState");
    fireEvent.click(screen.getByRole("button", { name: "Search" }));
    expect(push).toHaveBeenCalledWith(
      null,
      "",
      "/recruiter/job-postings?q=Go+dev",
    );
    push.mockRestore();
  });

  it("changes tabs through the History API instead of a route navigation", async () => {
    mocks.get
      .mockResolvedValueOnce(page([summary(0)]))
      .mockReturnValueOnce(new Promise(() => {}));
    mount();
    await screen.findByRole("table");
    const push = vi
      .spyOn(window.history, "pushState")
      .mockImplementation((_data, _unused, url) => {
        mocks.search = new URL(String(url), "http://localhost").searchParams;
      });
    fireEvent.click(screen.getByRole("link", { name: "Open" }));
    expect(push).toHaveBeenCalledWith(
      null,
      "",
      "/recruiter/job-postings?status=open",
    );
    push.mockRestore();
  });

  it("shows a first-time empty state with a create link", async () => {
    mocks.get.mockResolvedValue(page([]));
    mount();
    expect(
      await screen.findByText("You haven't created a job posting yet."),
    ).toBeInTheDocument();
    expect(
      screen.getAllByRole("link", { name: "Create job posting" })[1],
    ).toHaveAttribute("href", "/recruiter/job-postings/new");
  });

  it("shows a filtered empty state with a way back to all postings", async () => {
    mocks.get.mockResolvedValue(page([]));
    mount({ status: "all", search: "Rust", page: 1 });
    expect(
      await screen.findByText("No postings match “Rust”."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Show all postings" }),
    ).toHaveAttribute("href", "/recruiter/job-postings");
  });

  it("shows an error with a retry", async () => {
    mocks.get
      .mockRejectedValueOnce(failure(500))
      .mockResolvedValueOnce(page([summary(0)]));
    mount();
    expect(await screen.findByRole("alert")).toHaveTextContent(
      /couldn't load your job postings/i,
    );
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByRole("table")).toBeInTheDocument();
    expect(mocks.get).toHaveBeenCalledTimes(2);
  });

  it("treats 403 as an access problem and re-checks the session", async () => {
    mocks.get.mockRejectedValue(failure(403));
    const client = mount();
    const invalidate = vi.spyOn(client, "invalidateQueries");
    expect(await screen.findByRole("alert")).toHaveTextContent(
      /don't have access to job postings/i,
    );
    await waitFor(() =>
      expect(invalidate).toHaveBeenCalledWith({ queryKey: ["auth", "me"] }),
    );
    expect(
      screen.queryByRole("button", { name: "Try again" }),
    ).not.toBeInTheDocument();
  });

  it("offers the first page when the page is past the end", async () => {
    mocks.get.mockResolvedValue(page([], 30));
    mount({ status: "all", search: "", page: 5 });
    expect(
      await screen.findByText("This page is past the end of the list."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Go to the first page" }),
    ).toHaveAttribute("href", "/recruiter/job-postings");
  });
});
