import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, within } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import { DashboardOverview } from "@/features/dashboard/components/dashboard-overview";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
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

const PROFILE_ID = "22222222-2222-4222-8222-222222222222";
const INTERVIEW_ID = "33333333-3333-4333-8333-333333333333";
const APPLICATION_ID = "44444444-4444-4444-8444-444444444444";

const roleProfiles = {
  success: true,
  data: {
    items: [
      {
        id: PROFILE_ID,
        title: "Go Backend Engineer",
        seniorityLevel: "mid",
        status: "confirmed",
        createdAt: "2026-10-20T08:00:00Z",
      },
    ],
    pagination: { limit: 3, offset: 0, total: 1 },
  },
};

const practice = {
  success: true,
  data: {
    items: [
      {
        id: INTERVIEW_ID,
        type: "practice",
        started_at: "2026-10-21T09:00:00Z",
        completed_at: "2026-10-21T09:30:00Z",
        evaluation_status: "completed",
        status: "completed",
        score: 78,
        role_profile_title: "Go Backend Engineer",
        job_posting_title: null,
        application_id: null,
      },
    ],
    total: 1,
    page: 1,
    page_size: 3,
  },
};

const applications = {
  success: true,
  data: {
    items: [
      {
        id: APPLICATION_ID,
        job_posting_id: "55555555-5555-4555-8555-555555555555",
        job_posting_title: "Platform Engineer",
        status: "interview_eligible",
        reject_reason: null,
        interview_deadline: null,
        can_start_interview: true,
        created_at: "2026-10-19T08:00:00Z",
      },
    ],
    total: 1,
    page: 1,
    page_size: 20,
  },
};

const wallet = {
  success: true,
  data: { id: "w1", user_id: mocks.user.id, balance: 1250 },
};

// Routes each GET to its fixture so one card can fail while the others load.
function serve(overrides: Record<string, unknown> = {}) {
  const responses: Record<string, unknown> = {
    "/role-profiles": roleProfiles,
    "/interviews": practice,
    "/applications": applications,
    "/wallet": wallet,
    [`/interviews/${INTERVIEW_ID}/report`]: {
      success: true,
      data: {
        interview_id: INTERVIEW_ID,
        type: "practice",
        status: "completed",
        overall_score: 78,
        started_at: "2026-10-21T09:00:00Z",
        completed_at: "2026-10-21T09:30:00Z",
        summary: null,
        breakdown: [
          {
            metric_id: "66666666-6666-4666-8666-666666666666",
            name: "Technical Accuracy",
            score: 62,
            weight: 60,
          },
          {
            metric_id: "77777777-7777-4777-8777-777777777777",
            name: "Communication",
            score: 91,
            weight: 40,
          },
        ],
        questions: [],
      },
    },
    ...overrides,
  };
  mocks.get.mockImplementation((path: string) => {
    const body = responses[path];
    if (body instanceof Error) return Promise.reject(body);
    return Promise.resolve({ data: body });
  });
}

function failure(status: number) {
  return new AxiosError("failure", "ERR_BAD_RESPONSE", undefined, undefined, {
    status,
    statusText: "Error",
    data: { success: false, error: { code: "INTERNAL_ERROR", message: "x" } },
    headers: new AxiosHeaders(),
    config: { headers: new AxiosHeaders() },
  });
}

function renderOverview(locale: "en" | "vi" = "en") {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  });
  return renderWithIntl(
    <QueryClientProvider client={client}>
      <DashboardOverview />
    </QueryClientProvider>,
    { locale },
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.get.mockReset();
});

describe("DashboardOverview", () => {
  it("shows Vietnamese labels while preserving stored titles and scores", async () => {
    serve();
    renderOverview("vi");
    expect(
      screen.getByRole("heading", { name: "Tổng quan" }),
    ).toBeInTheDocument();
    expect(await screen.findByText("1 JD mục tiêu")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Platform Engineer" }),
    ).toBeInTheDocument();
    expect(
      within(
        screen.getByRole("region", { name: "Hoạt động luyện tập" }),
      ).getByText("78,00/100"),
    ).toBeInTheDocument();
    expect(screen.getByText("1.250")).toBeInTheDocument();
  });
  it("shows a loading state per card, then each card's data", async () => {
    serve();
    renderOverview();

    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(
      "Dashboard",
    );
    expect(screen.getAllByRole("status")).toHaveLength(7);

    expect(
      await screen.findByRole("heading", { name: "Target JDs" }),
    ).toBeInTheDocument();
    expect(await screen.findByText("1 Target JD")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Manage Target JDs" }),
    ).toHaveAttribute("href", "/target-jds");

    const practiceSection = screen
      .getByRole("heading", { name: "Practice activity" })
      .closest("section");
    expect(practiceSection).not.toBeNull();
    const practiceRow = within(practiceSection as HTMLElement).getByRole(
      "link",
      { name: /Go Backend Engineer/ },
    );
    expect(practiceRow).toHaveAttribute("href", `/reports/${INTERVIEW_ID}`);
    expect(
      within(practiceSection as HTMLElement).getByText("78.00/100"),
    ).toBeInTheDocument();

    expect(
      screen.getByRole("link", { name: "Platform Engineer" }),
    ).toHaveAttribute("href", `/applications/${APPLICATION_ID}`);
    expect(
      screen.getByText(/start the interview for/i, { exact: false }),
    ).toHaveTextContent("Platform Engineer");

    expect(await screen.findByText("1,250")).toBeInTheDocument();
    expect(mocks.get).toHaveBeenCalledWith("/interviews", {
      params: { type: "practice", page: 1, page_size: 20 },
      signal: expect.any(AbortSignal),
    });
    expect(mocks.get).toHaveBeenCalledWith("/role-profiles", {
      params: { limit: 3, offset: 0 },
      signal: expect.any(AbortSignal),
    });
  });

  it("keeps the other cards working when one request fails", async () => {
    serve({ "/wallet": failure(500) });
    renderOverview();

    const walletSection = (
      await screen.findByRole("heading", { name: "Coin balance" })
    ).closest("section") as HTMLElement;
    expect(
      await within(walletSection).findByText(
        "We couldn't load your coin balance.",
      ),
    ).toBeInTheDocument();

    expect(await screen.findByText("1 Target JD")).toBeInTheDocument();
    expect(screen.getByText("1 application")).toBeInTheDocument();
  });

  it("retries a failed card on demand", async () => {
    serve({ "/applications": failure(500) });
    renderOverview();

    const section = (
      await screen.findByRole("heading", { name: "Applications" })
    ).closest("section") as HTMLElement;
    expect(
      await within(section).findByText("We couldn't load your applications."),
    ).toBeInTheDocument();

    serve();
    fireEvent.click(within(section).getByRole("button", { name: "Try again" }));

    expect(
      await within(section).findByText("1 application"),
    ).toBeInTheDocument();
  });

  it("offers a first step when the candidate has no Target JDs or practice yet", async () => {
    serve({
      "/role-profiles": {
        success: true,
        data: {
          items: [],
          pagination: { limit: 3, offset: 0, total: 0 },
        },
      },
      "/interviews": {
        success: true,
        data: { items: [], total: 0, page: 1, page_size: 3 },
      },
      "/applications": {
        success: true,
        data: { items: [], total: 0, page: 1, page_size: 20 },
      },
    });
    renderOverview();

    expect(
      await screen.findByRole("link", { name: "Add a Target JD" }),
    ).toHaveAttribute("href", "/target-jds");
    expect(
      screen.getByRole("link", { name: "Start a practice interview" }),
    ).toHaveAttribute("href", "/interviews/new/job-description");
    expect(
      screen.getByRole("link", { name: "Browse the Job Board" }),
    ).toHaveAttribute("href", "/jobs");
  });

  it("names the next step for a Target JD that still needs review", async () => {
    serve({
      "/role-profiles": {
        success: true,
        data: {
          items: [
            {
              id: PROFILE_ID,
              title: "Frontend Engineer",
              seniorityLevel: null,
              status: "review",
              createdAt: "2026-10-20T08:00:00Z",
            },
          ],
          pagination: { limit: 3, offset: 0, total: 1 },
        },
      },
    });
    renderOverview();

    expect(
      await screen.findByText(/next step: waiting for your review/i),
    ).toHaveTextContent("Frontend Engineer");
  });
  it("loads one bounded practice window and reuses it across the dashboard", async () => {
    serve();
    renderOverview();
    await screen.findByText("Technical Accuracy");
    expect(
      mocks.get.mock.calls.filter(([path]) => path === "/interviews"),
    ).toHaveLength(1);
    expect(mocks.get).toHaveBeenCalledWith(
      `/interviews/${INTERVIEW_ID}/report`,
      { signal: expect.any(AbortSignal) },
    );
    expect(
      screen.getByRole("progressbar", { name: "Technical Accuracy" }),
    ).toHaveAttribute("aria-valuenow", "62");
    expect(screen.getByRole("link", { name: "Read feedback" })).toHaveAttribute(
      "href",
      `/reports/${INTERVIEW_ID}`,
    );
  });

  it("uses server interview eligibility rather than deriving it from the status", async () => {
    serve({
      "/applications": {
        ...applications,
        data: {
          ...applications.data,
          items: [
            { ...applications.data.items[0], can_start_interview: false },
          ],
        },
      },
    });
    renderOverview();
    const next = screen.getByRole("region", { name: "Your next step" });
    await within(next).findByText("Keep practising");
    expect(
      within(next).queryByText("Interview available"),
    ).not.toBeInTheDocument();
    expect(within(next).getByRole("link")).toHaveAttribute(
      "href",
      "/interviews/new/job-description",
    );
  });

  it("does not turn partial, withheld or pending scores into complete progress", async () => {
    serve({
      "/interviews": {
        success: true,
        data: {
          ...practice.data,
          items: [
            { ...practice.data.items[0], score: 88, score_outcome: "partial" },
            {
              ...practice.data.items[0],
              id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
              score: null,
              score_outcome: "withheld",
            },
          ],
        },
      },
    });
    renderOverview();
    const progress = screen.getByRole("region", { name: "Practice progress" });
    await within(progress).findByText("No complete scores yet");
    expect(within(progress).queryByText("88.00")).not.toBeInTheDocument();
    expect(
      within(
        screen.getByRole("region", { name: "Practice activity" }),
      ).getByText("Partial score"),
    ).toBeInTheDocument();
  });
  it("distinguishes unassessed feedback from a candidate who has never practised", async () => {
    serve({
      [`/interviews/${INTERVIEW_ID}/report`]: {
        success: true,
        data: {
          interview_id: INTERVIEW_ID,
          type: "practice",
          status: "completed",
          overall_score: null,
          started_at: "2026-10-21T09:00:00Z",
          completed_at: "2026-10-21T09:30:00Z",
          summary: null,
          breakdown: [
            {
              metric_id: "66666666-6666-4666-8666-666666666666",
              name: "Technical Accuracy",
              score: null,
              weight: 60,
            },
          ],
          questions: [],
        },
      },
    });
    renderOverview();
    const focus = screen.getByRole("region", { name: "What to work on" });
    expect(
      await within(focus).findByText("No assessed metrics in this report"),
    ).toBeInTheDocument();
    expect(within(focus).queryByRole("progressbar")).not.toBeInTheDocument();
    expect(
      within(focus).queryByText("Feedback after your first practice"),
    ).not.toBeInTheDocument();
  });
});
