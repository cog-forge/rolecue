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

const application = (
  i: number,
  name: string,
  email: string,
  over: Record<string, unknown> = {},
) => ({
  id: `44444444-4444-4444-8444-${String(i).padStart(12, "0")}`,
  job_posting_id: postingId,
  job_posting_title: "Backend Engineer",
  status: "pending",
  submitted_at: `2026-10-${String(10 + i).padStart(2, "0")}T08:00:00Z`,
  candidate: {
    id: `55555555-5555-4555-8555-${String(i).padStart(12, "0")}`,
    name,
    email,
    image: null,
  },
  interview_eligible: false,
  interview_deadline: null,
  reject_reason: null,
  updated_at: "2026-10-20T08:00:00Z",
  ...over,
});
const bao = application(1, "Bao To", "bao@example.com");
const linh = application(2, "Linh Tran", "linh@example.com", {
  status: "interview_eligible",
  interview_eligible: true,
  interview_deadline: "2099-01-01T08:00:00Z",
});
const mai = application(3, "Mai Pham", "mai@example.com", {
  status: "rejected",
});
const detailPath = (row: { id: string }) =>
  `/recruiter/job-postings/${postingId}/applications/${row.id}`;

type Options = {
  slot?: number;
  cvApprovals?: number;
  role?: "recruiter" | "candidate";
  missing?: boolean;
};

// Serves the Recruiter's lists and one Application's detail the way Go does, keeps the decisions in
// memory so a reload shows them, and records every decision request.
async function mockApplicationDetail(page: Page, options: Options = {}) {
  const rows = new Map<string, Record<string, unknown>>(
    [bao, linh, mai].map((row) => [row.id, { ...row }]),
  );
  const approves: string[] = [];
  const rejects: unknown[] = [];
  const posting = {
    id: postingId,
    title: "Backend Engineer",
    status: "open",
    interview_slot: options.slot ?? 5,
    cv_approvals: options.cvApprovals ?? 0,
  };
  const me = { ...recruiter, role: options.role ?? "recruiter" };
  const headers = {
    "access-control-allow-origin": "http://127.0.0.1:3100",
    "access-control-allow-credentials": "true",
    "access-control-allow-methods": "GET,POST,OPTIONS",
    "access-control-allow-headers": "Content-Type",
  };
  const detailRoute =
    /^\/job-postings\/([^/]+)\/applications\/([^/]+)(\/approve-cv|\/reject-cv)?$/;

  await page.route("http://localhost:8080/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const method = request.method();
    if (method === "OPTIONS") return route.fulfill({ status: 204, headers });
    if (url.pathname === "/auth/me")
      return route.fulfill({ headers, json: { success: true, data: me } });
    if (
      url.pathname === "/recruiter/applications" ||
      url.pathname === `/job-postings/${postingId}/applications`
    ) {
      const items = [...rows.values()];
      return route.fulfill({
        headers,
        json: {
          success: true,
          data: {
            items,
            pagination: { limit: 20, offset: 0, total: items.length },
          },
        },
      });
    }
    const match = url.pathname.match(detailRoute);
    if (match) {
      const [, requestedPosting, applicationId, action] = match;
      const row = rows.get(applicationId);
      if (requestedPosting !== postingId || !row || options.missing)
        return route.fulfill({
          status: 404,
          headers,
          json: {
            success: false,
            error: {
              code: "APPLICATION_NOT_FOUND",
              message: "This application could not be found.",
            },
          },
        });
      if (method === "GET" && !action)
        return route.fulfill({
          headers,
          json: {
            success: true,
            data: {
              ...row,
              job_posting: posting,
              // A short-lived link, as the API issues it.
              cv: {
                url: "https://files.example.com/cv/linh.pdf?signature=test",
                expires_at: new Date(Date.now() + 5 * 60 * 1000).toISOString(),
              },
            },
          },
        });
      if (method === "POST" && action === "/approve-cv") {
        approves.push(url.pathname);
        row.status = "interview_eligible";
        row.interview_eligible = true;
        row.interview_deadline = new Date(
          Date.now() + 24 * 60 * 60 * 1000,
        ).toISOString();
        return route.fulfill({
          headers,
          json: {
            success: true,
            data: {
              id: row.id,
              job_posting_id: postingId,
              status: row.status,
              interview_deadline: row.interview_deadline,
              reject_reason: null,
              updated_at: "2026-10-24T09:00:00Z",
              job_posting_status: "open",
            },
          },
        });
      }
      if (method === "POST" && action === "/reject-cv") {
        const body = request.postDataJSON() as {
          reject_reason?: string;
        };
        rejects.push(body);
        row.status = "rejected";
        row.reject_reason = body.reject_reason ?? null;
        return route.fulfill({
          headers,
          json: {
            success: true,
            data: {
              id: row.id,
              job_posting_id: postingId,
              status: row.status,
              interview_deadline: null,
              reject_reason: row.reject_reason,
              updated_at: "2026-10-24T09:00:00Z",
              job_posting_status: "open",
            },
          },
        });
      }
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
  return { approves, rejects };
}

test("a row opens the Application with its candidate, status and CV link", async ({
  page,
}) => {
  await mockApplicationDetail(page);
  await page.goto("/recruiter/applications");

  await page.getByRole("table").getByRole("link", { name: "Bao To" }).click();
  await expect(page).toHaveURL(detailPath(bao));
  await expect(
    page.getByRole("heading", { level: 1, name: "Bao To" }),
  ).toBeVisible();
  await expect(page.getByText("Awaiting CV review")).toBeVisible();
  await expect(
    page.getByRole("link", { name: /open the cv/i }),
  ).toHaveAttribute("href", /files\.example\.com/);
});

test("approving moves the Application to eligible with a deadline, and posts once", async ({
  page,
}) => {
  const api = await mockApplicationDetail(page);
  await page.goto(detailPath(bao));

  await page.getByRole("button", { name: "Approve CV" }).click();
  await page
    .getByRole("dialog", { name: "Approve this CV?" })
    .getByRole("button", { name: "Confirm approval" })
    .click();

  await expect(page.getByText("Interview eligible")).toBeVisible();
  await expect(page.getByText("Complete interview by")).toBeVisible();
  await expect(page.getByRole("button", { name: "Approve CV" })).toHaveCount(0);
  expect(api.approves).toEqual([
    `/job-postings/${postingId}/applications/${bao.id}/approve-cv`,
  ]);
});

test("rejecting posts the reason and shows the rejected state", async ({
  page,
}) => {
  const api = await mockApplicationDetail(page);
  await page.goto(detailPath(bao));

  await page.getByRole("button", { name: "Reject CV" }).click();
  const dialog = page.getByRole("dialog", { name: "Reject this CV?" });
  await dialog
    .getByLabel(/Reason for the candidate/)
    .fill("Missing Go experience.");
  await dialog.getByRole("button", { name: "Reject application" }).click();

  await expect(page.getByText("Rejected", { exact: true })).toBeVisible();
  await expect(page.getByText("Missing Go experience.")).toBeVisible();
  expect(api.rejects).toEqual([{ reject_reason: "Missing Go experience." }]);
});

test("with every interview slot taken, Approve is disabled and explained, and Reject still works", async ({
  page,
}) => {
  const api = await mockApplicationDetail(page, { slot: 2, cvApprovals: 2 });
  await page.goto(detailPath(bao));

  await expect(page.getByRole("button", { name: "Approve CV" })).toBeDisabled();
  await expect(
    page.getByText(/All interview slots for this posting are taken/),
  ).toBeVisible();

  await page.getByRole("button", { name: "Reject CV" }).click();
  await page
    .getByRole("dialog", { name: "Reject this CV?" })
    .getByRole("button", { name: "Reject application" })
    .click();
  await expect(page.getByText("Rejected", { exact: true })).toBeVisible();
  expect(api.rejects).toHaveLength(1);
});

test("a candidate who opens the detail is sent to their own dashboard", async ({
  page,
}) => {
  await mockApplicationDetail(page, { role: "candidate" });
  await page.goto(detailPath(bao));
  await expect(page).toHaveURL((url) => url.pathname === "/dashboard");
});

test("an Application that is not found shows the not-found state with a way back", async ({
  page,
}) => {
  await mockApplicationDetail(page, { missing: true });
  await page.goto(detailPath(bao));

  await expect(
    page.getByText("We couldn't find this application"),
  ).toBeVisible();
  await page
    .getByRole("link", { name: "Back to received applications" })
    .click();
  await expect(page).toHaveURL("/recruiter/applications");
});
