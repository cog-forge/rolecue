import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AdminRevenueView } from "@/features/admin/billing/components/admin-revenue-view";
import type { RevenueReport } from "@/features/admin/billing/schemas/billing-schema";
import { octoberReport } from "../fixtures/revenue-report";

const mocks = vi.hoisted(() => ({ get: vi.fn() }));

vi.mock("@/lib/api/client", () => ({ apiClient: { get: mocks.get } }));

// Radix Select calls scrollIntoView when it opens; jsdom does not implement it.
if (!HTMLElement.prototype.scrollIntoView)
  Object.defineProperty(HTMLElement.prototype, "scrollIntoView", {
    configurable: true,
    value: () => {},
  });

// 27/10/2026 15:00 in Vietnam.
const NOW = new Date("2026-10-27T08:00:00Z");

async function chooseOption(label: string, name: string) {
  fireEvent.keyDown(screen.getByRole("combobox", { name: label }), {
    key: "ArrowDown",
  });
  fireEvent.keyDown(await screen.findByRole("option", { name }), {
    key: "Enter",
  });
}

function lastParams() {
  const calls = mocks.get.mock.calls.filter(
    ([url]) => url === "/admin/billing/revenue-report",
  );
  return calls.at(-1)?.[1]?.params as Record<string, unknown> | undefined;
}

function respondWith(report: RevenueReport) {
  mocks.get.mockImplementation((url: string) =>
    url === "/admin/billing/revenue-report"
      ? Promise.resolve({ data: { success: true, data: report } })
      : Promise.reject(new Error(`Unhandled: ${url}`)),
  );
}

function renderView() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return renderWithIntl(
    <QueryClientProvider client={queryClient}>
      <AdminRevenueView />
    </QueryClientProvider>,
  );
}

describe("AdminRevenueView", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  beforeEach(() => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(NOW);
    vi.clearAllMocks();
    respondWith(octoberReport);
  });

  it("opens on this month against the same days last year and states the finding", async () => {
    renderView();

    expect(
      screen.getByRole("heading", { level: 1, name: "Revenue" }),
    ).toBeInTheDocument();
    expect(
      await screen.findByText(
        "1–27 Oct 2026 · 200.000 ₫, up 33% on 1–27 Oct 2025 (+50.000 ₫)",
      ),
    ).toBeInTheDocument();
    expect(lastParams()).toEqual({
      date_from: "2026-10-01T00:00:00+07:00",
      date_to: "2026-10-28T00:00:00+07:00",
      compare: "previous_year",
    });
    expect(
      screen.getByRole("combobox", { name: "Time window" }),
    ).toHaveTextContent("This month");
  });

  it("shows each headline figure with its change and baseline", async () => {
    renderView();

    const kpis = await screen.findByRole("region", {
      name: "Headline figures",
    });
    const revenue = within(kpis).getByRole("group", { name: "Revenue" });
    expect(revenue).toHaveTextContent("200.000 ₫");
    expect(revenue).toHaveTextContent("33%");
    expect(revenue).toHaveTextContent("+50.000 ₫");
    expect(revenue).toHaveTextContent("vs 1–27 Oct 2025");

    const payers = within(kpis).getByRole("group", { name: "Paying users" });
    expect(payers).toHaveTextContent("2");
    expect(payers).toHaveTextContent("No change");
    expect(payers).toHaveTextContent("0 new");

    expect(
      within(kpis).getByRole("group", { name: "Average order" }),
    ).toHaveTextContent("−8.334 ₫");
    expect(
      within(kpis).getByRole("group", { name: "Payment success" }),
    ).toHaveTextContent("16.7 pts");
  });

  it("says New when there is nothing to compare with", async () => {
    respondWith({
      ...octoberReport,
      comparison: { ...octoberReport.comparison, revenue_vnd: 0 },
    });
    renderView();

    expect(
      await screen.findByText(
        "1–27 Oct 2026 · 200.000 ₫; no paid top-ups in 1–27 Oct 2025",
      ),
    ).toBeInTheDocument();
    const revenue = screen.getByRole("group", { name: "Revenue" });
    expect(revenue).toHaveTextContent("New");
  });

  it("switches the comparison and the period", async () => {
    renderView();
    await screen.findByRole("region", { name: "Headline figures" });

    await chooseOption("Compare with", "Previous period");
    await waitFor(() =>
      expect(lastParams()).toMatchObject({ compare: "previous_period" }),
    );

    await chooseOption("Time window", "Last 12 months");
    await waitFor(() =>
      expect(lastParams()).toEqual({
        date_from: "2025-11-01T00:00:00+07:00",
        date_to: "2026-10-28T00:00:00+07:00",
        compare: "previous_period",
      }),
    );
  });

  it("explains where the money came from and what coins are still owed", async () => {
    renderView();

    const mix = await screen.findByRole("region", {
      name: "Where revenue came from",
    });
    expect(within(mix).getByText("Recruiters")).toBeInTheDocument();
    expect(within(mix).getByText("Candidates")).toBeInTheDocument();
    expect(within(mix).getByText("Standard")).toBeInTheDocument();

    const coins = screen.getByRole("region", { name: "Coins" });
    expect(within(coins).getByText("Interview slots")).toBeInTheDocument();
    expect(within(coins).getByText("Practice interviews")).toBeInTheDocument();
    expect(within(coins).getByText("Avatar storage")).toBeInTheDocument();

    const owed = screen.getByRole("region", { name: "Coins users still hold" });
    expect(owed).toHaveTextContent("1,258 coins");
    expect(owed).toHaveTextContent("1.258.000 ₫");
    expect(owed).toHaveTextContent("not revenue");
  });

  it("keeps the frame and offers a retry when the report fails", async () => {
    mocks.get.mockRejectedValue(new Error("Network Error"));
    renderView();

    expect(
      await screen.findByText("Couldn't load the revenue report."),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /retry/i })).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { level: 1, name: "Revenue" }),
    ).toBeInTheDocument();
  });
});
