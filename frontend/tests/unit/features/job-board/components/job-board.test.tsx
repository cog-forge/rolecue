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
import { JobBoard } from "@/features/job-board/components/job-board";
import {
  boardSearch,
  defaultBoardFilters,
  type BoardFilters,
} from "@/features/job-board/utils/board-filters";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  search: new URLSearchParams(),
  user: {
    id: "11111111-1111-4111-8111-111111111111",
    full_name: "Cand",
    email: "cand@example.com",
    role: "candidate" as const,
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

const posting = (i: number, extra: Record<string, unknown> = {}) => ({
  id: `33333333-3333-4333-8333-${String(i).padStart(12, "0")}`,
  title: `Posting ${i + 1}`,
  company_name: "Acme",
  location: "Hanoi",
  employment_type: "Full-time",
  salary_min: 10_000_000,
  salary_max: 20_000_000,
  seniority_level: "senior",
  created_at: "2026-10-12T08:00:00Z",
  ...extra,
});
const page = (
  items: ReturnType<typeof posting>[],
  total = items.length,
  offset = 0,
) => ({
  data: {
    success: true,
    data: { items, pagination: { limit: 20, offset, total } },
  },
});
const postings = (from: number, count: number) =>
  Array.from({ length: count }, (_, i) => posting(from + i));
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
function mount(filters: BoardFilters = defaultBoardFilters) {
  mocks.search = new URLSearchParams(boardSearch(filters));
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  });
  client.setQueryData(["auth", "me"], mocks.user);
  const tree = () => (
    <QueryClientProvider client={client}>
      <JobBoard />
    </QueryClientProvider>
  );
  const view = render(tree());
  // Simulates the URL changing (pushState or browser Back) and the page reading it again.
  const navigate = (next: BoardFilters) => {
    mocks.search = new URLSearchParams(boardSearch(next));
    view.rerender(tree());
  };
  return Object.assign(client, { navigate });
}
const disabledLink = (name: string) => {
  const link = screen.getByRole("link", { name });
  expect(link).toHaveAttribute("aria-disabled", "true");
  return link;
};

beforeEach(() => {
  vi.clearAllMocks();
  mocks.get.mockReset();
});

describe("JobBoard", () => {
  it("loads, then lists open postings with their public details", async () => {
    mocks.get.mockResolvedValue(
      page([
        posting(0),
        posting(1, {
          company_name: null,
          location: null,
          employment_type: null,
          salary_min: null,
          salary_max: null,
          seniority_level: "intern",
        }),
      ]),
    );
    mount();
    expect(screen.getByRole("status")).toHaveTextContent(
      /loading open job postings/i,
    );
    const list = await screen.findByRole("list", {
      name: "Open job postings, 1–2 of 2",
    });
    const [first, second] = within(list)
      .getAllByRole("listitem")
      .filter((item) => item.parentElement === list);
    expect(
      within(first).getByRole("link", { name: "Posting 1" }),
    ).toHaveAttribute("href", "/jobs/33333333-3333-4333-8333-000000000000");
    expect(first).toHaveTextContent("Acme");
    expect(first).toHaveTextContent("Hanoi");
    expect(first).toHaveTextContent("Full-time");
    expect(first).toHaveTextContent("Senior");
    expect(first).toHaveTextContent("10,000,000 – 20,000,000");
    expect(second).toHaveTextContent("Intern");
    expect(second).not.toHaveTextContent("Acme");
    expect(mocks.get).toHaveBeenCalledWith("/job-board", {
      params: { limit: 20, offset: 0 },
      signal: expect.any(AbortSignal),
    });
    expect(screen.getByText("Showing 1–2 of 2")).toBeInTheDocument();
    disabledLink("Previous");
    // A disabled page link stays a link, so keyboard focus is not dropped, and does nothing.
    const push = vi.spyOn(window.history, "pushState");
    fireEvent.click(disabledLink("Next"));
    expect(push).not.toHaveBeenCalled();
    push.mockRestore();
    // The board has no apply action; that lives on the detail page.
    expect(
      screen.queryByRole("button", { name: /apply/i }),
    ).not.toBeInTheDocument();
  });

  it("sends filters to the API and keeps them in page and detail links", async () => {
    mocks.get.mockResolvedValue(page(postings(20, 20), 45, 20));
    mount({ search: "Go", location: "Hanoi", type: "Full-time", page: 2 });
    await screen.findByText("Showing 21–40 of 45");
    expect(mocks.get).toHaveBeenCalledWith("/job-board", {
      params: {
        limit: 20,
        offset: 20,
        search: "Go",
        location: "Hanoi",
        employment_type: "Full-time",
      },
      signal: expect.any(AbortSignal),
    });
    expect(screen.getByRole("link", { name: "Next" })).toHaveAttribute(
      "href",
      "/jobs?q=Go&location=Hanoi&type=Full-time&page=3",
    );
    expect(screen.getByRole("link", { name: "Previous" })).toHaveAttribute(
      "href",
      "/jobs?q=Go&location=Hanoi&type=Full-time",
    );
    const href = screen
      .getByRole("link", { name: "Posting 21" })
      .getAttribute("href")!;
    const url = new URL(href, "http://localhost");
    expect(url.pathname).toBe("/jobs/33333333-3333-4333-8333-000000000020");
    expect(url.searchParams.get("back")).toBe(
      "q=Go&location=Hanoi&type=Full-time&page=2",
    );
    expect(screen.getByLabelText("Job title")).toHaveValue("Go");
    expect(screen.getByLabelText("Location")).toHaveValue("Hanoi");
    expect(screen.getByLabelText("Employment type")).toHaveValue("Full-time");
  });

  it("keeps the counts true to the results on screen while the next page loads", async () => {
    mocks.get
      .mockResolvedValueOnce(page(postings(0, 20), 25))
      .mockReturnValueOnce(new Promise(() => {}));
    const board = mount();
    await screen.findByText("Showing 1–20 of 25");
    board.navigate({ ...defaultBoardFilters, page: 2 });
    expect(await screen.findByRole("status")).toHaveTextContent(
      "Loading job postings…",
    );
    expect(screen.getByText("Showing 1–20 of 25")).toBeInTheDocument();
    expect(
      screen.getByRole("list", { name: /Open job postings/ }),
    ).toHaveAttribute("aria-busy", "true");
    disabledLink("Next");
    disabledLink("Previous");
  });

  it("does not show an old empty result as the answer to a new search", async () => {
    mocks.get
      .mockResolvedValueOnce(page([]))
      .mockReturnValueOnce(new Promise(() => {}));
    const board = mount({ ...defaultBoardFilters, search: "Rust" });
    await screen.findByText("No open postings match your search.");
    board.navigate({ ...defaultBoardFilters, search: "Go" });
    expect(await screen.findByRole("status")).toHaveTextContent(
      /loading open job postings/i,
    );
    expect(
      screen.queryByText("No open postings match your search."),
    ).not.toBeInTheDocument();
  });

  it("follows the URL when the browser goes back to earlier filters", async () => {
    mocks.get
      .mockResolvedValueOnce(page([posting(0)]))
      .mockResolvedValueOnce(page([posting(1)]));
    const board = mount({ ...defaultBoardFilters, search: "Go" });
    expect(await screen.findByLabelText("Job title")).toHaveValue("Go");
    board.navigate(defaultBoardFilters);
    expect(
      await screen.findByRole("link", { name: "Posting 2" }),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Job title")).toHaveValue("");
    expect(mocks.get).toHaveBeenLastCalledWith("/job-board", {
      params: { limit: 20, offset: 0 },
      signal: expect.any(AbortSignal),
    });
  });

  it("submits trimmed filters to the URL and resets the page", async () => {
    mocks.get.mockResolvedValue(page([posting(0)], 45));
    mount({ ...defaultBoardFilters, page: 3 });
    fireEvent.change(await screen.findByLabelText("Job title"), {
      target: { value: "  Go dev " },
    });
    fireEvent.change(screen.getByLabelText("Location"), {
      target: { value: " Hanoi" },
    });
    fireEvent.change(screen.getByLabelText("Employment type"), {
      target: { value: "Full-time " },
    });
    const push = vi.spyOn(window.history, "pushState");
    fireEvent.click(screen.getByRole("button", { name: "Search" }));
    expect(push).toHaveBeenCalledWith(
      null,
      "",
      "/jobs?q=Go+dev&location=Hanoi&type=Full-time",
    );
    push.mockRestore();
  });

  it("blocks a filter longer than the API allows", async () => {
    mocks.get.mockResolvedValue(page([]));
    mount();
    fireEvent.change(await screen.findByLabelText("Location"), {
      target: { value: "x".repeat(201) },
    });
    expect(screen.getByRole("alert")).toHaveTextContent(
      /200 characters or fewer/,
    );
    expect(screen.getByLabelText("Location")).toHaveAttribute(
      "aria-invalid",
      "true",
    );
    expect(screen.getByRole("button", { name: "Search" })).toBeDisabled();
  });

  it("shows a first-time empty state without a clear action", async () => {
    mocks.get.mockResolvedValue(page([]));
    mount();
    expect(
      await screen.findByText("There are no open job postings yet."),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: /clear/i }),
    ).not.toBeInTheDocument();
  });

  it("shows a filtered empty state with a way to clear the search", async () => {
    mocks.get.mockResolvedValue(page([]));
    mount({ ...defaultBoardFilters, type: "Contract" });
    expect(
      await screen.findByText("No open postings match your search."),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Clear search" })).toHaveAttribute(
      "href",
      "/jobs",
    );
    expect(screen.getByRole("link", { name: "Clear" })).toHaveAttribute(
      "href",
      "/jobs",
    );
  });

  it("offers the first page when the page is past the end", async () => {
    mocks.get.mockResolvedValue(page([], 30, 80));
    mount({ ...defaultBoardFilters, page: 5 });
    expect(
      await screen.findByText("This page is past the end of the list."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Go to the first page" }),
    ).toHaveAttribute("href", "/jobs");
  });

  it("shows an error with a retry", async () => {
    mocks.get
      .mockRejectedValueOnce(failure(500))
      .mockResolvedValueOnce(page([posting(0)]));
    mount();
    expect(await screen.findByRole("alert")).toHaveTextContent(
      /couldn't load the job board/i,
    );
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(
      await screen.findByRole("link", { name: "Posting 1" }),
    ).toBeInTheDocument();
    expect(mocks.get).toHaveBeenCalledTimes(2);
  });

  it("surfaces a drifted contract as an error instead of rendering it", async () => {
    mocks.get.mockResolvedValue(
      page([posting(0, { seniority_level: "principal" })]),
    );
    mount();
    expect(await screen.findByRole("alert")).toHaveTextContent(
      /couldn't load the job board/i,
    );
  });

  it.each([401, 403])(
    "treats %s as an access problem and re-checks the session",
    async (status) => {
      mocks.get.mockRejectedValue(failure(status));
      const client = mount();
      const invalidate = vi.spyOn(client, "invalidateQueries");
      expect(await screen.findByRole("alert")).toHaveTextContent(
        /don't have access to the job board/i,
      );
      await waitFor(() =>
        expect(invalidate).toHaveBeenCalledWith({ queryKey: ["auth", "me"] }),
      );
      expect(
        screen.queryByRole("button", { name: "Try again" }),
      ).not.toBeInTheDocument();
    },
  );
});
