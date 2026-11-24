import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import { renderWithIntl } from "@tests/helpers/intl";
import { AdminOverview } from "@/features/admin/overview/components/admin-overview";
import { routes } from "@/config/routes";
import type { AppLocale } from "@/lib/i18n/config";

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
  previous_window: {
    days: 30,
    from: "2026-09-01T00:00:00+07:00",
    to: "2026-10-01T00:00:00+07:00",
  },
  accounts: {
    candidates: 1234,
    recruiters: 56,
    locked: 7,
    new_in_window: 89,
    new_in_previous_window: 0,
  },
  job_postings: {
    pending: 3,
    open: 4,
    intake_closed: 5,
    closed: 6,
    rejected: 2,
  },
  applications: {
    pending: 11,
    interview_eligible: 12,
    interviewed: 13,
    approved: 14,
    rejected: 15,
    submitted_in_window: 16,
    submitted_in_previous_window: 20,
  },
  interviews: {
    practice_started_in_window: 21,
    recruitment_started_in_window: 22,
    completed_in_window: 23,
    evaluation_failed: 24,
  },
  interview_activity: {
    bucket_days: 1,
    buckets: [
      {
        practice: 21,
        recruitment: 22,
        previous_practice: 10,
        previous_recruitment: 10,
      },
    ],
  },
  finance: {
    revenue_vnd: 100000,
    previous_revenue_vnd: 50000,
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
  interview_activity: {
    bucket_days: 1,
    buckets: [
      {
        practice: 0,
        recruitment: 0,
        previous_practice: 0,
        previous_recruitment: 0,
      },
    ],
  },
  accounts: { candidates: 0, recruiters: 0, locked: 0, new_in_window: 0 },
  job_postings: {
    pending: 0,
    open: 0,
    intake_closed: 0,
    closed: 0,
    rejected: 0,
  },
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

const ok = (data: unknown) =>
  Promise.resolve({ data: { success: true, data } });
const httpError = (status: number) =>
  new AxiosError("failed", "ERR", undefined, undefined, {
    status,
    statusText: "",
    headers: {},
    config: { headers: new AxiosHeaders() },
    data: {},
  });

function renderPage(locale: AppLocale = "en") {
  const cache = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const invalidate = vi.spyOn(cache, "invalidateQueries");
  const view = renderWithIntl(
    <QueryClientProvider client={cache}>
      <AdminOverview />
    </QueryClientProvider>,
    { locale },
  );
  return { invalidate, rerender: view.rerender };
}

function sectionOf(title: string) {
  return screen.getByRole("region", { name: title });
}

beforeEach(() => {
  mocks.get.mockReset();
  window.history.replaceState(null, "", routes.admin.dashboard);
});

describe("AdminOverview", () => {
  it("shows comparable KPIs separately from current operational counts", async () => {
    mocks.get.mockImplementation(() => ok(overview));
    renderPage();

    await screen.findByRole("heading", { level: 1, name: "Admin dashboard" });
    const metrics = await screen.findByRole("region", { name: "Key metrics" });
    expect(metrics).toHaveTextContent("Current:");
    expect(metrics).toHaveTextContent("Previous:");
    expect(within(metrics).getByText("89")).toBeInTheDocument();
    expect(within(metrics).getByText("New activity")).toBeInTheDocument();
    expect(within(metrics).getByText("Down 20.0%")).toBeInTheDocument();
    expect(within(metrics).getByText("43")).toBeInTheDocument();
    expect(within(metrics).getByText("100.000 ₫")).toBeInTheDocument();

    const attention = sectionOf("Needs attention");
    expect(attention).toHaveTextContent("Current status across all dates");
    expect(
      within(attention).getByRole("link", { name: "Failed evaluations 24" }),
    ).toHaveAttribute("href", routes.admin.interviews);
    expect(sectionOf("Job postings")).toHaveTextContent("Rejected");
    expect(sectionOf("Applications")).toHaveTextContent("Awaiting CV review");
    expect(sectionOf("Interview activity")).toHaveTextContent(
      "23 completed in the current period",
    );
  });

  it("shows real zero values and an empty activity state", async () => {
    mocks.get.mockImplementation(() => ok(zeroOverview));
    renderPage();

    const metrics = await screen.findByRole("region", { name: "Key metrics" });
    expect(within(metrics).getByText("0 ₫")).toBeInTheDocument();
    expect(
      within(sectionOf("Needs attention")).getByRole("link", {
        name: "Locked accounts 0",
      }),
    ).toBeInTheDocument();
    expect(sectionOf("Interview activity")).toHaveTextContent(
      "No interviews of this type started in either period.",
    );
  });

  it("keeps the current metrics available when an older response omits the chart", async () => {
    mocks.get.mockImplementation(() =>
      ok({
        ...overview,
        previous_window: undefined,
        interview_activity: undefined,
      }),
    );
    renderPage();

    await screen.findByRole("region", { name: "Key metrics" });
    expect(sectionOf("Interview activity")).toHaveTextContent(
      "The interview trend is currently unavailable.",
    );
  });

  it("requests the window from the URL and shows the default when absent", async () => {
    mocks.get.mockImplementation(() => ok(overview));
    renderPage();

    await screen.findByRole("region", { name: "Key metrics" });
    expect(mocks.get).toHaveBeenCalledWith("/admin/overview", {
      params: { days: 30 },
      signal: expect.anything(),
    });
  });

  it("switches the window through the selector and requests the new days", async () => {
    mocks.get.mockImplementation(() => ok(overview));
    const { rerender } = renderPage();
    await screen.findByRole("region", { name: "Key metrics" });

    const selector = screen.getByRole("navigation", {
      name: "Reporting window",
    });
    const last7 = within(selector).getByRole("link", { name: "Last 7 days" });
    expect(last7).toHaveAttribute("href", `${routes.admin.dashboard}?days=7`);
    expect(
      within(selector).getByRole("link", { name: "Last 30 days" }),
    ).toHaveAttribute("aria-current", "page");

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

  it("marks the numbers as stale while another window loads", async () => {
    mocks.get.mockImplementationOnce(() => ok(overview));
    const cache = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const page = (
      <QueryClientProvider client={cache}>
        <AdminOverview />
      </QueryClientProvider>
    );
    const { rerender } = renderWithIntl(page);
    const accounts = await screen.findByRole("region", { name: "Key metrics" });
    expect(accounts.closest('[aria-busy="true"]')).toBeNull();
    expect(screen.getByText(/Last 30 days · Updated/)).toBeInTheDocument();

    // The next window never answers, so the previous numbers stay on screen, marked as not current.
    mocks.get.mockImplementation(() => new Promise(() => {}));
    fireEvent.click(
      within(
        screen.getByRole("navigation", { name: "Reporting window" }),
      ).getByRole("link", { name: "Last 90 days" }),
    );
    rerender(page);

    await waitFor(() =>
      expect(
        screen
          .getByRole("region", { name: "Key metrics" })
          .closest('[aria-busy="true"]'),
      ).not.toBeNull(),
    );
    expect(screen.getByText("Loading the last 90 days…")).toBeInTheDocument();
    // The label still says which window these numbers are for.
    expect(screen.getByText(/Last 30 days · Updated/)).toBeInTheDocument();
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

    expect(
      await screen.findByRole("region", { name: "Key metrics" }),
    ).toBeInTheDocument();
    expect(mocks.get).toHaveBeenCalledTimes(2);
  });

  it("on 403 shows a muted access message and invalidates the session", async () => {
    mocks.get.mockImplementation(() => Promise.reject(httpError(403)));
    const { invalidate } = renderPage();

    expect(
      await screen.findByText(/don't have access to the admin overview/),
    ).toBeInTheDocument();
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

  it("links to the relevant admin destinations", async () => {
    mocks.get.mockImplementation(() => ok(overview));
    renderPage();
    await screen.findByRole("region", { name: "Key metrics" });

    const attention = sectionOf("Needs attention");
    expect(
      within(attention).getByRole("link", {
        name: "Postings awaiting review 3",
      }),
    ).toHaveAttribute("href", routes.admin.jobPostings);
    expect(
      within(attention).getByRole("link", { name: "Locked accounts 7" }),
    ).toHaveAttribute("href", routes.admin.users);
    expect(
      screen.getByRole("link", { name: "Successful PayOS top-ups" }),
    ).toHaveAttribute("href", routes.admin.revenue);
  });

  it("never mentions wallet or membership", async () => {
    mocks.get.mockImplementation(() => ok(overview));
    renderPage();
    await screen.findByRole("region", { name: "Key metrics" });

    expect(document.body.textContent ?? "").not.toMatch(/wallet|membership/i);
  });

  it("renders the dashboard in Vietnamese with localized numbers and the same VND format", async () => {
    mocks.get.mockImplementation(() => ok(overview));
    renderPage("vi");

    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: "Bảng điều khiển quản trị",
      }),
    ).toBeInTheDocument();
    const metrics = await screen.findByRole("region", {
      name: "Chỉ số trong kỳ",
    });
    expect(within(metrics).getByText("100.000 ₫")).toBeInTheDocument();
    expect(within(metrics).getByText("Giảm 20,0%")).toBeInTheDocument();
    expect(sectionOf("Cần chú ý")).toHaveTextContent("Tài khoản đang khóa");
    expect(screen.getByText(/30 ngày gần nhất · Cập nhật/)).toBeInTheDocument();
    const selector = screen.getByRole("navigation", {
      name: "Khoảng thời gian báo cáo",
    });
    expect(
      within(selector).getByRole("link", { name: "7 ngày gần nhất" }),
    ).toHaveAttribute("href", `${routes.admin.dashboard}?days=7`);
    expect(
      screen.getByRole("link", { name: "Tài khoản đang khóa 7" }),
    ).toHaveAttribute("href", routes.admin.users);
  });

  it("shows the error state in Vietnamese", async () => {
    mocks.get.mockImplementation(() => Promise.reject(httpError(500)));
    renderPage("vi");

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Không thể tải tổng quan");
    expect(
      within(alert).getByRole("button", { name: "Thử lại" }),
    ).toBeInTheDocument();
  });
});
