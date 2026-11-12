import { test, expect, type Page } from "@playwright/test";

// KAN-377: the Admin sees why a posting whose bank does not cover every metric cannot be approved, and a
// refusal from the server for the same reason is shown in the user's language.

const admin = {
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
const postingId = "33333333-3333-4333-8333-333333333333";
const recruiter = {
  id: "22222222-2222-4222-8222-222222222222",
  name: "Rec Ruiter",
  email: "rec@example.com",
};
const failing = {
  passes: false,
  assessed: true,
  covered: 4,
  total: 5,
  question_limit: 8,
  missing_metrics: [
    {
      id: "88888888-8888-4888-8888-888888888881",
      name: "Depth of Understanding",
      suggested_rubrics: [],
    },
  ],
  exceeds_limit: false,
  min_questions: 0,
};
const passing = { ...failing, passes: true, covered: 5, missing_metrics: [] };

async function mockModeration(
  page: Page,
  options: { covered: boolean; refuseApproval?: boolean },
) {
  const approvals: string[] = [];
  const posting = () => ({
    id: postingId,
    title: "Go Backend Engineer",
    status: "pending",
    reject_reason: null,
    recruiter,
    location: null,
    employment_type: null,
    salary_min: null,
    salary_max: null,
    interview_slot: null,
    interview_duration: null,
    required_avatar: null,
    required_voice: null,
    created_at: "2026-10-12T08:00:00Z",
    content_path: "https://files.example.com/jd/go-backend.pdf",
    document_url: null,
    document_expires_at: null,
    raw_text: null,
    refinement_note: "",
    review_submitted: true,
    core_question_count: 8,
    coverage: options.covered ? passing : failing,
  });
  await page.route("https://files.example.com/**", (route) =>
    route.fulfill({
      headers: { "content-type": "application/pdf" },
      body: "%PDF-1.4\n%%EOF\n",
    }),
  );
  await page.route("http://localhost:8080/**", async (route) => {
    const url = new URL(route.request().url());
    const method = route.request().method();
    const headers = {
      "access-control-allow-origin": "http://127.0.0.1:3100",
      "access-control-allow-credentials": "true",
      "access-control-allow-methods": "GET,POST,OPTIONS",
      "access-control-allow-headers": "Content-Type",
    };
    if (method === "OPTIONS") return route.fulfill({ status: 204, headers });
    if (url.pathname === "/auth/me")
      return route.fulfill({ headers, json: { success: true, data: admin } });
    if (url.pathname === `/admin/job-postings/${postingId}`)
      return route.fulfill({
        headers,
        json: { success: true, data: posting() },
      });
    if (
      url.pathname === `/admin/job-postings/${postingId}/approve` &&
      method === "POST"
    ) {
      approvals.push("approve");
      return route.fulfill({
        status: 409,
        headers,
        json: {
          success: false,
          error: {
            code: "METRIC_COVERAGE_INCOMPLETE",
            message: "backend english",
          },
        },
      });
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
  return { approvals };
}

test("approval is blocked with the missing metric named, and Reject stays available", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  const api = await mockModeration(page, { covered: false });
  await page.goto(`/admin/job-postings/${postingId}`);

  await expect(
    page.getByRole("heading", {
      name: "Metric coverage",
    }),
  ).toBeVisible();
  await expect(page.getByRole("meter")).toHaveAttribute("aria-valuenow", "4");
  await expect(page.locator('[data-coverage="missing"]')).toContainText(
    "Depth of Understanding",
  );
  const decision = page.getByRole("region", { name: "Decision" });
  await expect(
    decision.getByRole("button", { name: "Approve" }),
  ).toBeDisabled();
  await expect(
    decision.getByText("The question bank doesn't cover every metric yet."),
  ).toBeVisible();
  await expect(decision.getByRole("button", { name: "Reject" })).toBeEnabled();
  expect(api.approvals).toEqual([]);
  await page.screenshot({
    path: "test-results/admin-metric-coverage.png",
    fullPage: true,
  });
});

test("a refusal from the server for coverage is explained in the active language", async ({
  page,
}) => {
  await mockModeration(page, { covered: true });
  await page.goto(`/admin/job-postings/${postingId}`);

  await page
    .getByRole("region", { name: "Decision" })
    .getByRole("button", { name: "Approve" })
    .click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Approve posting" })
    .click();
  await expect(
    page.getByText(
      "This posting's questions don't cover every evaluation metric yet. Fix the coverage and try again.",
    ),
  ).toBeVisible();
  await expect(page.getByText("backend english")).toHaveCount(0);
});

test("the Admin's block reason reads in Vietnamese and fits a phone", async ({
  page,
  context,
}) => {
  await context.addCookies([
    { name: "NEXT_LOCALE", value: "vi", url: "http://127.0.0.1:3100" },
  ]);
  await page.setViewportSize({ width: 390, height: 844 });
  await mockModeration(page, { covered: false });
  await page.goto(`/admin/job-postings/${postingId}`);

  await expect(
    page.getByRole("heading", {
      name: "Độ phủ tiêu chí",
    }),
  ).toBeVisible();
  await expect(
    page.getByText("Bộ câu hỏi chưa phủ hết các tiêu chí."),
  ).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
});
