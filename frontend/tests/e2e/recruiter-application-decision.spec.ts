import { expect, test, type Page } from "@playwright/test";

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
const headers = {
  "access-control-allow-origin": "http://127.0.0.1:3100",
  "access-control-allow-credentials": "true",
  "access-control-allow-methods": "GET,POST,OPTIONS",
  "access-control-allow-headers": "Content-Type",
};

const postingId = "33333333-3333-4333-8333-333333333333";
const applicationId = "44444444-4444-4444-8444-444444444444";
const interviewId = "55555555-5555-4555-8555-555555555555";
const candidateId = "77777777-7777-4777-8777-777777777777";
const detailPath = `/job-postings/${postingId}/applications/${applicationId}`;
const recruiterDetailPath = `/recruiter/job-postings/${postingId}/applications/${applicationId}`;

type Status = "interviewed" | "interview_eligible" | "approved" | "rejected";

type Scenario = {
  status?: Status;
  // The Application's interview deadline state. A missed deadline is interview_eligible with
  // interview_eligible false.
  missed?: boolean;
  canDecide?: boolean;
  // opens_at as the server computes it; null when no deadline exists for the posting.
  opensAt?: string | null;
  // Makes the decision request fail with this envelope instead of applying the decision.
  decisionError?: { status: number; code: string; message: string };
  // When set, the decision response waits for this promise before it is sent.
  decisionGate?: Promise<void>;
};

function detail(
  status: Status,
  canDecide: boolean,
  opensAt: string | null,
  missed: boolean,
) {
  return {
    id: applicationId,
    job_posting_id: postingId,
    job_posting_title: "Backend Engineer",
    status,
    submitted_at: "2026-10-10T08:00:00Z",
    candidate: {
      id: candidateId,
      name: "Riley Candidate",
      email: "riley@example.com",
      image: null,
    },
    interview_eligible: status === "interview_eligible" && !missed,
    interview_deadline:
      status === "interview_eligible" || status === "interviewed"
        ? "2020-01-01T08:00:00Z"
        : null,
    reject_reason:
      status === "rejected" && missed ? "Interview deadline passed" : null,
    updated_at: "2026-10-24T09:00:00Z",
    job_posting: {
      id: postingId,
      title: "Backend Engineer",
      status: "open",
      interview_slot: 5,
      cv_approvals: 1,
    },
    cv: {
      url: "https://files.example.com/cv/riley.pdf?signature=test",
      expires_at: new Date(Date.now() + 5 * 60 * 1000).toISOString(),
    },
    interview: {
      id: interviewId,
      started_at: "2026-10-12T08:00:00Z",
      completed_at: "2026-10-12T08:40:00Z",
      status: "completed",
      score: 78,
      record_status: "ready",
    },
    final_review: { opens_at: opensAt, can_decide: canDecide },
  };
}

// Serves one Application the way Go does: the detail carries the interview summary and the final
// review gate, the evidence endpoints answer only when asked, and a decision changes the state
// the next read returns. Every evidence and decision request is counted.
async function mockDecision(page: Page, scenario: Scenario = {}) {
  let status: Status = scenario.status ?? "interviewed";
  const decisions: unknown[] = [];
  const evidence = { report: 0, recording: 0, transcript: 0 };
  const canDecide = scenario.canDecide ?? false;
  const opensAt =
    scenario.opensAt === undefined ? "2099-06-15T12:00:00Z" : scenario.opensAt;
  const missed = scenario.missed ?? false;

  await page.route("http://localhost:8080/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const method = request.method();
    if (method === "OPTIONS") return route.fulfill({ status: 204, headers });
    const ok = (data: unknown) =>
      route.fulfill({ headers, json: { success: true, data } });

    if (url.pathname === "/auth/me") return ok(recruiter);
    if (url.pathname === detailPath && method === "GET")
      return ok(detail(status, canDecide, opensAt, missed));
    if (url.pathname === `${detailPath}/decision` && method === "POST") {
      decisions.push(request.postDataJSON());
      if (scenario.decisionGate) await scenario.decisionGate;
      if (scenario.decisionError)
        return route.fulfill({
          status: scenario.decisionError.status,
          headers,
          json: {
            success: false,
            error: {
              code: scenario.decisionError.code,
              message: scenario.decisionError.message,
            },
          },
        });
      const body = request.postDataJSON() as { decision: string };
      status = body.decision === "approve" ? "approved" : "rejected";
      return ok(detail(status, canDecide, opensAt, missed));
    }
    if (url.pathname === `/interviews/${interviewId}/report`) {
      evidence.report += 1;
      return ok({
        interview_id: interviewId,
        type: "recruitment",
        status: "completed",
        overall_score: 78,
        started_at: "2026-10-12T08:00:00Z",
        completed_at: "2026-10-12T08:40:00Z",
        summary: "Solid system design answers with clear tradeoffs.",
        breakdown: [
          {
            metric_id: "dddddddd-dddd-4ddd-8ddd-000000000001",
            name: "Communication",
            score: 80,
          },
        ],
        questions: null,
      });
    }
    if (url.pathname === `${detailPath}/recording`) {
      evidence.recording += 1;
      return ok({
        record_status: "ready",
        url: "https://files.example.com/recordings/riley.webm?signature=test",
        expires_at: "2099-01-01T08:00:00Z",
      });
    }
    if (url.pathname === `${detailPath}/transcript`) {
      evidence.transcript += 1;
      return ok({
        turns: [
          {
            position: 1,
            question: "Walk me through a service you built.",
            answer: "I built the payments API in Go.",
          },
        ],
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
  return { decisions, evidence };
}

test("the interview result loads the evaluation, transcript and recording only when the Recruiter asks", async ({
  page,
}) => {
  const { evidence } = await mockDecision(page);
  await page.goto(recruiterDetailPath);

  await expect(
    page.getByRole("heading", { level: 1, name: "Riley Candidate" }),
  ).toBeVisible();
  const result = page.getByRole("heading", { name: "Interview", exact: true });
  await expect(result).toBeVisible();
  await expect(page.getByText("Evaluated", { exact: true })).toBeVisible();
  await expect(page.getByText("78/100")).toBeVisible();
  expect(evidence).toEqual({ report: 0, recording: 0, transcript: 0 });

  await page.getByRole("button", { name: "Show evaluation" }).click();
  await expect(
    page.getByText("Solid system design answers with clear tradeoffs."),
  ).toBeVisible();
  await expect(page.getByText("Communication", { exact: true })).toBeVisible();
  await expect(page.getByText("80/100")).toBeVisible();
  expect(evidence.report).toBe(1);

  await page.getByRole("button", { name: "Show transcript" }).click();
  await expect(
    page.getByText("Walk me through a service you built."),
  ).toBeVisible();
  expect(evidence.transcript).toBe(1);
  expect(evidence.recording).toBe(0);

  await page.getByRole("button", { name: "Get the recording link" }).click();
  await expect(
    page.getByRole("link", { name: /open the recording/i }),
  ).toHaveAttribute("href", /files\.example\.com\/recordings/);
  expect(evidence.recording).toBe(1);
});

test("Approve and Reject stay disabled and show when the final decision opens until the server says it can decide", async ({
  page,
}) => {
  await mockDecision(page, {
    canDecide: false,
    opensAt: "2099-01-01T08:00:00Z",
  });
  await page.goto(recruiterDetailPath);

  await expect(
    page.getByRole("button", { name: "Approve application" }),
  ).toBeDisabled();
  await expect(
    page.getByRole("button", { name: "Reject application" }),
  ).toBeDisabled();
  await expect(
    page.getByRole("heading", { name: "Final decision is not open yet" }),
  ).toBeVisible();
  await expect(page.getByText(/Decisions open .*2099/)).toBeVisible();
});

test("once the server allows a decision, Approve posts once and the status changes only after the server confirms", async ({
  page,
}) => {
  let release!: () => void;
  const gate = new Promise<void>((resolve) => {
    release = resolve;
  });
  const { decisions } = await mockDecision(page, {
    canDecide: true,
    decisionGate: gate,
  });
  await page.goto(recruiterDetailPath);

  const approve = page.getByRole("button", { name: "Approve application" });
  await expect(approve).toBeEnabled();
  await approve.click();

  const dialog = page.getByRole("dialog", {
    name: "Approve this application?",
  });
  const sent = page.waitForRequest(
    (req) =>
      req.url().endsWith(`${detailPath}/decision`) && req.method() === "POST",
  );
  await dialog.getByRole("button", { name: "Confirm approval" }).click();
  await sent;

  await expect(
    dialog.getByRole("button", { name: "Approving…" }),
  ).toBeDisabled();
  await expect(page.getByText("Interviewed", { exact: true })).toBeVisible();
  await expect(page.getByText("Approved", { exact: true })).toHaveCount(0);

  release();

  await expect(page.getByText("Approved", { exact: true })).toBeVisible();
  await expect(dialog).toHaveCount(0);
  expect(decisions).toEqual([{ decision: "approve" }]);
});

test("a decision refused because the final review is locked shows the locked message and keeps the status", async ({
  page,
}) => {
  const { decisions } = await mockDecision(page, {
    canDecide: true,
    decisionError: {
      status: 409,
      code: "FINAL_REVIEW_LOCKED",
      message: "final review is locked",
    },
  });
  await page.goto(recruiterDetailPath);

  await page.getByRole("button", { name: "Approve application" }).click();
  await page
    .getByRole("dialog", { name: "Approve this application?" })
    .getByRole("button", { name: "Confirm approval" })
    .click();

  await expect(
    page.getByRole("alert").filter({
      hasText: "Final decisions are not open yet.",
    }),
  ).toBeVisible();
  await expect(page.getByText("Interviewed", { exact: true })).toBeVisible();
  expect(decisions).toHaveLength(1);
});

test("an Application whose interview deadline was missed shows as rejected with no final decision", async ({
  page,
}) => {
  await mockDecision(page, { status: "interview_eligible", missed: true });
  await page.goto(recruiterDetailPath);

  await expect(page.getByText("Rejected", { exact: true })).toBeVisible();
  await expect(page.getByText("Interview deadline passed")).toBeVisible();
  await expect(
    page.getByText("Interview eligible", { exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "Approve application" }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "Reject application" }),
  ).toHaveCount(0);
});
