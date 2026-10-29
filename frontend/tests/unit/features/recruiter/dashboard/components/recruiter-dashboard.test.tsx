import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import { RecruiterDashboard } from "@/features/recruiter/dashboard/components/recruiter-dashboard";

// Deadlines show in the viewer's time zone; pin one so the text is the same on every machine.
vi.hoisted(() => {
  process.env.TZ = "Asia/Ho_Chi_Minh";
});

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
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

type Totals = {
  pending: number;
  drafts: number;
  rejected: number;
  unfunded: number;
  live: number;
  closed: number;
  // Eligible Applications as [hours from now, interview_eligible] pairs, and the API total.
  eligible: [number, boolean][];
  eligibleTotal?: number;
};
const zero: Totals = {
  pending: 0,
  drafts: 0,
  rejected: 0,
  unfunded: 0,
  live: 0,
  closed: 0,
  eligible: [],
};

const envelope = (items: unknown[], total: number) => ({
  data: {
    success: true,
    data: { items, pagination: { limit: 1, offset: 0, total } },
  },
});
const hours = (h: number) => new Date(Date.now() + h * 3_600_000).toISOString();
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

// Answers the way Go filters: by status, funded and review_submitted on postings, by status on
// applications. Overrides replace one request's answer.
function serve(
  totals: Partial<Totals> = {},
  override: (url: string, params: Record<string, unknown>) => unknown = () =>
    undefined,
) {
  const t = { ...zero, ...totals };
  mocks.get.mockImplementation(
    async (url: string, { params }: { params: Record<string, unknown> }) => {
      const custom = override(url, params);
      if (custom !== undefined) return custom;
      if (url === "/recruiter/applications") {
        if (params.status === "pending") return envelope([], t.pending);
        const items = t.eligible.map(([h, eligible]) => ({
          interview_eligible: eligible,
          interview_deadline: hours(h),
        }));
        return envelope(items, t.eligibleTotal ?? items.length);
      }
      if (params.status === "pending") return envelope([], t.drafts);
      if (params.status === "rejected") return envelope([], t.rejected);
      if (params.status === "intake_closed") return envelope([], t.closed);
      return envelope([], params.funded === "true" ? t.live : t.unfunded);
    },
  );
}

function mount() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  });
  client.setQueryData(["auth", "me"], mocks.user);
  render(
    <QueryClientProvider client={client}>
      <RecruiterDashboard />
    </QueryClientProvider>,
  );
  return client;
}
const rows = async () =>
  within(await screen.findByRole("list", { name: /need your attention/ }));

beforeEach(() => {
  vi.clearAllMocks();
  mocks.get.mockReset();
});

describe("RecruiterDashboard", () => {
  it("lists each kind of work with its count and a link to where it is done", async () => {
    serve({
      pending: 4,
      drafts: 2,
      rejected: 1,
      unfunded: 3,
      eligible: [[2, true]],
    });
    mount();
    const list = await rows();

    expect(await list.findByText("CVs awaiting review")).toBeInTheDocument();
    expect(list.getByRole("link", { name: "Review CVs" })).toHaveAttribute(
      "href",
      "/recruiter/applications?status=pending",
    );
    expect(list.getByRole("link", { name: "Continue drafts" })).toHaveAttribute(
      "href",
      "/recruiter/job-postings?status=preparing",
    );
    expect(list.getByRole("link", { name: "Fix postings" })).toHaveAttribute(
      "href",
      "/recruiter/job-postings?status=rejected",
    );
    expect(list.getByRole("link", { name: "Fund & publish" })).toHaveAttribute(
      "href",
      "/recruiter/job-postings?status=approved",
    );
    expect(list.getByRole("link", { name: "View candidates" })).toHaveAttribute(
      "href",
      "/recruiter/applications?status=interview_eligible",
    );
    const cvs = list.getByText("CVs awaiting review").closest("li");
    expect(within(cvs as HTMLElement).getByText("4")).toBeInTheDocument();
    expect(screen.queryByText("You're all caught up")).not.toBeInTheDocument();
  });

  it("asks the API for exactly the counts it shows", async () => {
    serve();
    mount();
    await screen.findByText("You're all caught up");
    const signal = expect.any(AbortSignal);
    expect(mocks.get).toHaveBeenCalledWith("/recruiter/applications", {
      params: { status: "pending", limit: 1, offset: 0 },
      signal,
    });
    expect(mocks.get).toHaveBeenCalledWith("/recruiter/applications", {
      params: { status: "interview_eligible", limit: 100, offset: 0 },
      signal,
    });
    expect(mocks.get).toHaveBeenCalledWith("/job-postings", {
      params: {
        status: "pending",
        review_submitted: "false",
        limit: 1,
        offset: 0,
      },
      signal,
    });
    expect(mocks.get).toHaveBeenCalledWith("/job-postings", {
      params: { status: "rejected", limit: 1, offset: 0 },
      signal,
    });
    expect(mocks.get).toHaveBeenCalledWith("/job-postings", {
      params: { status: "open", funded: "false", limit: 1, offset: 0 },
      signal,
    });
  });

  it("hides rows with nothing to do", async () => {
    serve({ pending: 1 });
    mount();
    const list = await rows();
    expect(await list.findByText("CVs awaiting review")).toBeInTheDocument();
    expect(list.queryByText("Drafts to submit")).not.toBeInTheDocument();
    expect(list.queryByText("Rejected postings to fix")).toBeNull();
    expect(list.queryByText("Approved, waiting for funding")).toBeNull();
    expect(
      list.queryByText("Interview deadlines in the next 24 hours"),
    ).toBeNull();
  });

  it("says everything is handled when every count is zero, with a way to start a posting", async () => {
    serve();
    mount();
    expect(await screen.findByText("You're all caught up")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Create a job posting" }),
    ).toHaveAttribute("href", "/recruiter/job-postings/new");
  });

  it("shows loading placeholders until the counts arrive", async () => {
    mocks.get.mockReturnValue(new Promise(() => {}));
    mount();
    expect(await screen.findAllByRole("status")).toHaveLength(5);
    expect(screen.getAllByText(/^Loading /)).toHaveLength(5);
  });

  it("counts only deadlines still ahead and inside 24 hours, and shows the earliest", async () => {
    serve({
      eligible: [
        [30, true],
        [5, true],
        [-2, true],
        [1, false],
        [12, true],
      ],
    });
    mount();
    const list = await rows();
    const row = (
      await list.findByText("Interview deadlines in the next 24 hours")
    ).closest("li") as HTMLElement;
    expect(within(row).getByText("2")).toBeInTheDocument();
    expect(row).toHaveTextContent(/Earliest: .*GMT\+7/);
    expect(row).not.toHaveTextContent("first 100");
  });

  it("does not pretend to be exact when more eligible candidates exist than were checked", async () => {
    serve({
      eligible: Array.from({ length: 100 }, (_, i) => [i % 2 ? 3 : 40, true]),
      eligibleTotal: 250,
    });
    mount();
    const list = await rows();
    const row = (
      await list.findByText("Interview deadlines in the next 24 hours")
    ).closest("li") as HTMLElement;
    expect(within(row).getByText("50+")).toBeInTheDocument();
    expect(row).toHaveTextContent("Only the first 100 cleared candidates");
  });

  it("keeps the deadlines row, as a lower bound, when none of the checked ones are due", async () => {
    serve({ eligible: [[48, true]], eligibleTotal: 400 });
    mount();
    const list = await rows();
    const row = (
      await list.findByText("Interview deadlines in the next 24 hours")
    ).closest("li") as HTMLElement;
    expect(within(row).getByText("0+")).toBeInTheDocument();
    expect(screen.queryByText("You're all caught up")).not.toBeInTheDocument();
  });

  it("shows one row's failure with a retry while the other rows still load", async () => {
    let failing = true;
    serve({ pending: 3, drafts: 1 }, (url, params) => {
      if (url === "/job-postings" && params.status === "rejected" && failing)
        return Promise.reject(failure(500));
    });
    mount();
    const list = await rows();
    expect(await list.findByText("CVs awaiting review")).toBeInTheDocument();
    expect(list.getByText("Drafts to submit")).toBeInTheDocument();
    expect(
      await list.findByText("We couldn't load this count."),
    ).toBeInTheDocument();
    // A failed count never counts as "nothing to do".
    expect(screen.queryByText("You're all caught up")).toBeNull();

    failing = false;
    serve({ pending: 3, drafts: 1, rejected: 2 });
    list.getByRole("button", { name: "Try again" }).click();
    expect(
      await list.findByText("Rejected postings to fix"),
    ).toBeInTheDocument();
    expect(list.queryByText("We couldn't load this count.")).toBeNull();
  });

  it("does not claim all clear when a count failed even if the others are zero", async () => {
    serve({}, (url, params) =>
      url === "/recruiter/applications" && params.status === "pending"
        ? Promise.reject(failure(500))
        : undefined,
    );
    mount();
    expect(
      await screen.findByText("We couldn't load this count."),
    ).toBeInTheDocument();
    expect(screen.queryByText("You're all caught up")).toBeNull();
  });

  it("shows the live and intake-closed postings as a quiet summary", async () => {
    serve({ pending: 1, live: 6, closed: 2 });
    mount();
    expect(
      await screen.findByRole("heading", { name: "Your postings at a glance" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /^Live\s*6$/ })).toHaveAttribute(
      "href",
      "/recruiter/job-postings?status=open",
    );
    expect(
      screen.getByRole("link", { name: /^Intake closed\s*2$/ }),
    ).toHaveAttribute("href", "/recruiter/job-postings?status=intake_closed");
  });

  it("re-checks the session when access is denied", async () => {
    mocks.get.mockRejectedValue(failure(403));
    const client = mount();
    expect(
      await screen.findByText("You don't have access to the dashboard"),
    ).toBeInTheDocument();
    expect(client.getQueryState(["auth", "me"])?.isInvalidated).toBe(true);
  });
});
