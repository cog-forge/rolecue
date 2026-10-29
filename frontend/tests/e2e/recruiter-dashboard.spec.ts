import { test, expect, type Page } from "@playwright/test";

const recruiter = {
  id: "22222222-2222-4222-8222-222222222222",
  full_name: "Rec Ruiter",
  email: "rec@example.com",
  image: null,
  role: "recruiter",
  email_verified: true,
  is_locked: false,
  onboarding_role_selected: true,
  onboarding_completed: true,
};

type Totals = Partial<{
  pending: number;
  drafts: number;
  rejected: number;
  unfunded: number;
  live: number;
  closed: number;
}>;

const soon = (hours: number) =>
  new Date(Date.now() + hours * 3_600_000).toISOString();
const eligible = [3, 9, 50].map((hours) => ({
  interview_eligible: true,
  interview_deadline: soon(hours),
}));

// Serves the dashboard's counts the way Go filters them and records every query.
async function mockDashboard(page: Page, totals: Totals = {}) {
  const queries: { path: string; query: Record<string, string> }[] = [];
  await page.route("http://localhost:8080/**", async (route) => {
    const url = new URL(route.request().url());
    const method = route.request().method();
    const headers = {
      "access-control-allow-origin": "http://127.0.0.1:3100",
      "access-control-allow-credentials": "true",
      "access-control-allow-methods": "GET,OPTIONS",
      "access-control-allow-headers": "Content-Type",
    };
    const reply = (items: unknown[], total: number) =>
      route.fulfill({
        headers,
        json: {
          success: true,
          data: { items, pagination: { limit: 1, offset: 0, total } },
        },
      });
    if (method === "OPTIONS") return route.fulfill({ status: 204, headers });
    if (url.pathname === "/auth/me")
      return route.fulfill({
        headers,
        json: { success: true, data: recruiter },
      });
    const query = Object.fromEntries(url.searchParams);
    if (url.pathname === "/recruiter/applications") {
      queries.push({ path: url.pathname, query });
      if (query.status === "pending") return reply([], totals.pending ?? 0);
      const items = totals.pending === 0 ? [] : eligible;
      return reply(items, items.length);
    }
    if (url.pathname === "/job-postings") {
      queries.push({ path: url.pathname, query });
      if (query.status === "pending") return reply([], totals.drafts ?? 0);
      if (query.status === "rejected") return reply([], totals.rejected ?? 0);
      if (query.status === "intake_closed")
        return reply([], totals.closed ?? 0);
      return reply(
        [],
        query.funded === "true" ? (totals.live ?? 0) : (totals.unfunded ?? 0),
      );
    }
    return route.fulfill({
      status: 404,
      headers,
      json: {
        success: false,
        error: { code: "NOT_FOUND", message: "unmocked" },
      },
    });
  });
  return { queries };
}

test("the dashboard lists what needs the recruiter and links to where to do it", async ({
  page,
}) => {
  const api = await mockDashboard(page, {
    pending: 4,
    drafts: 2,
    rejected: 1,
    unfunded: 3,
    live: 5,
    closed: 1,
  });
  await page.goto("/recruiter/dashboard");

  await expect(
    page.getByRole("heading", { level: 1, name: "Dashboard" }),
  ).toBeVisible();
  const list = page.getByRole("list", { name: /need your attention/ });
  await expect(list.getByText("CVs awaiting review")).toBeVisible();
  await expect(list.getByText("Approved, waiting for funding")).toBeVisible();
  // Two of the three eligible deadlines are inside the next 24 hours.
  const deadlines = list
    .getByText("Interview deadlines in the next 24 hours")
    .locator("xpath=ancestor::li");
  await expect(deadlines).toContainText("2");
  await expect(deadlines).toContainText("Earliest:");
  await expect(
    page.getByRole("heading", { name: "Your postings at a glance" }),
  ).toBeVisible();
  expect(
    api.queries.some(
      ({ path, query }) =>
        path === "/job-postings" &&
        query.status === "open" &&
        query.funded === "false",
    ),
  ).toBe(true);

  await list.getByRole("link", { name: "Fund & publish" }).click();
  await expect(page).toHaveURL(/\/recruiter\/job-postings\?status=approved/);
  await expect(page.getByRole("combobox", { name: "Status" })).toHaveText(
    "Needs funding",
  );
});

test("a CV row opens the review inbox on the awaiting-review filter", async ({
  page,
}) => {
  await mockDashboard(page, { pending: 2 });
  await page.goto("/recruiter/dashboard");
  await page.getByRole("link", { name: "Review CVs" }).click();
  await expect(page).toHaveURL(/\/recruiter\/applications\?status=pending/);
});

test("with nothing to do the dashboard says so and offers a new posting", async ({
  page,
}) => {
  await mockDashboard(page, { pending: 0 });
  await page.goto("/recruiter/dashboard");
  await expect(page.getByText("You're all caught up")).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Create a job posting" }),
  ).toHaveAttribute("href", "/recruiter/job-postings/new");
});

test("the recruiter workspace opens on the dashboard at 390px without horizontal overflow", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await mockDashboard(page, { pending: 12, drafts: 1, unfunded: 2 });
  await page.goto("/recruiter");
  await expect(page).toHaveURL(/\/recruiter\/dashboard/);
  await expect(page.getByRole("link", { name: "Review CVs" })).toBeVisible();
  const overflow = await page.evaluate(
    () =>
      document.documentElement.scrollWidth -
      document.documentElement.clientWidth,
  );
  expect(overflow).toBeLessThanOrEqual(0);
});
