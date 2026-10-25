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
};

// Serves the four dashboard reads from the Go API shapes. A failing endpoint answers 500 for the
// configured number of requests, then recovers, so a test can check Try again.
async function mockDashboard(page: Page, options: Options = {}) {
  const hits: Record<string, number> = {};
  const fail = (key: Failing) => {
    hits[key] = (hits[key] ?? 0) + 1;
    const budget = options.failing?.[key];
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
    if (url.pathname === "/role-profiles" && method === "GET")
      return fail("roleProfiles")
        ? broken()
        : ok({
            items: [
              {
                id: roleProfileId,
                title: "Frontend Developer",
                seniorityLevel: "mid",
                status: "review",
                createdAt: "2026-10-01T08:00:00Z",
              },
            ],
            pagination: { limit: 3, offset: 0, total: 1 },
          });
    if (url.pathname === "/interviews" && method === "GET")
      return fail("practice")
        ? broken()
        : ok({
            items: [
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
            total: 1,
            page: 1,
            page_size: 3,
          });
    if (url.pathname === "/applications" && method === "GET")
      return fail("applications")
        ? broken()
        : ok({
            items: [
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
            total: 1,
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
  return { hits };
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
  await expect(practice).toContainText("82/100");

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
}) => {
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
  ).toContainText("82/100");
  await expect(
    page.getByRole("region", { name: "Coin balance" }),
  ).toContainText("250 coins");
});

test("Try again reloads a failed card once the request recovers", async ({
  page,
}) => {
  const { hits } = await mockDashboard(page, { failing: { wallet: 1 } });
  await page.goto("/dashboard");

  const coins = page.getByRole("region", { name: "Coin balance" });
  await expect(
    coins.getByText("We couldn't load your coin balance."),
  ).toBeVisible();

  await coins.getByRole("button", { name: "Try again" }).click();

  await expect(coins).toContainText("250 coins");
  await expect(
    coins.getByText("We couldn't load your coin balance."),
  ).toHaveCount(0);
  expect(hits.wallet).toBeGreaterThanOrEqual(2);
});
