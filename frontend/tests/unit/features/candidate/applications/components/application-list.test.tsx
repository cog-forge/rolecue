import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import { CandidateApplicationList } from "@/features/candidate/applications/components/application-list";

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

const item = (i: number, over: Record<string, unknown> = {}) => ({
  id: `44444444-4444-4444-8444-${String(i).padStart(12, "0")}`,
  job_posting_id: "33333333-3333-4333-8333-333333333333",
  job_posting_title: `Backend Engineer ${i}`,
  status: "pending",
  reject_reason: null,
  interview_deadline: null,
  can_start_interview: false,
  created_at: "2026-10-20T08:00:00Z",
  ...over,
});
const envelope = (items: unknown[], total = items.length, page = 1) => ({
  data: {
    success: true,
    data: { items, total, page, page_size: 20 },
  },
});
function failure(status: number, code = "INTERNAL_ERROR") {
  return new AxiosError("failure", "ERR_BAD_RESPONSE", undefined, undefined, {
    status,
    statusText: "Error",
    data: { success: false, error: { code, message: "server says" } },
    headers: new AxiosHeaders(),
    config: { headers: new AxiosHeaders() },
  });
}

function renderList() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  });
  return render(
    <QueryClientProvider client={client}>
      <CandidateApplicationList />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.get.mockReset();
  mocks.search = new URLSearchParams();
});

describe("CandidateApplicationList", () => {
  it("shows a loading state, then each application with its status", async () => {
    mocks.get.mockResolvedValue(
      envelope([
        item(1),
        item(2, { status: "rejected", reject_reason: "Missing Go" }),
        item(3, { status: "approved" }),
      ]),
    );
    renderList();
    expect(screen.getByRole("status")).toHaveTextContent(
      /loading your applications/i,
    );

    const list = await screen.findByRole("list");
    const rows = within(list).getAllByRole("listitem");
    expect(rows).toHaveLength(3);
    expect(rows[0]).toHaveTextContent("Awaiting CV review");
    expect(rows[1]).toHaveTextContent("Not selected");
    expect(rows[2]).toHaveTextContent("Approved");
    expect(
      screen.getByRole("link", { name: "Backend Engineer 1" }),
    ).toHaveAttribute("href", `/applications/${item(1).id}`);
    expect(mocks.get).toHaveBeenCalledWith("/applications", {
      params: { page: 1, page_size: 20 },
      signal: expect.any(AbortSignal),
    });
  });

  it("pages through the list from the URL", async () => {
    mocks.search = new URLSearchParams("page=2");
    mocks.get.mockResolvedValue(
      envelope(
        Array.from({ length: 20 }, (_, i) => item(i + 21)),
        45,
        2,
      ),
    );
    renderList();
    expect(await screen.findByText("Showing 21–40 of 45")).toBeInTheDocument();
    expect(mocks.get).toHaveBeenCalledWith("/applications", {
      params: { page: 2, page_size: 20 },
      signal: expect.any(AbortSignal),
    });
    expect(screen.getByRole("link", { name: "Previous" })).toHaveAttribute(
      "href",
      "/applications?page=1",
    );
    expect(screen.getByRole("link", { name: "Next" })).toHaveAttribute(
      "href",
      "/applications?page=3",
    );
  });

  it("disables Next on the last page", async () => {
    mocks.get.mockResolvedValue(envelope([item(1)], 1));
    renderList();
    await screen.findByRole("list");
    expect(screen.getByRole("button", { name: "Next" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Previous" })).toBeDisabled();
  });

  it("shows the empty state when the Candidate has not applied", async () => {
    mocks.get.mockResolvedValue(envelope([], 0));
    renderList();
    expect(
      await screen.findByRole("heading", {
        name: "You haven't applied to any postings yet.",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Browse the Job Board" }),
    ).toHaveAttribute("href", "/jobs");
  });

  it("offers a retry when the list cannot load", async () => {
    mocks.get
      .mockRejectedValueOnce(failure(500))
      .mockResolvedValueOnce(envelope([item(1)]));
    renderList();
    expect(
      await screen.findByRole("heading", {
        name: "We couldn't load your applications",
      }),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByRole("list")).toBeInTheDocument();
  });

  it("explains a refused request without listing anything", async () => {
    mocks.get.mockRejectedValue(failure(403, "FORBIDDEN"));
    renderList();
    expect(
      await screen.findByRole("heading", {
        name: "You don't have access to applications",
      }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("list")).not.toBeInTheDocument();
  });
});
