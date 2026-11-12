import { expect, test, type Page, type Route } from "@playwright/test";

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

const practiceDone = "aaaaaaaa-aaaa-4aaa-8aaa-000000000001";
const practicePending = "aaaaaaaa-aaaa-4aaa-8aaa-000000000002";
const practiceRunning = "aaaaaaaa-aaaa-4aaa-8aaa-000000000003";
const practiceFailed = "aaaaaaaa-aaaa-4aaa-8aaa-000000000004";
const recruitmentDone = "aaaaaaaa-aaaa-4aaa-8aaa-000000000005";
const missing = "aaaaaaaa-aaaa-4aaa-8aaa-000000000006";
const forbidden = "aaaaaaaa-aaaa-4aaa-8aaa-000000000007";

const started = "2026-10-10T08:00:00Z";
const finished = "2026-10-10T08:30:00Z";

// The candidate's view of GET /interviews/:id/report (KAN-336). A practice report explains its overall:
// question 1 is graded with a rubric (Communication 4/5 = 80, Technical depth 3/5 = 60), question 2 was not
// answered, so the session is partial and the overall is 0.6 x 80 + 0.4 x 60 = 72.00 (weights 60/40).
// Recruitment reports carry only the status: no score, summary, breakdown or questions.
const communication = "dddddddd-dddd-4ddd-8ddd-000000000001";
const depth = "dddddddd-dddd-4ddd-8ddd-000000000002";
function report(over: Record<string, unknown>) {
  return {
    interview_id: practiceDone,
    type: "practice",
    status: "completed",
    overall_score: 72,
    started_at: started,
    completed_at: finished,
    summary: "Clear answers with one gap in system design.",
    scoring: {
      method: "rubric",
      outcome: "partial",
      withheld_reason: null,
      assigned_questions: 2,
      graded_questions: 1,
      weight_total: 100,
      decimals: 2,
    },
    breakdown: [
      {
        metric_id: communication,
        name: "Communication",
        score: 80,
        weight: 60,
        question_count: 1,
      },
      {
        metric_id: depth,
        name: "Technical depth",
        score: 60,
        weight: 40,
        question_count: 1,
      },
    ],
    graded_questions: [
      {
        question_number: 1,
        status: "graded",
        rubric_name: "Diagnose and resolve a scenario",
        total: 70,
        earned: 7,
        max: 10,
        metrics: [
          {
            metric_id: communication,
            name: "Communication",
            earned: 4,
            max: 5,
            score: 80,
          },
          {
            metric_id: depth,
            name: "Technical depth",
            earned: 3,
            max: 5,
            score: 60,
          },
        ],
        criteria: [
          {
            criterion_id: "eeeeeeee-eeee-4eee-8eee-000000000001",
            name: "Identifies a correct cause and fix",
            metric_id: depth,
            metric_name: "Technical depth",
            points: 3,
            max_points: 5,
            evidence: "A race in the cache invalidation.",
            evidence_position: 1,
            feedback: "Names the race but not how to prevent it.",
          },
          {
            criterion_id: "eeeeeeee-eeee-4eee-8eee-000000000002",
            name: "Explains the diagnosis clearly",
            metric_id: communication,
            metric_name: "Communication",
            points: 4,
            max_points: 5,
            evidence: "cache invalidation",
            evidence_position: 1,
            feedback: "Clear and short.",
          },
        ],
      },
      {
        question_number: 2,
        status: "not_answered",
        rubric_name: null,
        total: null,
        earned: 0,
        max: 0,
        metrics: [],
        criteria: [],
      },
    ],
    questions: [
      {
        position: 1,
        question_number: 1,
        kind: "core",
        question: "Describe a hard bug you fixed.",
        answer: "A race in the cache invalidation.",
        feedback: "Good root-cause analysis.",
      },
      {
        position: 2,
        question_number: 2,
        kind: "core",
        question: "How would you scale a queue?",
        answer: null,
        feedback: null,
      },
    ],
    ...over,
  };
}

type Responses = Record<string, { status: number; body: unknown }[]>;

// Serves one report per interview id. A response list is consumed in order, so a test can show a
// pending report and then a finished one after "Check again".
async function mockReports(page: Page, responses: Responses) {
  const calls: Record<string, number> = {};
  await page.route("http://localhost:8080/**", async (route: Route) => {
    const request = route.request();
    const url = new URL(request.url());
    if (request.method() === "OPTIONS")
      return route.fulfill({ status: 204, headers });
    if (url.pathname === "/auth/me")
      return route.fulfill({
        headers,
        json: { success: true, data: candidate },
      });
    const match = url.pathname.match(/^\/interviews\/([^/]+)\/report$/);
    if (match && request.method() === "GET") {
      const id = match[1];
      const list = responses[id] ?? [];
      const index = Math.min(calls[id] ?? 0, list.length - 1);
      calls[id] = (calls[id] ?? 0) + 1;
      const next = list[index] ?? {
        status: 404,
        body: { code: "INTERVIEW_NOT_FOUND", message: "not found" },
      };
      return route.fulfill({
        status: next.status,
        headers,
        json:
          next.status === 200
            ? { success: true, data: next.body }
            : { success: false, error: next.body },
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
  return { calls };
}

test("a completed practice report shows the score, the breakdown and each question's feedback", async ({
  page,
}) => {
  await mockReports(page, {
    [practiceDone]: [{ status: 200, body: report({}) }],
  });
  await page.goto(`/reports/${practiceDone}`);

  await expect(
    page.getByRole("heading", { level: 1, name: "Practice interview" }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Overall score" }),
  ).toBeVisible();
  await expect(page.getByTestId("overall-score")).toHaveText("72.00 / 100");
  await expect(page.getByTestId("scoring-outcome")).toHaveText("Partial");
  await expect(page.getByTestId("overall-formula")).toHaveText(
    "(80.00 × 60 + 60.00 × 40) ÷ 100 = 72.00",
  );
  await expect(
    page.getByText("Clear answers with one gap in system design."),
  ).toBeVisible();
  await expect(
    page.getByRole("progressbar", { name: "Communication" }),
  ).toHaveAttribute("aria-valuenow", "80");
  await expect(
    page.getByRole("progressbar", { name: "Technical depth" }),
  ).toHaveAttribute("aria-valuenow", "60");
  await expect(page.getByTestId("metric-list")).toContainText("Weight 60.0%");
  await expect(
    page.getByText(/Question totals are for reference only/),
  ).toBeVisible();
  // The criteria of question 1 open from the keyboard (native disclosure), with points, quote and why.
  const grading = page.getByTestId("question-grading");
  await expect(grading).toContainText("7 / 10 points");
  await expect(grading).toContainText("Question total 70.00 (reference)");
  await grading.locator("summary").focus();
  await page.keyboard.press("Enter");
  await expect(
    grading.getByText("Identifies a correct cause and fix"),
  ).toBeVisible();
  await expect(grading).toContainText("3 / 5 points");
  await expect(grading).toContainText("Technical depth: 3/5 = 60.00");
  await expect(
    grading.getByText("Names the race but not how to prevent it."),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: /Describe a hard bug you fixed/ }),
  ).toBeVisible();
  await expect(page.getByText("Good root-cause analysis.")).toBeVisible();
  await expect(page.getByText("No answer recorded.")).toBeVisible();
});

test("a pending practice report says it is being evaluated and Check again shows the finished result", async ({
  page,
}) => {
  const { calls } = await mockReports(page, {
    [practicePending]: [
      {
        status: 200,
        body: report({
          interview_id: practicePending,
          status: "pending",
          overall_score: null,
          completed_at: finished,
          summary: null,
          scoring: null,
          breakdown: null,
          questions: null,
          graded_questions: null,
        }),
      },
      {
        status: 200,
        body: report({ interview_id: practicePending }),
      },
    ],
  });
  await page.goto(`/reports/${practicePending}`);

  await expect(
    page.getByRole("heading", { name: "Your interview is being evaluated" }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Overall score" }),
  ).toHaveCount(0);
  await page.getByRole("button", { name: "Check again" }).click();

  await expect(
    page.getByRole("heading", { name: "Overall score" }),
  ).toBeVisible();
  await expect(page.getByTestId("overall-score")).toHaveText("72.00 / 100");
  await expect.poll(() => calls[practicePending]).toBeGreaterThanOrEqual(2);
});

test("an interview still in progress has no score and no report content", async ({
  page,
}) => {
  await mockReports(page, {
    [practiceRunning]: [
      {
        status: 200,
        body: report({
          interview_id: practiceRunning,
          status: "in_progress",
          overall_score: null,
          completed_at: null,
          summary: null,
          scoring: null,
          breakdown: null,
          graded_questions: null,
          questions: null,
        }),
      },
    ],
  });
  await page.goto(`/reports/${practiceRunning}`);

  await expect(
    page.getByRole("heading", { name: "This interview isn't finished yet" }),
  ).toBeVisible();
  await expect(page.getByText("In progress", { exact: true })).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Overall score" }),
  ).toHaveCount(0);
  await expect(page.getByRole("heading", { name: "Questions" })).toHaveCount(0);
});

test("a failed evaluation says no scores were saved instead of showing a score", async ({
  page,
}) => {
  await mockReports(page, {
    [practiceFailed]: [
      {
        status: 200,
        body: report({
          interview_id: practiceFailed,
          status: "failed",
          overall_score: null,
          summary: null,
          scoring: null,
          breakdown: null,
          graded_questions: null,
          questions: null,
        }),
      },
    ],
  });
  await page.goto(`/reports/${practiceFailed}`);

  await expect(
    page.getByRole("heading", { name: "We couldn't evaluate this interview" }),
  ).toBeVisible();
  await expect(
    page.getByText("Evaluation failed", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Overall score" }),
  ).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Check again" })).toHaveCount(
    0,
  );
});

test("a recruitment report shows only the status, with no score, transcript or recording", async ({
  page,
}) => {
  await mockReports(page, {
    [recruitmentDone]: [
      {
        status: 200,
        body: report({
          interview_id: recruitmentDone,
          type: "recruitment",
          overall_score: undefined,
          summary: null,
          scoring: null,
          breakdown: null,
          questions: null,
          graded_questions: null,
        }),
      },
    ],
  });
  await page.goto(`/reports/${recruitmentDone}`);

  await expect(
    page.getByRole("heading", { level: 1, name: "Recruitment interview" }),
  ).toBeVisible();
  await expect(page.getByText("Completed", { exact: true })).toBeVisible();
  // The Recruiter announces a recruitment result: the Candidate sees no score at all.
  await expect(page.getByText(/\/ 100/)).toHaveCount(0);
  await expect(page.getByTestId("scoring-summary")).toHaveCount(0);
  await expect(
    page.getByText(
      "Detailed feedback for recruitment interviews is not shown here.",
    ),
  ).toBeVisible();
  await expect(page.getByRole("progressbar")).toHaveCount(0);
  await expect(page.getByRole("heading", { name: "Breakdown" })).toHaveCount(0);
  await expect(page.getByRole("heading", { name: "Questions" })).toHaveCount(0);
  await expect(page.getByText("Answer:", { exact: false })).toHaveCount(0);
  await expect(page.getByText(/transcript|recording|video|audio/i)).toHaveCount(
    0,
  );
});

test("a report that does not exist for this candidate shows the not-found state", async ({
  page,
}) => {
  await mockReports(page, {});
  await page.goto(`/reports/${missing}`);

  await expect(
    page.getByRole("heading", { name: "We couldn't find this report" }),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Open interview history" }),
  ).toHaveAttribute("href", "/history");
  await expect(
    page.getByRole("heading", { name: "Overall score" }),
  ).toHaveCount(0);
});

test("a report the candidate may not read shows the access-denied state", async ({
  page,
}) => {
  await mockReports(page, {
    [forbidden]: [
      {
        status: 403,
        body: { code: "FORBIDDEN", message: "forbidden" },
      },
    ],
  });
  await page.goto(`/reports/${forbidden}`);

  await expect(
    page.getByRole("heading", {
      name: "You don't have access to this report",
    }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Overall score" }),
  ).toHaveCount(0);
});

test("in Vietnamese the report chrome is translated while the AI feedback stays as stored", async ({
  page,
}) => {
  await page
    .context()
    .addCookies([
      { name: "NEXT_LOCALE", value: "vi", url: "http://127.0.0.1:3100" },
    ]);
  await mockReports(page, {
    [practiceDone]: [{ status: 200, body: report({}) }],
  });
  await page.goto(`/reports/${practiceDone}`);

  await expect(
    page.getByRole("heading", { level: 1, name: "Phỏng vấn luyện tập" }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Điểm tổng thể" }),
  ).toBeVisible();
  await expect(page.getByTestId("overall-score")).toHaveText("72,00 / 100");
  await expect(page.getByTestId("scoring-outcome")).toHaveText("Một phần");
  await expect(page.getByText(/Điểm từng câu chỉ để tham khảo/)).toBeVisible();
  await expect(
    page.getByText("Clear answers with one gap in system design."),
  ).toBeVisible();
  await expect(page.getByText("Good root-cause analysis.")).toBeVisible();
  await expect(page.getByText("Chưa ghi nhận câu trả lời.")).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Quay lại lịch sử phỏng vấn" }).first(),
  ).toBeVisible();
});

test("PDF export is offered only for a completed own practice report", async ({
  page,
}) => {
  await mockReports(page, {
    [practiceDone]: [{ status: 200, body: report({}) }],
    [recruitmentDone]: [
      {
        status: 200,
        body: report({
          type: "recruitment",
          overall_score: undefined,
          summary: null,
          scoring: null,
          breakdown: null,
          questions: null,
          graded_questions: null,
        }),
      },
    ],
    [practicePending]: [
      {
        status: 200,
        body: report({
          status: "pending",
          overall_score: null,
          scoring: null,
          breakdown: null,
          graded_questions: null,
        }),
      },
    ],
    [practiceFailed]: [
      {
        status: 200,
        body: report({
          status: "failed",
          overall_score: null,
          scoring: null,
          breakdown: null,
          graded_questions: null,
        }),
      },
    ],
  });
  await page.goto(`/reports/${practiceDone}`);
  await expect(
    page.getByRole("button", { name: "Export PDF", exact: true }),
  ).toBeVisible();
  for (const id of [
    recruitmentDone,
    practicePending,
    practiceFailed,
    missing,
    forbidden,
  ]) {
    await page.goto(`/reports/${id}`);
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Export PDF", exact: true }),
    ).toHaveCount(0);
  }
});

test("PDF rechecks access and prints only the authorized report", async ({
  page,
}) => {
  await page.context().addInitScript(() => {
    window.print = () => {};
    // CDP printToPDF also fires afterprint; keep the target alive until the PDF bytes return.
    window.close = () => {};
  });
  const { calls } = await mockReports(page, {
    [practiceDone]: [{ status: 200, body: report({}) }],
  });
  await page.goto(`/reports/${practiceDone}`);
  const popupPromise = page.waitForEvent("popup");
  await page.getByRole("button", { name: "Export PDF", exact: true }).click();
  const popup = await popupPromise;
  await expect(
    popup.getByRole("heading", { name: "RoleCue — Practice interview" }),
  ).toBeVisible();
  await expect(popup.getByText("Good root-cause analysis.")).toBeVisible();
  await expect(
    popup.getByText("(80.00 × 60 + 60.00 × 40) ÷ 100 = 72.00"),
  ).toBeVisible();
  await expect(
    popup.getByText(
      /Identifies a correct cause and fix \(Technical depth\): 3 \/ 5 points/,
    ),
  ).toBeVisible();
  // The question's score per metric (the step between the criteria and the session metric) is exported too.
  await expect(
    popup.getByText(
      /Metrics in this question: .*Technical depth: 3\/5 = 60\.00/,
    ),
  ).toBeVisible();
  expect(calls[practiceDone]).toBe(2);
  await popup.evaluate(() => document.fonts.ready);
  await popup.pdf({
    path: test.info().outputPath("practice-report.pdf"),
    format: "A4",
    printBackground: true,
  });
  await popup.close();
});

test("PDF refuses access revoked after the report was loaded", async ({
  page,
}) => {
  await mockReports(page, {
    [practiceDone]: [
      { status: 200, body: report({}) },
      {
        status: 404,
        body: { code: "INTERVIEW_NOT_FOUND", message: "not found" },
      },
    ],
  });
  await page.goto(`/reports/${practiceDone}`);
  await page.getByRole("button", { name: "Export PDF", exact: true }).click();
  await expect(
    page.getByRole("alert").filter({ hasText: "Unable to export." }),
  ).toBeVisible();
});

test("Vietnamese PDF embeds the font and preserves English feedback", async ({
  page,
}) => {
  await page
    .context()
    .addCookies([
      { name: "NEXT_LOCALE", value: "vi", url: "http://127.0.0.1:3100" },
    ]);
  await page.context().addInitScript(() => {
    window.print = () => {};
    // CDP printToPDF also fires afterprint; keep the target alive until the PDF bytes return.
    window.close = () => {};
  });
  await mockReports(page, {
    [practiceDone]: [{ status: 200, body: report({}) }],
  });
  await page.goto(`/reports/${practiceDone}`);
  const popupPromise = page.waitForEvent("popup");
  await page.getByRole("button", { name: "Xuất PDF", exact: true }).click();
  const popup = await popupPromise;
  await expect(popup.locator("html")).toHaveAttribute("lang", "vi");
  await expect(popup.getByText("Good root-cause analysis.")).toBeVisible();
  await popup.evaluate(() => document.fonts.ready);
  expect(await popup.evaluate(() => document.fonts.check("11pt Report"))).toBe(
    true,
  );
  await popup.pdf({
    path: test.info().outputPath("practice-report-vi.pdf"),
    format: "A4",
  });
  await popup.close();
});

const practiceWithheld = "aaaaaaaa-aaaa-4aaa-8aaa-000000000008";

// KAN-337: a withheld overall says why and never shows a made-up number; the layout holds on a phone.
for (const locale of ["en", "vi"] as const) {
  test(`a withheld overall explains the missing metric on a phone (${locale})`, async ({
    page,
  }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    if (locale === "vi")
      await page
        .context()
        .addCookies([
          { name: "NEXT_LOCALE", value: "vi", url: "http://127.0.0.1:3100" },
        ]);
    const base = report({});
    await mockReports(page, {
      [practiceWithheld]: [
        {
          status: 200,
          body: {
            ...base,
            interview_id: practiceWithheld,
            overall_score: null,
            scoring: {
              ...base.scoring,
              outcome: "withheld",
              withheld_reason: "missing_metric_coverage",
            },
            breakdown: [
              base.breakdown[0],
              base.breakdown[1],
              {
                metric_id: "dddddddd-dddd-4ddd-8ddd-000000000003",
                name: "Answer Relevance",
                score: null,
                weight: 20,
                question_count: 0,
              },
            ],
          },
        },
      ],
    });
    await page.goto(`/reports/${practiceWithheld}`);
    await expect(page.getByTestId("overall-withheld")).toHaveText(
      locale === "en" ? "No overall score" : "Chưa có điểm tổng",
    );
    await expect(page.getByText(/Answer Relevance/).first()).toBeVisible();
    await expect(page.getByTestId("overall-score")).toHaveCount(0);
    await expect(page.getByTestId("overall-formula")).toHaveCount(0);
    await expect(page.getByTestId("metric-list")).toContainText(
      locale === "en" ? "No evidence" : "Chưa có bằng chứng",
    );
    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth - window.innerWidth,
    );
    expect(overflow).toBeLessThanOrEqual(0);
    await page.screenshot({
      path: test.info().outputPath(`report-withheld-mobile-${locale}.png`),
      fullPage: true,
    });
  });
}
