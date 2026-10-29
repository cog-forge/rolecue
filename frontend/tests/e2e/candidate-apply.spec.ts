import { test, expect, type Page } from "@playwright/test";

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
const postingId = "33333333-3333-4333-8333-333333333333";
const applicationId = "77777777-7777-4777-8777-777777777777";
const headers = {
  "access-control-allow-origin": "http://127.0.0.1:3100",
  "access-control-allow-credentials": "true",
  "access-control-allow-methods": "GET,POST,OPTIONS",
  "access-control-allow-headers": "Content-Type",
};

const posting = {
  id: postingId,
  title: "Go Backend Engineer",
  company_name: "Acme",
  location: "Hanoi",
  employment_type: "Full-time",
  salary_min: null,
  salary_max: null,
  seniority_level: "senior",
  created_at: "2026-10-12T08:00:00Z",
  company_website: null,
  interview_duration: 30,
  raw_text: "We build reliable APIs in Go.",
  requirements: [],
};

const submitted = {
  id: applicationId,
  job_posting_id: postingId,
  status: "pending",
  created_at: "2026-10-25T08:00:00Z",
};

const application = (over: Record<string, unknown> = {}) => ({
  id: applicationId,
  job_posting_id: postingId,
  job_posting_title: "Go Backend Engineer",
  company_name: "Acme",
  job_posting_location: "Hanoi",
  status: "pending",
  reject_reason: null,
  interview_deadline: null,
  can_start_interview: false,
  created_at: "2026-10-25T08:00:00Z",
  ...over,
});

type Options = {
  alreadyApplied?: boolean;
  // Makes the upload fail with this envelope instead of creating the Application.
  applyError?: { status: number; code: string; message: string };
  // Rows for GET /applications.
  applications?: ReturnType<typeof application>[];
};

// Serves the Candidate's job board detail, the CV upload, one Application and the Applications list
// the way Go does, and records every upload request.
async function mockCandidateApply(page: Page, options: Options = {}) {
  const uploads: string[] = [];
  await page.route("http://localhost:8080/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const method = request.method();
    if (method === "OPTIONS") return route.fulfill({ status: 204, headers });
    const ok = (data: unknown, status = 200) =>
      route.fulfill({ status, headers, json: { success: true, data } });

    if (url.pathname === "/auth/me") return ok(candidate);
    if (url.pathname === `/job-board/${postingId}` && method === "GET")
      return ok({
        ...posting,
        already_applied: options.alreadyApplied ?? false,
      });
    if (
      url.pathname === `/job-board/${postingId}/applications` &&
      method === "POST"
    ) {
      uploads.push(request.postData() ?? "");
      if (options.applyError)
        return route.fulfill({
          status: options.applyError.status,
          headers,
          json: {
            success: false,
            error: {
              code: options.applyError.code,
              message: options.applyError.message,
            },
          },
        });
      return ok(submitted, 201);
    }
    if (url.pathname === `/applications/${applicationId}` && method === "GET")
      return ok(application());
    if (url.pathname === "/applications" && method === "GET") {
      const items = options.applications ?? [];
      return ok({
        items,
        total: items.length,
        page: 1,
        page_size: 20,
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
  return { uploads };
}

// Opens the apply dialog, then picks a PDF through its "Choose your CV (PDF)" button and the native
// file chooser.
async function chooseCv(page: Page, name = "cv.pdf") {
  await page.getByRole("button", { name: "Apply with your CV" }).click();
  await expect(
    page.getByRole("dialog", { name: "Apply for Go Backend Engineer" }),
  ).toBeVisible();
  const chooser = page.waitForEvent("filechooser");
  await page.getByRole("button", { name: "Choose your CV (PDF)" }).click();
  (await chooser).setFiles({
    name,
    mimeType: "application/pdf",
    buffer: Buffer.from("%PDF-1.4\n% test CV\n%%EOF\n"),
  });
  await expect(page.getByText(name, { exact: true })).toBeVisible();
}

test("a candidate applies with a PDF CV in a dialog, sees what happens next and opens the Application", async ({
  page,
}) => {
  const api = await mockCandidateApply(page);
  await page.goto(`/jobs/${postingId}`);
  await expect(
    page.getByRole("heading", { level: 1, name: "Go Backend Engineer" }),
  ).toBeVisible();

  await chooseCv(page);
  await page.getByRole("button", { name: "Send application" }).click();

  const dialog = page.getByRole("dialog", { name: "Application sent" });
  await expect(dialog).toBeVisible();
  await expect(
    dialog.getByText("The recruiter reviews your CV", { exact: false }),
  ).toBeVisible();
  await dialog.getByRole("link", { name: "View my application" }).click();

  await expect(page).toHaveURL(new RegExp(`/applications/${applicationId}$`));
  await expect(
    page.getByRole("heading", { level: 1, name: "Go Backend Engineer" }),
  ).toBeVisible();
  await expect(
    page.getByText("Awaiting CV review", { exact: true }),
  ).toBeVisible();
  expect(api.uploads).toHaveLength(1);
  expect(api.uploads[0]).toContain('filename="cv.pdf"');
});

test("applying twice shows the already-applied message and does not redirect", async ({
  page,
}) => {
  const api = await mockCandidateApply(page, {
    applyError: {
      status: 409,
      code: "ALREADY_APPLIED",
      message: "already applied",
    },
  });
  await page.goto(`/jobs/${postingId}`);
  await chooseCv(page);
  await page.getByRole("button", { name: "Send application" }).click();

  await expect(
    page.getByText("You already applied to this posting.", { exact: true }),
  ).toBeVisible();
  await expect(page).toHaveURL(new RegExp(`/jobs/${postingId}$`));
  await expect(
    page.getByRole("button", { name: "Send application" }),
  ).toBeDisabled();
  expect(api.uploads).toHaveLength(1);
});

test("a posting that stopped taking applications shows the intake message and does not redirect", async ({
  page,
}) => {
  const api = await mockCandidateApply(page, {
    applyError: {
      status: 409,
      code: "INTAKE_NOT_OPEN",
      message: "intake not open",
    },
  });
  await page.goto(`/jobs/${postingId}`);
  await chooseCv(page);
  await page.getByRole("button", { name: "Send application" }).click();

  await expect(
    page.getByText("This posting is no longer accepting applications.", {
      exact: true,
    }),
  ).toBeVisible();
  await expect(page).toHaveURL(new RegExp(`/jobs/${postingId}$`));
  await expect(
    page.getByRole("button", { name: "Send application" }),
  ).toBeDisabled();
  expect(api.uploads).toHaveLength(1);
});

test("My Applications lists each application with its company, status, progress and next action", async ({
  page,
}) => {
  await mockCandidateApply(page, {
    applications: [
      application(),
      application({
        id: "88888888-8888-4888-8888-888888888888",
        job_posting_title: "React Developer",
        status: "interview_eligible",
        can_start_interview: true,
        interview_deadline: "2099-01-01T08:00:00Z",
      }),
    ],
  });
  await page.goto("/applications");

  await expect(
    page.getByRole("heading", { level: 1, name: "My Applications" }),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Go Backend Engineer", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "React Developer", exact: true }),
  ).toBeVisible();
  await expect(page.getByText("Awaiting CV review")).toBeVisible();
  await expect(page.getByText("Interview eligible")).toBeVisible();
  await expect(page.getByText("Acme · Hanoi").first()).toBeVisible();
  await expect(
    page.getByRole("list", { name: "Application progress" }),
  ).toHaveCount(2);
  await expect(
    page.getByRole("link", { name: /Start interview/ }),
  ).toHaveAttribute(
    "href",
    "/interviews/new/preflight?applicationId=88888888-8888-4888-8888-888888888888",
  );
  await expect(
    page.getByRole("link", {
      name: "View application for Go Backend Engineer",
    }),
  ).toBeVisible();
  await expect(page.getByText("Showing 1–2 of 2")).toBeVisible();
});
