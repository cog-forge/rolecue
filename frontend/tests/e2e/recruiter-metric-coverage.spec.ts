import { test, expect, type Page } from "@playwright/test";

// KAN-377: the bank screen warns about metrics no question measures, keeps Submit off until a change covers
// them, and reads the verdict from the server after every change.

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
const postingId = "33333333-3333-4333-8333-333333333333";
const bankPath = `/job-postings/${postingId}/questions`;
const weightsPath = `/job-postings/${postingId}/evaluation-weights`;
const depth = "88888888-8888-4888-8888-888888888881";
const accuracy = "88888888-8888-4888-8888-888888888882";
const grading = {
  rubric_id: null,
  expected_answer: "Key points.",
  grading_incomplete: false,
  grading_issues: [],
};
const failing = {
  passes: false,
  assessed: true,
  covered: 4,
  total: 5,
  question_limit: 8,
  missing_metrics: [
    {
      id: depth,
      name: "Depth of Understanding",
      suggested_rubrics: [
        {
          id: "99999999-9999-4999-8999-999999999991",
          name: "Explain a mechanism",
        },
      ],
    },
  ],
  exceeds_limit: false,
  min_questions: 0,
};
const passing = { ...failing, passes: true, covered: 5, missing_metrics: [] };

async function mockCoverage(
  page: Page,
  options: { covered?: boolean; staleSubmit?: boolean } = {},
) {
  // `covered` is the server's verdict; an edit of a question fixes it, a weight change on the metric the
  // bank does not measure breaks it again.
  let covered = options.covered ?? false;
  let submitted = false;
  let submissions = 0;
  const questions = [
    {
      id: "44444444-4444-4444-8444-444444444401",
      content: "Explain Go channels.",
    },
    {
      id: "44444444-4444-4444-8444-444444444402",
      content: "How do you index a table?",
    },
  ];
  const coverage = () => (covered ? passing : failing);
  const revision = (content: string) =>
    content.length.toString(16).padStart(64, "a");
  const bank = () => ({
    questions: questions.map((q) => ({
      ...grading,
      ...q,
      revision: revision(q.content),
    })),
    grading_ready: true,
    grading_incomplete_count: 0,
    posting_status: "pending",
    editable: true,
    max_question_length: 1000,
    generation_size: 2,
    review_submitted: submitted,
    coverage: coverage(),
  });
  const posting = () => ({
    id: postingId,
    title: "Backend Engineer",
    status: "pending",
    reject_reason: null,
    raw_text: "Technical backend role",
    has_document: true,
    seniority_level: "senior",
    requirements: [],
    refinement_note: "",
    review_submitted: submitted,
    location: null,
    employment_type: null,
    salary_min: null,
    salary_max: null,
    interview_duration: null,
    interview_slot: null,
    required_avatar: null,
    required_voice: null,
    core_question_count: questions.length,
    created_at: "2026-10-12T08:00:00Z",
  });
  const weights = () => ({
    configured: false,
    using_defaults: true,
    editable: true,
    posting_status: "pending",
    total: 100,
    weights: [
      { metric_id: accuracy, name: "Technical correctness", percentage: 50 },
      { metric_id: depth, name: "Depth of Understanding", percentage: 50 },
    ],
    coverage: coverage(),
  });
  await page.route("http://localhost:8080/**", async (route) => {
    const url = new URL(route.request().url());
    const method = route.request().method();
    const headers = {
      "access-control-allow-origin": "http://127.0.0.1:3100",
      "access-control-allow-credentials": "true",
      "access-control-allow-methods": "GET,POST,PATCH,PUT,DELETE,OPTIONS",
      "access-control-allow-headers": "Content-Type",
    };
    const ok = (data: unknown) =>
      route.fulfill({ headers, json: { success: true, data } });
    if (method === "OPTIONS") return route.fulfill({ status: 204, headers });
    if (url.pathname === "/auth/me") return ok(recruiter);
    if (url.pathname === bankPath && method === "GET") return ok(bank());
    if (url.pathname === `/job-postings/${postingId}` && method === "GET")
      return ok(posting());
    if (url.pathname === weightsPath && method === "GET") return ok(weights());
    if (url.pathname === "/evaluation-metrics")
      return ok({
        items: [
          { id: accuracy, name: "Technical correctness" },
          { id: depth, name: "Depth of Understanding" },
        ],
      });
    if (url.pathname === weightsPath && method === "PUT") {
      // A change that uncovers the bank takes a submitted posting out of the Admin queue (KAN-377).
      covered = false;
      return ok({ ...weights(), submission_withdrawn: true });
    }
    const match = url.pathname.match(new RegExp(`^${bankPath}/(.+)$`));
    if (match && method === "PATCH") {
      const body = route.request().postDataJSON() as { content: string };
      const question = questions.find((q) => q.id === match[1]);
      if (question) question.content = body.content;
      covered = true;
      submitted = false;
      return ok({
        ...grading,
        id: match[1],
        content: body.content,
        revision: revision(body.content),
        coverage: coverage(),
      });
    }
    if (
      url.pathname === `/job-postings/${postingId}/submit` &&
      method === "POST"
    ) {
      if (!covered || options.staleSubmit)
        return route.fulfill({
          status: 409,
          headers,
          json: {
            success: false,
            error: {
              code: "METRIC_COVERAGE_INCOMPLETE",
              message: "the question bank must cover every active metric",
            },
          },
        });
      submissions += 1;
      submitted = true;
      return ok(posting());
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
  return { submissions: () => submissions };
}

test("the warning names the missing metric, goes away after a fix, and then Submit works", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.stack ?? error.message));
  await page.setViewportSize({ width: 1440, height: 1000 });
  const api = await mockCoverage(page);
  await page.goto(`/recruiter/job-postings/${postingId}/questions`);

  await expect(
    page.getByRole("heading", {
      name: "Metric coverage",
    }),
  ).toBeVisible();
  await expect(page.getByRole("meter")).toHaveAttribute("aria-valuenow", "4");
  await expect(page.locator('[data-coverage="missing"]')).toContainText(
    "Depth of Understanding",
  );
  await expect(
    page.getByText(
      "Assign a rubric that covers it, for example Explain a mechanism.",
    ),
  ).toBeVisible();
  const submit = page.getByRole("button", { name: "Submit for approval" });
  await expect(submit).toBeDisabled();
  await expect(page.getByText("Cover every metric to submit.")).toBeVisible();
  await page.screenshot({
    path: "test-results/recruiter-metric-coverage-warning.png",
    fullPage: true,
  });

  await page.getByRole("button", { name: "Edit question 1" }).click();
  await page
    .getByRole("textbox", { name: "Edit question 1" })
    .fill("Explain buffered channels.");
  await page.getByRole("button", { name: "Save question" }).click();

  await expect(page.locator('[data-coverage="missing"]')).toBeHidden();
  await expect(page.getByRole("meter")).toHaveAttribute("aria-valuenow", "5");
  await expect(submit).toBeEnabled();
  await submit.click();
  const review = page.getByRole("dialog", { name: "Review & submit" });
  await expect(review.getByText("All metrics covered")).toBeVisible();
  await review.getByRole("button", { name: "Submit for approval" }).click();
  await expect(page.getByText("Submitted for approval.")).toBeVisible();
  expect(api.submissions()).toBe(1);
  expect(errors).toEqual([]);
});

test("a weight change on the evaluation page shows the coverage it leaves", async ({
  page,
}) => {
  await mockCoverage(page, { covered: true });
  await page.goto(`/recruiter/job-postings/${postingId}/evaluation`);

  await expect(page.getByText("All metrics covered")).toBeVisible();
  await page.getByLabel("Depth of Understanding").fill("60");
  await page.getByLabel("Technical correctness").fill("40");
  await page.getByRole("button", { name: "Save criteria" }).click();

  await expect(
    page.getByRole("heading", {
      name: "Metric coverage",
    }),
  ).toBeVisible();
  await expect(page.getByRole("meter")).toHaveAttribute("aria-valuenow", "4");
  await expect(page.locator('[data-coverage="missing"]')).toContainText(
    "Depth of Understanding",
  );
  await expect(
    page.getByText(
      "Your change left the question bank unable to cover every evaluation metric, so the posting left the review queue. Fix the questions or the weights, then submit it again.",
    ),
  ).toBeVisible();
});

test("a submission the server refuses for coverage shows the translated reason", async ({
  page,
}) => {
  await mockCoverage(page, { covered: true, staleSubmit: true });
  await page.goto(`/recruiter/job-postings/${postingId}/questions`);

  await page.getByRole("button", { name: "Submit for approval" }).click();
  await page
    .getByRole("dialog", { name: "Review & submit" })
    .getByRole("button", { name: "Submit for approval" })
    .click();
  await expect(
    page.getByText(
      "This posting's questions don't cover every evaluation metric yet. Fix the coverage and try again.",
    ),
  ).toBeVisible();
  await expect(
    page.getByText("the question bank must cover every active metric"),
  ).toHaveCount(0);
});

test("the warning reads in Vietnamese", async ({ page, context }) => {
  await context.addCookies([
    { name: "NEXT_LOCALE", value: "vi", url: "http://127.0.0.1:3100" },
  ]);
  await mockCoverage(page);
  await page.goto(`/recruiter/job-postings/${postingId}/questions`);

  await expect(
    page.getByRole("heading", {
      name: "Độ phủ tiêu chí",
    }),
  ).toBeVisible();
  await expect(page.getByText("Chưa có câu đo:")).toBeVisible();
  await expect(
    page.getByText("Hãy phủ đủ tiêu chí để gửi duyệt."),
  ).toBeVisible();
});

test("the warning fits a 320px screen", async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 780 });
  await mockCoverage(page);
  await page.goto(`/recruiter/job-postings/${postingId}/questions`);

  await expect(
    page.getByRole("heading", { name: /Metric coverage/ }),
  ).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: "test-results/recruiter-metric-coverage-mobile.png",
    fullPage: true,
  });
});

test("the withdrawal notice on the evaluation page reads in Vietnamese", async ({
  page,
  context,
}) => {
  await context.addCookies([
    { name: "NEXT_LOCALE", value: "vi", url: "http://127.0.0.1:3100" },
  ]);
  await mockCoverage(page, { covered: true });
  await page.goto(`/recruiter/job-postings/${postingId}/evaluation`);
  await page.getByLabel("Depth of Understanding").fill("60");
  await page.getByLabel("Technical correctness").fill("40");
  await page.getByRole("button", { name: "Lưu tiêu chí" }).click();

  await expect(
    page.getByText(
      "Thay đổi của bạn khiến bộ câu hỏi không còn phủ hết các tiêu chí đánh giá nên tin đã rời hàng chờ duyệt. Hãy chỉnh câu hỏi hoặc trọng số rồi gửi duyệt lại.",
    ),
  ).toBeVisible();
});
