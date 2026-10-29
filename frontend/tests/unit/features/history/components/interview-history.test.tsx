import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, within } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import { InterviewHistory } from "@/features/history/components/interview-history";

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

const APPLICATION = "66666666-6666-4666-8666-666666666666";

const practice = (i: number, over: Record<string, unknown> = {}) => ({
  id: `44444444-4444-4444-8444-${String(i).padStart(12, "0")}`,
  type: "practice",
  started_at: "2026-10-20T08:00:00Z",
  completed_at: "2026-10-20T08:30:00Z",
  evaluation_status: "completed",
  status: "completed",
  score: 72,
  role_profile_title: `Practice role ${i}`,
  job_posting_title: null,
  application_id: null,
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

function renderHistory() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  });
  return renderWithIntl(
    <QueryClientProvider client={client}>
      <InterviewHistory />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.get.mockReset();
  mocks.search = new URLSearchParams();
});

describe("InterviewHistory", () => {
  it("shows a loading state, then each interview with its status and score", async () => {
    mocks.get.mockResolvedValue(
      envelope([
        practice(1),
        practice(2, {
          status: "in_progress",
          evaluation_status: "pending",
          completed_at: null,
          score: null,
        }),
      ]),
    );
    renderHistory();
    expect(screen.getByRole("status")).toHaveTextContent(
      /loading your interview history/i,
    );

    const list = await screen.findByRole("list", { name: "Your interviews" });
    const rows = within(list).getAllByRole("listitem");
    expect(rows).toHaveLength(2);
    expect(rows[0]).toHaveTextContent("Completed");
    expect(rows[0]).toHaveTextContent("72 / 100");
    expect(rows[1]).toHaveTextContent("In progress");
    expect(rows[1]).toHaveTextContent("No score yet");
    expect(
      screen.getByRole("link", { name: "Practice role 1" }),
    ).toHaveAttribute("href", `/reports/${practice(1).id}`);

    expect(mocks.get).toHaveBeenCalledWith("/interviews", {
      params: { page: 1, page_size: 20 },
      signal: expect.any(AbortSignal),
    });
  });

  it("sends the type filter from the URL and marks the active tab", async () => {
    mocks.search = new URLSearchParams("type=recruitment");
    mocks.get.mockResolvedValue(envelope([]));
    renderHistory();
    expect(
      await screen.findByRole("heading", {
        name: "You have no recruitment interviews yet.",
      }),
    ).toBeInTheDocument();
    expect(mocks.get).toHaveBeenCalledWith("/interviews", {
      params: { type: "recruitment", page: 1, page_size: 20 },
      signal: expect.any(AbortSignal),
    });
    expect(screen.getByRole("link", { name: "Recruitment" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(screen.getByRole("link", { name: "Practice" })).not.toHaveAttribute(
      "aria-current",
    );
  });

  it("links a recruitment row to its Application and shows only the score", async () => {
    mocks.get.mockResolvedValue(
      envelope([
        practice(1, {
          type: "recruitment",
          role_profile_title: "Role profile",
          job_posting_title: "Backend Engineer, Hanoi",
          application_id: APPLICATION,
        }),
      ]),
    );
    renderHistory();
    expect(
      await screen.findByRole("link", { name: "Backend Engineer, Hanoi" }),
    ).toHaveAttribute("href", `/reports/${practice(1).id}`);
    expect(
      screen.getByRole("link", { name: "View application" }),
    ).toHaveAttribute("href", `/applications/${APPLICATION}`);
    expect(screen.getByText("Recruitment interview")).toBeInTheDocument();
  });

  it("pages through the history with links that keep the type filter", async () => {
    mocks.search = new URLSearchParams("type=practice&page=2");
    mocks.get.mockResolvedValue(
      envelope(
        Array.from({ length: 20 }, (_, i) => practice(i + 21)),
        45,
        2,
      ),
    );
    renderHistory();
    expect(await screen.findByText("Showing 21–40 of 45")).toBeInTheDocument();
    expect(mocks.get).toHaveBeenCalledWith("/interviews", {
      params: { type: "practice", page: 2, page_size: 20 },
      signal: expect.any(AbortSignal),
    });
    expect(screen.getByRole("link", { name: "Previous" })).toHaveAttribute(
      "href",
      "/history?type=practice",
    );
    expect(screen.getByRole("link", { name: "Next" })).toHaveAttribute(
      "href",
      "/history?type=practice&page=3",
    );
  });

  it("disables Next on the last page", async () => {
    mocks.get.mockResolvedValue(envelope([practice(1)], 1));
    renderHistory();
    await screen.findByText("Showing 1–1 of 1");
    expect(screen.getByRole("button", { name: "Next" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Previous" })).toBeDisabled();
  });

  it("shows the empty state for a new Candidate", async () => {
    mocks.get.mockResolvedValue(envelope([], 0));
    renderHistory();
    expect(
      await screen.findByRole("heading", {
        name: "You have no interviews yet.",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Go to the dashboard" }),
    ).toHaveAttribute("href", "/dashboard");
  });

  it("offers a retry when the history cannot load", async () => {
    mocks.get
      .mockRejectedValueOnce(failure(500))
      .mockResolvedValueOnce(envelope([practice(1)]));
    renderHistory();
    expect(
      await screen.findByRole("heading", {
        name: "We couldn't load your interview history",
      }),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(
      await screen.findByRole("link", { name: "Practice role 1" }),
    ).toBeInTheDocument();
  });

  it("explains a refused request without listing anything", async () => {
    mocks.get.mockRejectedValue(failure(403, "FORBIDDEN"));
    renderHistory();
    expect(
      await screen.findByRole("heading", {
        name: "You don't have access to interview history",
      }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Practice role/ })).toBeNull();
  });
});
