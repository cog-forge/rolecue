import { expect, test, type Page } from "@playwright/test";

const candidate = {
  id: "11111111-1111-4111-8111-111111111111",
  full_name: "Cam Candidate",
  email: "cand@example.com",
  image: null,
  role: "candidate",
  email_verified: true,
  is_locked: false,
  onboarding_role_selected: true,
  onboarding_completed: true,
};
const headers = {
  "access-control-allow-origin": "http://127.0.0.1:3100",
  "access-control-allow-credentials": "true",
  "access-control-allow-methods": "GET,POST,OPTIONS",
  "access-control-allow-headers": "Content-Type",
};

const roleProfileId = "eeeeeeee-eeee-4eee-8eee-000000000001";
const interviewId = "bbbbbbbb-bbbb-4bbb-8bbb-000000000001";
const applicationId = "cccccccc-cccc-4ccc-8ccc-000000000001";
const postingId = "33333333-3333-4333-8333-333333333333";

type Failing = "roleProfiles" | "practice" | "applications" | "wallet";

type Options = {
  // Endpoints that answer 500 for the first `failures` requests; endpoints not listed always succeed.
  failing?: Partial<Record<Failing, number>>;
  rich?: boolean;
  empty?: boolean;
};

// Serves the four dashboard reads from the Go API shapes. A failing endpoint answers 500 for the
// configured number of requests, then recovers, so a test can check Try again.
async function mockDashboard(page: Page, options: Options = {}) {
  const hits: Record<string, number> = {};
  const failing = { ...options.failing };
  const fail = (key: Failing) => {
    hits[key] = (hits[key] ?? 0) + 1;
    const budget = failing[key];
    return budget !== undefined && hits[key] <= budget;
  };
  await page.route("http://localhost:8080/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const method = request.method();
    if (method === "OPTIONS") return route.fulfill({ status: 204, headers });
    const ok = (data: unknown) =>
      route.fulfill({ headers, json: { success: true, data } });
    const broken = () =>
      route.fulfill({
        status: 500,
        headers,
        json: {
          success: false,
          error: { code: "INTERNAL", message: "unavailable" },
        },
      });

    if (url.pathname === "/auth/me") return ok(candidate);
    if (/^\/interviews\/[^/]+\/report$/.test(url.pathname))
      return ok({
        interview_id: url.pathname.split("/")[2],
        type: "practice",
        status: "completed",
        overall_score: 82,
        started_at: "2026-11-12T08:00:00Z",
        completed_at: "2026-11-12T08:30:00Z",
        summary: "Explain your trade-offs before choosing a solution.",
        breakdown: [
          {
            metric_id: "11111111-aaaa-4aaa-8aaa-000000000001",
            name: "Technical Accuracy",
            score: 68,
            weight: 40,
          },
          {
            metric_id: "11111111-aaaa-4aaa-8aaa-000000000002",
            name: "Problem Solving",
            score: 74,
            weight: 35,
          },
          {
            metric_id: "11111111-aaaa-4aaa-8aaa-000000000003",
            name: "Communication",
            score: 88,
            weight: 25,
          },
        ],
        questions: [],
      });
    if (url.pathname === "/role-profiles" && method === "GET")
      return fail("roleProfiles")
        ? broken()
        : ok({
            items: options.empty
              ? []
              : options.rich
                ? [
                    {
                      id: roleProfileId,
                      title: "Senior Backend Engineer",
                      seniorityLevel: "senior",
                      status: "confirmed",
                      createdAt: "2026-11-01T08:00:00Z",
                    },
                    {
                      id: "eeeeeeee-eeee-4eee-8eee-000000000002",
                      title: "Platform Engineer",
                      seniorityLevel: "mid",
                      status: "review",
                      createdAt: "2026-11-02T08:00:00Z",
                    },
                    {
                      id: "eeeeeeee-eeee-4eee-8eee-000000000003",
                      title: "Full Stack Developer",
                      seniorityLevel: "mid",
                      status: "draft",
                      createdAt: "2026-11-03T08:00:00Z",
                    },
                  ]
                : [
                    {
                      id: roleProfileId,
                      title: "Frontend Developer",
                      seniorityLevel: "mid",
                      status: "review",
                      createdAt: "2026-10-01T08:00:00Z",
                    },
                  ],
            pagination: {
              limit: 3,
              offset: 0,
              total: options.empty ? 0 : options.rich ? 3 : 1,
            },
          });
    if (url.pathname === "/interviews" && method === "GET")
      return fail("practice")
        ? broken()
        : ok({
            items: options.empty
              ? []
              : options.rich
                ? Array.from({ length: 20 }, (_, index) => ({
                    id: `bbbbbbbb-bbbb-4bbb-8bbb-${String(index + 1).padStart(12, "0")}`,
                    type: "practice",
                    started_at: new Date(
                      Date.UTC(2026, 10, 12 - index, 8),
                    ).toISOString(),
                    completed_at:
                      index === 0
                        ? null
                        : new Date(
                            Date.UTC(2026, 10, 12 - index, 8, 30),
                          ).toISOString(),
                    evaluation_status: index === 0 ? "pending" : "completed",
                    status: index === 0 ? "in_progress" : "completed",
                    score:
                      index === 0 || index === 7
                        ? null
                        : [
                            84, 82, 79, 67, 74, 71, 62, 0, 69, 64, 66, 58, 65,
                            60, 55, 63, 48, 57, 54, 51,
                          ][index],
                    score_outcome:
                      index === 3
                        ? "partial"
                        : index === 7
                          ? "withheld"
                          : index === 0
                            ? null
                            : "complete",
                    role_profile_title:
                      index % 4 === 0
                        ? "Platform Engineer"
                        : "Senior Backend Engineer",
                    job_posting_title: null,
                    application_id: null,
                  }))
                : [
                    {
                      id: interviewId,
                      type: "practice",
                      started_at: "2026-10-10T08:00:00Z",
                      completed_at: "2026-10-10T08:30:00Z",
                      evaluation_status: "completed",
                      status: "completed",
                      score: 82,
                      role_profile_title: "Frontend Developer",
                      job_posting_title: null,
                      application_id: null,
                    },
                  ],
            total: options.empty ? 0 : options.rich ? 32 : 1,
            page: 1,
            page_size: 20,
          });
    if (url.pathname === "/applications" && method === "GET")
      return fail("applications")
        ? broken()
        : ok({
            items: options.empty
              ? []
              : options.rich
                ? [
                    {
                      id: applicationId,
                      job_posting_id: postingId,
                      job_posting_title: "Backend Engineer",
                      company_name: "Northstar Labs",
                      status: "interview_eligible",
                      reject_reason: null,
                      interview_deadline: "2026-11-18T17:00:00Z",
                      can_start_interview: true,
                      created_at: "2026-11-11T08:00:00Z",
                    },
                    {
                      id: "cccccccc-cccc-4ccc-8ccc-000000000002",
                      job_posting_id: postingId,
                      job_posting_title: "Software Engineer",
                      company_name: "Arc Studio",
                      status: "pending",
                      reject_reason: null,
                      interview_deadline: null,
                      can_start_interview: false,
                      created_at: "2026-11-10T08:00:00Z",
                    },
                    {
                      id: "cccccccc-cccc-4ccc-8ccc-000000000003",
                      job_posting_id: postingId,
                      job_posting_title: "Platform Engineer",
                      company_name: "Meridian",
                      status: "approved",
                      reject_reason: null,
                      interview_deadline: null,
                      can_start_interview: false,
                      created_at: "2026-11-08T08:00:00Z",
                    },
                  ]
                : [
                    {
                      id: applicationId,
                      job_posting_id: postingId,
                      job_posting_title: "Backend Engineer",
                      status: "interview_eligible",
                      reject_reason: null,
                      interview_deadline: "2099-01-01T08:00:00Z",
                      can_start_interview: true,
                      created_at: "2026-10-12T08:00:00Z",
                    },
                  ],
            total: options.empty ? 0 : options.rich ? 9 : 1,
            page: 1,
            page_size: 20,
          });
    if (url.pathname === "/wallet" && method === "GET")
      return fail("wallet")
        ? broken()
        : ok({ id: "wallet", user_id: candidate.id, balance: 250 });
    return route.fulfill({
      status: 404,
      headers,
      json: {
        success: false,
        error: { code: "NOT_FOUND", message: "unmocked" },
      },
    });
  });
  // Stops the endpoint from failing, whoever asks for it next (other widgets can share a request).
  const recover = (key: Failing) => {
    failing[key] = 0;
  };
  return { hits, recover };
}

test("the dashboard shows the Target JDs, practice, application and coin cards from the API", async ({
  page,
}) => {
  await mockDashboard(page);
  await page.goto("/dashboard");

  await expect(
    page.getByRole("heading", { level: 1, name: "Dashboard" }),
  ).toBeVisible();

  const targetJds = page.getByRole("region", { name: "Target JDs" });
  await expect(targetJds).toContainText("1 Target JD");
  await expect(targetJds).toContainText(
    "Next step: Waiting for your review for Frontend Developer.",
  );

  const practice = page.getByRole("region", { name: "Practice activity" });
  await expect(practice).toContainText("1 session");
  await expect(practice).toContainText("Frontend Developer");
  await expect(practice).toContainText("82.00/100");

  const applications = page.getByRole("region", { name: "Applications" });
  await expect(applications).toContainText("1 application");
  await expect(applications).toContainText(
    "Next step: start the interview for Backend Engineer.",
  );

  const coins = page.getByRole("region", { name: "Coin balance" });
  await expect(coins).toContainText("250 coins");
});

test("when the applications request fails, that card shows its error and the other cards still load", async ({
  page,
}, testInfo) => {
  await mockDashboard(page, { failing: { applications: Infinity } });
  await page.goto("/dashboard");

  const applications = page.getByRole("region", { name: "Applications" });
  await expect(
    applications.getByText("We couldn't load your applications."),
  ).toBeVisible();
  await expect(
    applications.getByRole("button", { name: "Try again" }),
  ).toBeEnabled();

  await expect(page.getByRole("region", { name: "Target JDs" })).toContainText(
    "1 Target JD",
  );
  await expect(
    page.getByRole("region", { name: "Practice activity" }),
  ).toContainText("82.00/100");
  await expect(
    page.getByRole("region", { name: "Coin balance" }),
  ).toContainText("250 coins");
  await page.screenshot({
    path: testInfo.outputPath("candidate-dashboard-error.png"),
    fullPage: true,
    animations: "disabled",
  });
});

test("Try again reloads a failed card once the request recovers", async ({
  page,
}) => {
  const { hits, recover } = await mockDashboard(page, {
    failing: { wallet: Infinity },
  });
  await page.goto("/dashboard");

  const coins = page.getByRole("region", { name: "Coin balance" });
  await expect(
    coins.getByText("We couldn't load your coin balance."),
  ).toBeVisible();

  recover("wallet");
  await coins.getByRole("button", { name: "Try again" }).click();

  await expect(coins).toContainText("250 coins");
  await expect(
    coins.getByText("We couldn't load your coin balance."),
  ).toHaveCount(0);
  expect(hits.wallet).toBeGreaterThanOrEqual(2);
});

test("the dashboard switches to Vietnamese and keeps stored content", async ({
  page,
}) => {
  await mockDashboard(page);
  await page.goto("/dashboard");
  await page.getByRole("button", { name: /Change language/ }).click();
  await page.getByRole("menuitemradio", { name: "Tiếng Việt" }).click();
  await expect(
    page.getByRole("heading", { name: "Tổng quan", level: 1 }),
  ).toBeVisible();
  await expect(
    page.getByRole("region", { name: "Đơn ứng tuyển" }),
  ).toContainText("Backend Engineer");
});

for (const width of [320, 390, 768, 1280, 1440]) {
  test(`candidate bento dashboard fits ${width}px and shows a bounded score trend`, async ({
    page,
  }, testInfo) => {
    await page.setViewportSize({ width, height: 1000 });
    await page.emulateMedia({ reducedMotion: "reduce" });
    await mockDashboard(page, { rich: true });
    await page.goto("/dashboard");
    await expect(
      page
        .getByRole("region", { name: "Practice progress" })
        .getByText("82.00", { exact: true }),
    ).toBeVisible();
    await expect(
      page.getByRole("progressbar", { name: "Technical Accuracy" }),
    ).toHaveAttribute("aria-valuenow", "68");
    await expect(
      page.getByRole("region", { name: "Your next step" }),
    ).toContainText("Complete by");
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    await page.screenshot({
      path: testInfo.outputPath("candidate-dashboard.png"),
      fullPage: true,
      animations: "disabled",
    });
    await page
      .getByRole("combobox", { name: "Practice history window" })
      .click();
    await page
      .getByRole("option", { name: "Last 20 sessions", exact: true })
      .click();
    await expect(
      page.getByText("Your latest 20 sessions", { exact: false }),
    ).toBeVisible();
  });
}

test("candidate dashboard supports a genuine first-use state", async ({
  page,
}, testInfo) => {
  await mockDashboard(page, { empty: true });
  await page.goto("/dashboard");
  await expect(page.getByText("No complete scores yet")).toBeVisible();
  await expect(
    page.getByText("Feedback after your first practice"),
  ).toBeVisible();
  await expect(
    page
      .getByRole("region", { name: "Practice progress" })
      .locator(".recharts-area"),
  ).toHaveCount(0);
  await page.screenshot({
    path: testInfo.outputPath("candidate-dashboard-empty.png"),
    fullPage: true,
    animations: "disabled",
  });
});

test("candidate dashboard remains readable in dark mode with reduced motion", async ({
  page,
}, testInfo) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.emulateMedia({ colorScheme: "dark", reducedMotion: "reduce" });
  await mockDashboard(page, { rich: true });
  await page.goto("/dashboard");
  await expect(
    page.getByRole("progressbar", { name: "Communication" }),
  ).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  const axisFill = await page
    .locator(".recharts-cartesian-axis-tick-value")
    .first()
    .evaluate((node) => getComputedStyle(node).fill);
  const labelColor = await page
    .getByText("Latest complete score", { exact: true })
    .evaluate((node) => getComputedStyle(node).color);
  expect(axisFill).toBe(labelColor);
  await page.evaluate(() => document.fonts.ready);
  const reveal = page
    .locator('clipPath[id^="animationClipPath-"] rect')
    .first();
  const initialWidth = await reveal.getAttribute("width");
  await page.evaluate(
    () =>
      new Promise<void>((resolve) =>
        requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
      ),
  );
  expect(await reveal.getAttribute("width")).toBe(initialWidth);

  await page.screenshot({
    path: testInfo.outputPath("candidate-dashboard-dark.png"),
    fullPage: true,
    animations: "disabled",
  });
});

test("dashboard skeletons stay visible until their own requests resolve", async ({
  page,
}, testInfo) => {
  await mockDashboard(page, { rich: true });
  const release: Array<() => void> = [];
  await page.route("http://localhost:8080/**", async (route) => {
    if (
      ["/role-profiles", "/interviews", "/applications", "/wallet"].includes(
        new URL(route.request().url()).pathname,
      ) &&
      route.request().method() === "GET"
    )
      await new Promise<void>((resolve) => release.push(resolve));
    return route.fallback();
  });
  await page.goto("/dashboard");
  await expect(
    page.getByRole("status", { name: "Loading Practice progress…" }),
  ).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath("candidate-dashboard-loading.png"),
    fullPage: true,
    animations: "disabled",
  });
  await expect.poll(() => release.length).toBe(4);
  release.forEach((resolve) => resolve());
  await expect(
    page
      .getByRole("region", { name: "Practice progress" })
      .getByText("82.00", { exact: true }),
  ).toBeVisible();
});

test("practice score line draws on load and replays when the window changes", async ({
  page,
}) => {
  await page.emulateMedia({ reducedMotion: "no-preference" });
  await page.addInitScript(() => {
    const samples: Record<string, number[]> = {};
    Object.defineProperty(window, "__practiceDrawWidths", { value: samples });
    new MutationObserver(() => {
      for (const rect of document.querySelectorAll(
        'clipPath[id^="animationClipPath-"] rect',
      )) {
        const id = rect.parentElement?.id;
        const width = Number(rect.getAttribute("width"));
        if (id && Number.isFinite(width)) {
          const values = samples[id] ?? (samples[id] = []);
          if (values.at(-1) !== width) values.push(width);
        }
      }
    }).observe(document, {
      childList: true,
      subtree: true,
      attributes: true,
      attributeFilter: ["width"],
    });
  });
  await mockDashboard(page, { rich: true });
  await page.goto("/dashboard");
  const drawnWindows = () =>
    page.evaluate(
      () =>
        Object.values(
          (
            window as unknown as {
              __practiceDrawWidths: Record<string, number[]>;
            }
          ).__practiceDrawWidths,
        ).filter(
          (widths) =>
            widths.length > 4 &&
            widths.some((width) => width > 0 && width < Math.max(...widths)),
        ).length,
    );
  await expect.poll(drawnWindows).toBeGreaterThanOrEqual(1);
  const before = await drawnWindows();
  await page.getByRole("combobox", { name: "Practice history window" }).click();
  await page
    .getByRole("option", { name: "Last 20 sessions", exact: true })
    .click();
  await expect.poll(drawnWindows).toBeGreaterThan(before);
});
