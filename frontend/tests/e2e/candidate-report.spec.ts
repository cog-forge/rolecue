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

// The candidate's view of GET /interviews/:id/report. Recruitment reports carry only the score and
// status: summary, breakdown and questions are null.
function report(over: Record<string, unknown>) {
  return {
    interview_id: practiceDone,
    type: "practice",
    status: "completed",
    overall_score: 82,
    started_at: started,
    completed_at: finished,
    summary: "Clear answers with one gap in system design.",
    breakdown: [
      {
        metric_id: "dddddddd-dddd-4ddd-8ddd-000000000001",
        name: "Communication",
        score: 80,
      },
      {
        metric_id: "dddddddd-dddd-4ddd-8ddd-000000000002",
        name: "Technical depth",
        score: 70,
      },
    ],
    questions: [
      {
        position: 1,
        question: "Describe a hard bug you fixed.",
        answer: "A race in the cache invalidation.",
        feedback: "Good root-cause analysis.",
      },
      {
        position: 2,
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
  await expect(page.getByText("82 / 100")).toBeVisible();
  await expect(
    page.getByText("Clear answers with one gap in system design."),
  ).toBeVisible();
  await expect(
    page.getByRole("progressbar", { name: "Communication" }),
  ).toHaveAttribute("aria-valuenow", "80");
  await expect(
    page.getByRole("progressbar", { name: "Technical depth" }),
  ).toHaveAttribute("aria-valuenow", "70");
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
          breakdown: null,
          questions: null,
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
  await expect(page.getByText("82 / 100")).toBeVisible();
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
          breakdown: null,
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
          breakdown: null,
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

test("a recruitment report shows only the score and status, with no transcript or recording", async ({
  page,
}) => {
  await mockReports(page, {
    [recruitmentDone]: [
      {
        status: 200,
        body: report({
          interview_id: recruitmentDone,
          type: "recruitment",
          overall_score: 74,
          summary: null,
          breakdown: null,
          questions: null,
        }),
      },
    ],
  });
  await page.goto(`/reports/${recruitmentDone}`);

  await expect(
    page.getByRole("heading", { level: 1, name: "Recruitment interview" }),
  ).toBeVisible();
  await expect(page.getByText("Completed", { exact: true })).toBeVisible();
  await expect(page.getByText("74 / 100")).toBeVisible();
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
