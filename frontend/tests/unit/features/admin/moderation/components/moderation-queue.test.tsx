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
import { ModerationQueue } from "@/features/admin/moderation/components/moderation-queue";
import { defaultQueueFilters } from "@/features/admin/moderation/utils/queue-filters";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  push: vi.fn(),
  user: {
    id: "11111111-1111-4111-8111-111111111111",
    full_name: "Admin",
    email: "admin@example.com",
    role: "admin" as const,
    email_verified: true,
    is_locked: false,
    image: null,
  },
}));
vi.mock("@/lib/api/client", () => ({ apiClient: { get: mocks.get } }));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));
vi.mock("next/navigation", () => ({ useRouter: () => ({ push: mocks.push }) }));

const recruiterId = "22222222-2222-4222-8222-222222222222";
const page = (count: number, total = count) => ({
  data: {
    success: true,
    data: {
      items: Array.from({ length: count }, (_, i) => ({
        id: `33333333-3333-4333-8333-${String(i).padStart(12, "0")}`,
        title: `Posting ${i + 1}`,
        status: "pending",
        recruiter: {
          id: recruiterId,
          name: "Rec Ruiter",
          email: "rec@example.com",
        },
        created_at: "2026-10-12T08:00:00Z",
      })),
      pagination: { limit: 20, offset: 0, total },
    },
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
function mount(filters = defaultQueueFilters) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  });
  client.setQueryData(["auth", "me"], mocks.user);
  render(
    <QueryClientProvider client={client}>
      <ModerationQueue filters={filters} />
    </QueryClientProvider>,
  );
  return client;
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.get.mockReset();
});

describe("ModerationQueue", () => {
  it("loads the pending queue, then lists postings with status text", async () => {
    mocks.get.mockResolvedValue(page(2, 2));
    mount();
    expect(screen.getByRole("status")).toHaveTextContent(
      /loading job postings/i,
    );
    const table = await screen.findByRole("table");
    expect(within(table).getByText("Posting 1")).toBeInTheDocument();
    expect(within(table).getAllByText("Pending review")).toHaveLength(2);
    expect(mocks.get).toHaveBeenCalledWith("/admin/job-postings", {
      params: { status: "pending", limit: 20, offset: 0 },
      signal: expect.any(AbortSignal),
    });
    expect(screen.getByRole("link", { name: "Pending" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(screen.getByText("Showing 1–2 of 2")).toBeInTheDocument();
  });

  it("links each row to its review page and keeps the filters for the way back", async () => {
    mocks.get.mockResolvedValue(page(1));
    mount({ ...defaultQueueFilters, status: "all", page: 1 });
    const review = await screen.findByRole("link", {
      name: /review posting 1/i,
    });
    expect(review.getAttribute("href")).toBe(
      "/admin/job-postings/33333333-3333-4333-8333-000000000000?back=status%3Dall",
    );
  });

  it("filters by recruiter from the recruiter name", async () => {
    mocks.get.mockResolvedValue(page(1));
    mount();
    const recruiter = await screen.findByRole("link", { name: "Rec Ruiter" });
    expect(recruiter).toHaveAttribute(
      "href",
      `/admin/job-postings?recruiter_id=${recruiterId}`,
    );
  });

  it("shows a friendly empty state", async () => {
    mocks.get.mockResolvedValue(page(0));
    mount();
    expect(
      await screen.findByText("No postings are waiting for review."),
    ).toBeInTheDocument();
  });

  it("shows an error with a working retry", async () => {
    mocks.get.mockRejectedValueOnce(failure(500)).mockResolvedValue(page(1));
    mount();
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(/couldn't load the job postings/i);
    fireEvent.click(within(alert).getByRole("button", { name: "Try again" }));
    expect(await screen.findByText("Posting 1")).toBeInTheDocument();
    expect(mocks.get).toHaveBeenCalledTimes(2);
  });

  it("treats 403 as an access problem and re-checks the session", async () => {
    mocks.get.mockRejectedValue(failure(403));
    const client = mount();
    const spy = vi.spyOn(client, "invalidateQueries");
    expect(await screen.findByRole("alert")).toHaveTextContent(
      /don't have access to moderation/i,
    );
    await waitFor(() =>
      expect(spy).toHaveBeenCalledWith({ queryKey: ["auth", "me"] }),
    );
  });

  it("blocks a reversed date range and pushes valid dates to the URL", async () => {
    mocks.get.mockResolvedValue(page(1));
    mount();
    await screen.findByRole("table");
    fireEvent.change(screen.getByLabelText("Submitted from"), {
      target: { value: "2026-10-10" },
    });
    fireEvent.change(screen.getByLabelText("to"), {
      target: { value: "2026-10-01" },
    });
    expect(screen.getByRole("alert")).toHaveTextContent(
      /start date must be on or before/i,
    );
    expect(screen.getByRole("button", { name: "Apply" })).toBeDisabled();
    fireEvent.change(screen.getByLabelText("to"), {
      target: { value: "2026-10-20" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Apply" }));
    expect(mocks.push).toHaveBeenCalledWith(
      "/admin/job-postings?from=2026-10-10&to=2026-10-20",
      { scroll: false },
    );
  });
});
