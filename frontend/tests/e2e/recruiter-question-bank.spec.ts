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
const postingId = "33333333-3333-4333-8333-333333333333";
const bankPath = `/job-postings/${postingId}/questions`;

async function mockBank(
  page: Page,
  options: {
    editable?: boolean;
    empty?: boolean;
    failGenerationOnce?: boolean;
  } = {},
) {
  const editable = options.editable ?? true;
  let questions: { id: string; content: string }[] = options.empty
    ? []
    : [
        {
          id: "44444444-4444-4444-8444-444444444401",
          content: "Explain Go channels.",
        },
        {
          id: "44444444-4444-4444-8444-444444444402",
          content: "How do you index a table?",
        },
      ];
  let submitted = false;
  let submissions = 0;
  let generationAttempts = 0;
  let next = 1;
  const bodies: unknown[] = [];
  const id = () =>
    `44444444-4444-4444-8444-${String(next++).padStart(12, "0")}`;
  const bank = () => ({
    questions,
    posting_status: editable ? "pending" : "open",
    // Like the server: an empty bank is generated, not edited.
    editable: editable && questions.length > 0,
    max_question_length: 1000,
    generation_size: 2,
    review_submitted: submitted,
  });
  const posting = () => ({
    id: postingId,
    title: "Backend Engineer",
    status: editable ? "pending" : "open",
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
  await page.route("http://localhost:8080/**", async (route) => {
    const url = new URL(route.request().url());
    const method = route.request().method();
    const headers = {
      "access-control-allow-origin": "http://127.0.0.1:3100",
      "access-control-allow-credentials": "true",
      "access-control-allow-methods": "GET,POST,PATCH,DELETE,OPTIONS",
      "access-control-allow-headers": "Content-Type",
    };
    if (method === "OPTIONS") return route.fulfill({ status: 204, headers });
    if (url.pathname === "/auth/me")
      return route.fulfill({
        headers,
        json: { success: true, data: recruiter },
      });
    if (url.pathname === bankPath && method === "GET")
      return route.fulfill({ headers, json: { success: true, data: bank() } });
    if (url.pathname === `/job-postings/${postingId}` && method === "GET")
      return route.fulfill({
        headers,
        json: { success: true, data: posting() },
      });
    if (
      url.pathname === `/job-postings/${postingId}/evaluation-weights` &&
      method === "GET"
    )
      return route.fulfill({
        headers,
        json: {
          success: true,
          data: {
            configured: false,
            using_defaults: true,
            editable,
            posting_status: editable ? "pending" : "open",
            total: 100,
            weights: [
              {
                metric_id: "77777777-7777-4777-8777-777777777777",
                name: "Technical correctness",
                percentage: 100,
              },
            ],
          },
        },
      });
    if (
      url.pathname === `/job-postings/${postingId}/submit` &&
      method === "POST"
    ) {
      submissions += 1;
      submitted = true;
      return route.fulfill({
        headers,
        json: { success: true, data: posting() },
      });
    }
    if (url.pathname === `${bankPath}/generate` && method === "POST") {
      generationAttempts++;
      if (options.failGenerationOnce && generationAttempts === 1)
        return route.fulfill({
          status: 502,
          headers,
          json: {
            success: false,
            error: {
              code: "QUESTION_GENERATION_FAILED",
              message: "Generation failed. Please retry.",
            },
          },
        });
      questions = [
        { id: id(), content: "Explain Go channels." },
        { id: id(), content: "How do you index a table?" },
      ];
      return route.fulfill({
        status: 201,
        headers,
        json: { success: true, data: { questions } },
      });
    }
    if (url.pathname === bankPath && method === "POST") {
      const body = route.request().postDataJSON() as { content: string };
      bodies.push(body);
      const question = { id: id(), content: body.content };
      questions = [...questions, question];
      return route.fulfill({
        status: 201,
        headers,
        json: { success: true, data: question },
      });
    }
    const match = url.pathname.match(new RegExp(`^${bankPath}/(.+)$`));
    if (match && method === "DELETE") {
      questions = questions.filter((q) => q.id !== match[1]);
      if (questions.length === 0) submitted = false;
      return route.fulfill({ status: 204, headers });
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
  return { bodies, submissions: () => submissions };
}

test("recruiter reviews generated questions, edits and explicitly submits", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.stack ?? error.message));
  await page.setViewportSize({ width: 1440, height: 1000 });
  const api = await mockBank(page);
  await page.goto(`/recruiter/job-postings/${postingId}/questions`);

  await expect(
    page.getByRole("heading", { level: 1, name: "Question bank" }),
  ).toBeVisible();
  await expect(
    page.getByText("Explain Go channels.", { exact: true }),
  ).toBeVisible();
  await expect(
    page
      .getByRole("region", { name: "Questions" })
      .getByText("2 questions", { exact: true }),
  ).toBeVisible();

  const form = page.getByRole("form", { name: "Add a question" });
  await form.getByLabel("Add a question").fill("What is a goroutine leak?");
  await expect(
    page.getByRole("button", { name: "Submit for approval" }),
  ).toBeDisabled();
  await form.getByRole("button", { name: "Add question" }).click();
  await expect(
    page.getByRole("region", { name: "Questions" }).getByText("3 questions"),
  ).toBeVisible();
  expect(api.bodies).toEqual([{ content: "What is a goroutine leak?" }]);

  await page.getByRole("button", { name: "Delete question 1" }).click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Delete question" })
    .click();
  await expect(
    page.getByText("Explain Go channels.", { exact: true }),
  ).toBeHidden();
  await expect(
    page
      .getByRole("region", { name: "Questions" })
      .getByText("2 questions", { exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Submit for approval" }).click();
  const review = page.getByRole("dialog", { name: "Review & submit" });
  await expect(
    review.getByText("Backend Engineer", { exact: true }),
  ).toBeVisible();
  await expect(
    review.getByText("Default criteria · equal weights"),
  ).toBeVisible();
  expect(api.submissions()).toBe(0);
  await review.getByRole("button", { name: "Submit for approval" }).click();
  await expect(page.getByText("Submitted for approval.")).toBeVisible();
  expect(api.submissions()).toBe(1);
  expect(errors).toEqual([]);
  await page.screenshot({
    path: "test-results/recruiter-question-bank-desktop.png",
    fullPage: true,
  });
});

test("a posting that left review shows a read-only bank", async ({ page }) => {
  await mockBank(page, { editable: false });
  await page.goto(`/recruiter/job-postings/${postingId}/questions`);
  await expect(
    page.getByText(/so the question bank is read-only/),
  ).toBeVisible();
  await expect(page.getByRole("form", { name: "Add a question" })).toHaveCount(
    0,
  );
});

test("empty bank blocks submission and generation can be retried on mobile", async ({
  page,
}) => {
  await page.setViewportSize({ width: 320, height: 780 });
  await mockBank(page, { empty: true, failGenerationOnce: true });
  await page.goto(`/recruiter/job-postings/${postingId}/questions`);
  await expect(
    page.getByRole("button", { name: "Submit for approval" }),
  ).toBeDisabled();
  await page.getByRole("button", { name: "Generate questions" }).click();
  await expect(
    page
      .getByRole("alert")
      .filter({ hasText: "Generation failed. Please retry." }),
  ).toContainText("Generation failed. Please retry.");
  await page
    .getByRole("button", { name: "Retry generating questions" })
    .click();
  await expect(
    page.getByText("Explain Go channels.", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Submit for approval" }),
  ).toBeEnabled();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: "test-results/recruiter-question-bank-mobile.png",
    fullPage: true,
  });
});
