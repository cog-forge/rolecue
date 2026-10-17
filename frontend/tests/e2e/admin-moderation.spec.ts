import { test, expect, type Page } from "@playwright/test";

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

async function mockModeration(page: Page) {
  let status = "pending";
  let rejectReason: string | null = null;
  const rejectBodies: unknown[] = [];
  const posting = () => ({
    id: postingId,
    title: "Go Backend Engineer",
    status,
    reject_reason: rejectReason,
    recruiter,
    location: "Ho Chi Minh City",
    employment_type: "Full-time",
    salary_min: 1500,
    salary_max: 2500,
    interview_slot: 10,
    interview_duration: 30,
    required_avatar: null,
    required_voice: null,
    created_at: "2026-10-12T08:00:00Z",
    content_path: "https://files.example.com/jd/go-backend.pdf",
    document_url: null,
    document_expires_at: null,
    raw_text: null,
    core_question_count: 50,
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
    if (url.pathname === "/admin/job-postings") {
      const wanted = url.searchParams.get("status") ?? "pending";
      const items =
        wanted === "all" || wanted === status
          ? [
              {
                id: postingId,
                title: "Go Backend Engineer",
                status,
                recruiter,
                created_at: "2026-10-12T08:00:00Z",
              },
            ]
          : [];
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
    if (url.pathname === `/admin/job-postings/${postingId}`)
      return route.fulfill({
        headers,
        json: { success: true, data: posting() },
      });
    if (
      url.pathname === `/admin/job-postings/${postingId}/reject` &&
      method === "POST"
    ) {
      const body = route.request().postDataJSON() as { reject_reason: string };
      rejectBodies.push(body);
      status = "rejected";
      rejectReason = body.reject_reason;
      return route.fulfill({
        headers,
        json: { success: true, data: posting() },
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
  return { rejectBodies };
}

test("admin reviews a pending posting and rejects it with a reason", async ({
  page,
}) => {
  const api = await mockModeration(page);
  await page.goto("/admin/job-postings");

  await expect(
    page.getByRole("heading", { level: 1, name: "Job Posting Moderation" }),
  ).toBeVisible();
  const row = page.getByRole("row", { name: /Go Backend Engineer/ });
  await expect(row).toContainText("Pending review");

  await row.getByRole("link", { name: "Review Go Backend Engineer" }).click();
  await expect(page).toHaveURL(new RegExp(`/admin/job-postings/${postingId}`));
  await expect(page.getByTitle("Job description PDF")).toHaveAttribute(
    "src",
    "https://files.example.com/jd/go-backend.pdf",
  );
  await expect(
    page.getByText(/core questions in the question bank/),
  ).toContainText("50");

  await page.getByRole("button", { name: "Reject" }).click();
  const dialog = page.getByRole("dialog", { name: "Reject this posting?" });
  await dialog.getByRole("button", { name: "Reject posting" }).click();
  await expect(dialog.getByRole("alert")).toContainText(
    "Tell the recruiter why",
  );

  await dialog
    .getByLabel(/Reason for the recruiter/)
    .fill("Salary range is missing.");
  await dialog.getByRole("button", { name: "Reject posting" }).click();
  await expect(dialog).toBeHidden();
  await expect(page.getByText("“Salary range is missing.”")).toBeVisible();
  expect(api.rejectBodies).toEqual([
    { reject_reason: "Salary range is missing." },
  ]);

  await page.getByRole("link", { name: "Back to moderation queue" }).click();
  await expect(
    page.getByText("No postings are waiting for review."),
  ).toBeVisible();
});

test("queue filters live in the URL", async ({ page }) => {
  await mockModeration(page);
  await page.goto("/admin/job-postings");
  await page.getByRole("link", { name: "All" }).click();
  await expect(page).toHaveURL(/status=all/);
  await expect(page.getByRole("link", { name: "All" })).toHaveAttribute(
    "aria-current",
    "page",
  );
  await page.reload();
  await expect(page.getByRole("link", { name: "All" })).toHaveAttribute(
    "aria-current",
    "page",
  );
});
