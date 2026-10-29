import { test, expect, type Locator, type Page } from "@playwright/test";

const adminUser = {
  id: "11111111-1111-4111-8111-111111111111",
  full_name: "Ada Admin",
  email: "admin@example.com",
  image: null,
  role: "admin",
  email_verified: true,
  is_locked: false,
  onboarding_role_selected: true,
  onboarding_completed: true,
};
const candidateUser = {
  ...adminUser,
  id: "22222222-2222-4222-8222-222222222222",
  full_name: "Cara Candidate",
  email: "candidate@example.com",
  role: "candidate",
};
const recruiterUser = {
  ...adminUser,
  id: "33333333-3333-4333-8333-333333333333",
  full_name: "Rec Ruiter",
  email: "rec@example.com",
  role: "recruiter",
};

const corsHeaders = {
  "access-control-allow-origin": "http://127.0.0.1:3100",
  "access-control-allow-credentials": "true",
  "access-control-allow-methods": "GET,OPTIONS",
  "access-control-allow-headers": "Content-Type",
};

// Distinct numbers per tile so each assertion can only match its own tile.
function overviewFor(days: number) {
  const generatedAt = "2026-10-30T09:00:00+07:00";
  const from = new Date(
    Date.parse(generatedAt) - days * 24 * 60 * 60 * 1000,
  ).toISOString();
  return {
    generated_at: generatedAt,
    window: { days, from, to: generatedAt },
    accounts: {
      candidates: 1234,
      recruiters: 56,
      locked: 7,
      new_in_window: 89,
    },
    job_postings: {
      pending: 11,
      open: 22,
      intake_closed: 33,
      closed: 44,
      rejected: 55,
    },
    applications: {
      pending: 101,
      interview_eligible: 102,
      interviewed: 103,
      approved: 104,
      rejected: 105,
      submitted_in_window: 106,
    },
    interviews: {
      practice_started_in_window: 201,
      recruitment_started_in_window: 202,
      completed_in_window: 203,
      evaluation_failed: 204,
    },
    finance: {
      revenue_vnd: 100000,
      successful_deposits: 9,
      coins_sold: 1500,
      coins_spent: 640,
      coins_refunded: 12,
      unique_payers: 8,
      success_rate: 87.5,
    },
  };
}

// Serves /auth/me and /admin/overview and records every requested window.
async function mockOverview(page: Page, user = adminUser) {
  const api = {
    requestedDays: [] as string[],
    mode: "ok" as "ok" | "error",
  };
  await page.route("http://localhost:8080/**", async (route) => {
    const url = new URL(route.request().url());
    if (route.request().method() === "OPTIONS")
      return route.fulfill({ status: 204, headers: corsHeaders });
    if (url.pathname === "/auth/me")
      return route.fulfill({
        headers: corsHeaders,
        json: { success: true, data: user },
      });
    if (url.pathname === "/admin/overview") {
      const days = url.searchParams.get("days") ?? "";
      api.requestedDays.push(days);
      if (api.mode === "error")
        return route.fulfill({
          status: 500,
          headers: corsHeaders,
          json: {
            success: false,
            error: { code: "INTERNAL", message: "boom" },
          },
        });
      return route.fulfill({
        headers: corsHeaders,
        json: { success: true, data: overviewFor(Number(days) || 30) },
      });
    }
    return route.fulfill({
      status: 404,
      headers: corsHeaders,
      json: {
        success: false,
        error: { code: "NOT_FOUND", message: "unmocked" },
      },
    });
  });
  return api;
}

// The value next to a tile label inside one section (the dt/dd pair of the definition list).
function tile(section: Locator, label: string) {
  return section.locator(
    `xpath=.//dt[normalize-space()="${label}"]/following-sibling::dd[1]`,
  );
}

test("admin sees the five overview sections with the mocked numbers", async ({
  page,
}) => {
  await mockOverview(page);
  await page.goto("/admin/dashboard");

  await expect(
    page.getByRole("heading", {
      level: 1,
      name: "Admin dashboard",
      exact: true,
    }),
  ).toBeVisible();
  await expect(page.getByText(/^Last 30 days · Updated/)).toBeVisible();

  const accounts = page.getByRole("region", { name: "Accounts", exact: true });
  await expect(tile(accounts, "Candidates")).toHaveText("1,234");
  await expect(tile(accounts, "Recruiters")).toHaveText("56");
  await expect(tile(accounts, "Locked now")).toHaveText("7");
  await expect(tile(accounts, "New in window")).toHaveText("89");

  const jobs = page.getByRole("region", { name: "Job postings", exact: true });
  await expect(tile(jobs, "Pending review")).toHaveText("11");
  await expect(tile(jobs, "Open")).toHaveText("22");
  await expect(tile(jobs, "Intake closed")).toHaveText("33");
  await expect(tile(jobs, "Closed")).toHaveText("44");
  await expect(tile(jobs, "Rejected")).toHaveText("55");

  const applications = page.getByRole("region", {
    name: "Applications",
    exact: true,
  });
  await expect(tile(applications, "Awaiting CV review")).toHaveText("101");
  await expect(tile(applications, "Interview eligible")).toHaveText("102");
  await expect(tile(applications, "Interviewed")).toHaveText("103");
  await expect(tile(applications, "Approved")).toHaveText("104");
  await expect(tile(applications, "Rejected")).toHaveText("105");
  await expect(tile(applications, "Submitted in window")).toHaveText("106");

  const interviews = page.getByRole("region", {
    name: "Interviews",
    exact: true,
  });
  await expect(tile(interviews, "Practice started")).toHaveText("201");
  await expect(tile(interviews, "Recruitment started")).toHaveText("202");
  await expect(tile(interviews, "Completed")).toHaveText("203");
  await expect(tile(interviews, "Evaluation failed")).toHaveText("204");

  const finance = page.getByRole("region", { name: "Finance", exact: true });
  await expect(tile(finance, "Revenue")).toHaveText(/100\.000\s₫/);
  await expect(tile(finance, "Successful top-ups")).toHaveText("9");
  await expect(tile(finance, "Coins sold")).toHaveText("1,500");
  await expect(tile(finance, "Coins spent")).toHaveText("640");
  await expect(tile(finance, "Coins refunded")).toHaveText("12");
  await expect(tile(finance, "Unique payers")).toHaveText("8");
  await expect(tile(finance, "Payment success rate")).toHaveText("87.5%");
});

test("the reporting window lives in the URL and drives the request", async ({
  page,
}) => {
  const api = await mockOverview(page);
  await page.goto("/admin/dashboard");

  const selector = page.getByRole("navigation", { name: "Reporting window" });
  await expect(
    selector.getByRole("link", { name: "Last 30 days", exact: true }),
  ).toHaveAttribute("aria-current", "page");
  expect(api.requestedDays.at(-1)).toBe("30");

  await selector
    .getByRole("link", { name: "Last 7 days", exact: true })
    .click();
  await expect(page).toHaveURL(/\/admin\/dashboard\?days=7$/);
  await expect.poll(() => api.requestedDays.at(-1)).toBe("7");
  await expect(
    selector.getByRole("link", { name: "Last 7 days", exact: true }),
  ).toHaveAttribute("aria-current", "page");
  await expect(
    selector.getByRole("link", { name: "Last 30 days", exact: true }),
  ).not.toHaveAttribute("aria-current");

  await page.goto("/admin/dashboard?days=90");
  await expect.poll(() => api.requestedDays.at(-1)).toBe("90");
  await expect(
    selector.getByRole("link", { name: "Last 90 days", exact: true }),
  ).toHaveAttribute("aria-current", "page");
});

test("an unsupported window in the URL falls back to 30 days", async ({
  page,
}) => {
  const api = await mockOverview(page);
  await page.goto("/admin/dashboard?days=5");

  await expect(
    page.getByRole("heading", {
      level: 1,
      name: "Admin dashboard",
      exact: true,
    }),
  ).toBeVisible();
  await expect.poll(() => api.requestedDays.at(-1)).toBe("30");
  expect(api.requestedDays).not.toContain("5");
  await expect(
    page
      .getByRole("navigation", { name: "Reporting window" })
      .getByRole("link", { name: "Last 30 days", exact: true }),
  ).toHaveAttribute("aria-current", "page");
});

test("quick links point to the admin routes", async ({ page }) => {
  await mockOverview(page);
  await page.goto("/admin/dashboard");

  const tools = page.getByRole("navigation", { name: "Admin tools" });
  const expected: Array<[string, string]> = [
    ["Moderation queue", "/admin/job-postings"],
    ["Interview sessions", "/admin/interviews"],
    ["Voice profiles", "/admin/voices"],
    ["Payment transactions", "/admin/billing"],
    ["Revenue reports", "/admin/revenue"],
    ["Accounts", "/admin/users"],
  ];
  for (const [label, href] of expected)
    await expect(
      tools.getByRole("link", { name: label, exact: true }),
    ).toHaveAttribute("href", href);
});

test("a failed load shows the error state and Try again recovers", async ({
  page,
}) => {
  const api = await mockOverview(page);
  api.mode = "error";
  await page.goto("/admin/dashboard");

  const alert = page
    .getByRole("alert")
    .filter({ hasText: "We couldn't load the overview" });
  await expect(alert).toBeVisible();
  await expect(
    page.getByRole("region", { name: "Accounts", exact: true }),
  ).toHaveCount(0);

  api.mode = "ok";
  await alert.getByRole("button", { name: "Try again", exact: true }).click();
  await expect(alert).toHaveCount(0);
  await expect(
    page.getByRole("region", { name: "Accounts", exact: true }),
  ).toBeVisible();
  await expect(
    tile(
      page.getByRole("region", { name: "Accounts", exact: true }),
      "Candidates",
    ),
  ).toHaveText("1,234");
});

for (const user of [candidateUser, recruiterUser])
  test(`${user.role} never sees the admin overview or requests it`, async ({
    page,
  }) => {
    const api = await mockOverview(page, user);
    await page.goto("/admin/dashboard");

    await expect(page).toHaveURL(
      user.role === "candidate" ? /\/dashboard$/ : /\/recruiter\/dashboard$/,
    );
    await expect(
      page.getByRole("heading", {
        level: 1,
        name: "Admin dashboard",
        exact: true,
      }),
    ).toHaveCount(0);
    expect(api.requestedDays).toEqual([]);
  });

test("the overview shows no membership or subscription copy", async ({
  page,
}) => {
  await mockOverview(page);
  await page.goto("/admin/dashboard");
  await expect(
    page.getByRole("region", { name: "Finance", exact: true }),
  ).toBeVisible();

  const text = await page.getByRole("main").innerText();
  expect(text).not.toMatch(/membership/i);
  expect(text).not.toMatch(/subscription/i);
});
