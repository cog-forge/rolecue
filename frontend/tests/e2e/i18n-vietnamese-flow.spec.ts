import { expect, test, type Page } from "@playwright/test";

// KAN-229: one recruiter journey with the UI in Vietnamese, from the sign-in form to an open
// Job Posting. Screen text is Vietnamese; what the recruiter wrote stays as stored.
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
const postingId = "66666666-6666-4666-8666-666666666666";
const headers = {
  "access-control-allow-origin": "http://127.0.0.1:3100",
  "access-control-allow-credentials": "true",
  "access-control-allow-methods": "GET,POST,OPTIONS",
  "access-control-allow-headers": "Content-Type",
};
const posting = {
  id: postingId,
  title: "Backend Engineer",
  status: "open",
  reject_reason: null,
  raw_text: "We build reliable APIs in Go.",
  has_document: false,
  seniority_level: "mid",
  requirements: [],
  refinement_note: "",
  location: "Hanoi",
  employment_type: "Full-time",
  salary_min: null,
  salary_max: null,
  interview_duration: 30,
  interview_slot: 5,
  required_avatar: null,
  required_voice: null,
  core_question_count: 0,
  review_submitted: true,
  created_at: "2026-10-12T08:00:00Z",
};

async function mockRecruiterApi(page: Page) {
  let signedIn = false;
  await page.route("**/api/auth/**", (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path.endsWith("/sign-in/email")) {
      signedIn = true;
      return route.fulfill({
        json: {
          redirect: false,
          token: "MOCK-TOKEN",
          user: { id: recruiter.id, role: "recruiter" },
        },
      });
    }
    if (path.endsWith("/get-session"))
      return route.fulfill({
        json: signedIn
          ? {
              user: {
                id: recruiter.id,
                name: recruiter.full_name,
                email: recruiter.email,
                emailVerified: true,
                role: "recruiter",
              },
              session: { id: "s1", userId: recruiter.id },
            }
          : null,
      });
    return route.fulfill({ status: 404, json: { code: "NOT_FOUND" } });
  });
  await page.route("http://localhost:8080/**", (route) => {
    const request = route.request();
    const url = new URL(request.url());
    if (request.method() === "OPTIONS")
      return route.fulfill({ status: 204, headers });
    const ok = (data: unknown) =>
      route.fulfill({ headers, json: { success: true, data } });
    const page_ = (items: unknown[]) =>
      ok({
        items,
        pagination: { limit: 20, offset: 0, total: items.length },
      });
    if (url.pathname === "/auth/me") return ok(recruiter);
    if (url.pathname === "/recruiter/applications") return page_([]);
    if (url.pathname === "/job-postings") return page_([posting]);
    if (url.pathname === `/job-postings/${postingId}`) return ok(posting);
    if (url.pathname === `/job-postings/${postingId}/applications`)
      return page_([]);
    if (url.pathname === `/job-postings/${postingId}/interview-capacity`)
      return ok({
        interview_slot: 5,
        funded: true,
        price_per_slot: 1,
        cv_approvals: 0,
        consumed: 0,
        available_to_approve: 5,
        refundable: 5,
        posting_status: "open",
        refunded: null,
      });
    return route.fulfill({
      status: 404,
      headers,
      json: {
        success: false,
        error: { code: "NOT_FOUND", message: "unmocked" },
      },
    });
  });
}

test("a recruiter signs in, sees the dashboard and opens a Job Posting, all in Vietnamese", async ({
  page,
}) => {
  await page
    .context()
    .addCookies([
      { name: "NEXT_LOCALE", value: "vi", url: "http://127.0.0.1:3100" },
    ]);
  await mockRecruiterApi(page);

  await page.goto("/login");
  await expect(page.locator("html")).toHaveAttribute("lang", "vi");
  await expect(
    page.getByRole("heading", { name: "Đăng nhập vào RoleCue" }),
  ).toBeVisible();
  await page.getByLabel("Địa chỉ email").fill("rec@example.com");
  await page.getByLabel("Mật khẩu", { exact: true }).fill("Correct-Horse-1");
  await page.getByRole("button", { name: "Đăng nhập", exact: true }).click();

  await expect(page).toHaveURL(/\/recruiter\/dashboard/);
  await expect(
    page.getByRole("heading", { level: 1, name: "Tổng quan" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Đổi ngôn ngữ: Tiếng Việt" }),
  ).toBeVisible();

  await page
    .getByRole("link", { name: "Tin tuyển dụng của tôi" })
    .first()
    .click();
  await expect(
    page.getByRole("heading", { level: 1, name: "Tin tuyển dụng của tôi" }),
  ).toBeVisible();
  await expect(
    page
      .getByRole("button", { name: /Tạo tin tuyển dụng/ })
      .or(page.getByRole("link", { name: /Tạo tin tuyển dụng/ }))
      .first(),
  ).toBeVisible();
  await expect(page.getByText("Đang tuyển").first()).toBeVisible();

  await page.getByRole("link", { name: "Backend Engineer" }).first().click();
  await expect(page).toHaveURL(new RegExp(`/job-postings/${postingId}`));
  // The recruiter's own title stays as written; the screen around it is Vietnamese.
  await expect(
    page.getByRole("heading", { name: "Backend Engineer" }).first(),
  ).toBeVisible();
  await expect(page.getByText("Lượt phỏng vấn:")).toBeVisible();
  await expect(page.getByText("Đang tuyển").first()).toBeVisible();
  await expect(page.getByText("Interview slots")).toHaveCount(0);
});
