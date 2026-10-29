import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import { renderWithIntl } from "@tests/helpers/intl";
import { AdminOverview } from "@/features/admin/overview/components/admin-overview";
import { routes } from "@/config/routes";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
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
// The window lives in the real URL; the mock reads it on every render.
vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(window.location.search),
  usePathname: () => routes.admin.dashboard,
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
}));

const overview = {
  generated_at: "2026-10-30T08:00:00+07:00",
  window: {
    days: 30,
    from: "2026-10-01T00:00:00+07:00",
    to: "2026-10-30T08:00:00+07:00",
  },
  accounts: { candidates: 1234, recruiters: 56, locked: 7, new_in_window: 89 },
  job_postings: { pending: 3, open: 4, intake_closed: 5, closed: 6, rejected: 2 },
  applications: {
    pending: 11,
    interview_eligible: 12,
    interviewed: 13,
    approved: 14,
    rejected: 15,
    submitted_in_window: 16,
  },
  interviews: {
    practice_started_in_window: 21,
    recruitment_started_in_window: 22,
    completed_in_window: 23,
    evaluation_failed: 24,
  },
  finance: {
    revenue_vnd: 100000,
    successful_deposits: 31,
    coins_sold: 32000,
    coins_spent: 33,
    coins_refunded: 34,
    unique_payers: 35,
    success_rate: 66.7,
  },
};

const zeroOverview = {
  ...overview,
  accounts: { candidates: 0, recruiters: 0, locked: 0, new_in_window: 0 },
  job_postings: { pending: 0, open: 0, intake_closed: 0, closed: 0, rejected: 0 },
  applications: {
    pending: 0,
    interview_eligible: 0,
    interviewed: 0,
    approved: 0,
    rejected: 0,
    submitted_in_window: 0,
  },
  interviews: {
    practice_started_in_window: 0,
    recruitment_started_in_window: 0,
    completed_in_window: 0,
    evaluation_failed: 0,
  },
  finance: {
    revenue_vnd: 0,
    successful_deposits: 0,
    coins_sold: 0,
    coins_spent: 0,
    coins_refunded: 0,
    unique_payers: 0,
    success_rate: 0,
  },
};

const ok = (data: unknown) => Promise.resolve({ data: { success: true, data } });
const httpError = (status: number) =>
  new AxiosError("failed", "ERR", undefined, undefined, {
    status,
    statusText: "",
    headers: {},
    config: { headers: new AxiosHeaders() },
    data: {},
  });

function renderPage() {
  const cache = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const invalidate = vi.spyOn(cache, "invalidateQueries");
  renderWithIntl(
    <QueryClientProvider client={cache}>
      <AdminOverview />
    </QueryClientProvider>,
  );
  return { invalidate };
}

function sectionOf(title: string) {
  return screen.getByRole("region", { name: title });
}

beforeEach(() => {
  mocks.get.mockReset();
  window.history.replaceState(null, "", routes.admin.dashboard);
});

describe("AdminOverview", () => {
  it("renders every section with its numbers and definition text", async () => {
    mocks.get.mockImplementation(() => ok(overview));
    renderPage();

    expect(
      await screen.findByRole("heading", { level: 1, name: "Admin dashboard" }),
    ).toBeInTheDocument();
    const accounts = sectionOf("Accounts");
    expect(within(accounts).getByText("1,234")).toBeInTheDocument();
    expect(
      within(accounts).getByText(/Candidates and recruiters only/),
    ).toBeInTheDocument();
    expect(sectionOf("Job postings")).toHaveTextContent("All-time counts by status.");
    expect(within(sectionOf("Job postings")).getByText("Rejected")).toBeInTheDocument();
    expect(sectionOf("Applications")).toHaveTextContent("Submitted counts the window.");
    expect(within(sectionOf("Applications")).getByText("16")).toBeInTheDocument();
    expect(sectionOf("Interviews")).toHaveTextContent("Evaluation failed is all-time.");
    expect(within(sectionOf("Interviews")).getByText("24")).toBeInTheDocument();
    const finance = sectionOf("Finance");
    expect(finance).toHaveTextContent("Coin debits and refunds are not revenue.");
    expect(within(finance).getByText("100.000 ₫")).toBeInTheDocument();
    expect(within(finance).getByText("32,000")).toBeInTheDocument();
  });

  it("shows zero values as 0 instead of hiding them", async () => {
    mocks.get.mockImplementation(() => ok(zeroOverview));
    renderPage();

    const finance = sectionOf("Finance");
    expect(
      await within(finance).findByText("0 ₫"),
    ).toBeInTheDocument();
    expect(within(finance).getByText("Coins refunded").nextSibling).toHaveTextContent("0");
    expect(within(sectionOf("Accounts")).getByText("Locked now").nextSibling).toHaveTextContent("0");
  });

  it("requests the window from the URL and shows the default when absent", async () => {
    mocks.get.mockImplementation(() => ok(overview));
    renderPage();

    await screen.findByRole("region", { name: "Accounts" });
    expect(mocks.get).toHaveBeenCalledWith("/admin/overview", {
      params: { days: 30 },
      signal: expect.anything(),
    });
  });

  it("switches the window through the selector and requests the new days", async () => {
    mocks.get.mockImplementation(() => ok(overview));
    const { rerender } = renderPage();
    await screen.findByRole("region", { name: "Accounts" });

    const selector = screen.getByRole("navigation", { name: "Reporting window" });
    const last7 = within(selector).getByRole("link", { name: "Last 7 days" });
    expect(last7).toHaveAttribute("href", `${routes.admin.dashboard}?days=7`);
    expect(within(selector).getByRole("link", { name: "Last 30 days" })).toHaveAttribute(
      "aria-current",
      "page",
    );

    fireEvent.click(last7);
    expect(window.location.search).toBe("?days=7");
    rerender(
      <QueryClientProvider client={new QueryClient()}>
        <AdminOverview />
      </QueryClientProvider>,
    );

    await waitFor(() =>
      expect(mocks.get).toHaveBeenLastCalledWith("/admin/overview", {
        params: { days: 7 },
        signal: expect.anything(),
      }),
    );
  });

  it("shows a loading status while the first request is pending", () => {
    mocks.get.mockImplementation(() => new Promise(() => {}));
    renderPage();

    const status = screen.getByRole("status");
    expect(status).toHaveTextContent("Loading admin overview…");
  });

  it("shows an error with a retry that requests again", async () => {
    mocks.get.mockImplementationOnce(() => Promise.reject(httpError(500)));
    mocks.get.mockImplementationOnce(() => ok(overview));
    renderPage();

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("We couldn't load the overview");
    fireEvent.click(within(alert).getByRole("button", { name: "Try again" }));

    expect(await screen.findByRole("region", { name: "Accounts" })).toBeInTheDocument();
    expect(mocks.get).toHaveBeenCalledTimes(2);
  });

  it("on 403 shows a muted access message and invalidates the session", async () => {
    mocks.get.mockImplementation(() => Promise.reject(httpError(403)));
    const { invalidate } = renderPage();

    expect(await screen.findByText(/don't have access to the admin overview/)).toBeInTheDocument();
    await waitFor(() =>
      expect(invalidate).toHaveBeenCalledWith({ queryKey: ["auth", "me"] }),
    );
  });

  it("on 401 also invalidates the session", async () => {
    mocks.get.mockImplementation(() => Promise.reject(httpError(401)));
    const { invalidate } = renderPage();

    await waitFor(() =>
      expect(invalidate).toHaveBeenCalledWith({ queryKey: ["auth", "me"] }),
    );
  });

  it("links to the admin tools with the right hrefs", async () => {
    mocks.get.mockImplementation(() => ok(overview));
    renderPage();
    await screen.findByRole("region", { name: "Accounts" });

    expect(screen.getByRole("link", { name: "Moderation queue" })).toHaveAttribute(
      "href",
      routes.admin.jobPostings,
    );
    expect(screen.getByRole("link", { name: "Interview sessions" })).toHaveAttribute(
      "href",
      routes.admin.interviews,
    );
    expect(screen.getByRole("link", { name: "Voice profiles" })).toHaveAttribute(
      "href",
      routes.admin.voices,
    );
    expect(screen.getByRole("link", { name: "Payment transactions" })).toHaveAttribute(
      "href",
      routes.admin.billing,
    );
    expect(screen.getByRole("link", { name: "Revenue reports" })).toHaveAttribute(
      "href",
      routes.admin.revenue,
    );
    expect(screen.getByRole("link", { name: "Accounts" })).toHaveAttribute(
      "href",
      routes.admin.users,
    );
  });

  it("never mentions wallet or membership", async () => {
    mocks.get.mockImplementation(() => ok(overview));
    renderPage();
    await screen.findByRole("region", { name: "Accounts" });

    expect(document.body.textContent ?? "").not.toMatch(/wallet|membership/i);
  });
});
