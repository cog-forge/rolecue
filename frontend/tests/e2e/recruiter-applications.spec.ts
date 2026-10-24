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
const all = [
  application(3, "Mai Pham", "mai@example.com", { status: "rejected" }),
  application(2, "Linh Tran", "linh@example.com", {
    status: "interview_eligible",
    interview_eligible: true,
    interview_deadline: "2099-01-01T08:00:00Z",
  }),
  application(1, "Bao To", "bao@example.com"),
];

const posting = {
  id: postingId,
  title: "Backend Engineer",
  status: "open",
  reject_reason: null,
  raw_text: "We build Go services on PostgreSQL for payments. ".repeat(3),
  has_document: false,
  seniority_level: "senior",
  requirements: [
    { name: "Go", target: "programming_language", type: "required" },
  ],
  location: "Hanoi",
  employment_type: null,
  salary_min: null,
  salary_max: null,
  interview_duration: 45,
  interview_slot: 5,
  required_avatar: null,
  required_voice: null,
  core_question_count: 3,
  created_at: "2026-10-12T08:00:00Z",
};

// Serves the Recruiter's Application lists the way Go filters them, and records every query.
async function mockApplications(page: Page) {
  const queries: Record<string, string>[] = [];
  await page.route("http://localhost:8080/**", async (route) => {
    const url = new URL(route.request().url());
    const method = route.request().method();
    const headers = {
      "access-control-allow-origin": "http://127.0.0.1:3100",
      "access-control-allow-credentials": "true",
      "access-control-allow-methods": "GET,OPTIONS",
      "access-control-allow-headers": "Content-Type",
    };
    if (method === "OPTIONS") return route.fulfill({ status: 204, headers });
    if (url.pathname === "/auth/me")
      return route.fulfill({
        headers,
        json: { success: true, data: recruiter },
      });
    if (url.pathname === `/job-postings/${postingId}`)
      return route.fulfill({ headers, json: { success: true, data: posting } });
    if (
      url.pathname === "/recruiter/applications" ||
      url.pathname === `/job-postings/${postingId}/applications`
    ) {
      const query = Object.fromEntries(url.searchParams);
      queries.push(query);
      const search = (query.q ?? "").toLowerCase();
      const items = all.filter(
        (a) =>
          (!query.status || a.status === query.status) &&
          (!search ||
            a.candidate.name.toLowerCase().includes(search) ||
            a.candidate.email.toLowerCase().includes(search)),
      );
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
    return route.fulfill({
      status: 404,
      headers,
      json: {
        success: false,
        error: { code: "NOT_FOUND", message: "unmocked" },
      },
    });
  });
  return { queries };
}

test("recruiter searches and filters the received applications", async ({
  page,
}) => {
  const api = await mockApplications(page);
  await page.goto("/recruiter/applications");

  await expect(
    page.getByRole("heading", { level: 1, name: "Received Applications" }),
  ).toBeVisible();
  const table = page.getByRole("table");
  await expect(table.getByText("Mai Pham")).toBeVisible();
  await expect(table.getByText("Linh Tran")).toBeVisible();
  await expect(table.getByText("Bao To")).toBeVisible();
  await expect(table.getByText(/Complete interview by/)).toBeVisible();

  // Search by name: the URL carries it and only the match stays.
  await page.getByLabel("Search by name or email").fill("linh");
  // Scoped to the form: the sidebar also has a "Search pages" button.
  await page
    .getByRole("search", { name: "Search applications" })
    .getByRole("button", { name: "Search", exact: true })
    .click();
  await expect(page).toHaveURL(/[?&]q=linh/);
  await expect(table.getByText("Linh Tran")).toBeVisible();
  await expect(table.getByText("Mai Pham")).toHaveCount(0);

  // A status tab narrows it, and nothing matches the pair.
  await page.getByRole("link", { name: "Rejected" }).click();
  await expect(page).toHaveURL(/status=rejected/);
  await expect(page.getByText("No applications match “linh”.")).toBeVisible();

  await page.getByRole("link", { name: "Show all applications" }).click();
  await expect(table.getByText("Bao To")).toBeVisible();
  expect(api.queries.some((q) => q.q === "linh")).toBe(true);
  expect(api.queries.some((q) => q.status === "rejected")).toBe(true);
});

test("a filtered URL reloads into the same list", async ({ page }) => {
  await mockApplications(page);
  await page.goto("/recruiter/applications?status=rejected");
  const table = page.getByRole("table");
  await expect(table.getByText("Mai Pham")).toBeVisible();
  await expect(table.getByText("Linh Tran")).toHaveCount(0);
  await expect(
    page.getByRole("link", { name: "Rejected", exact: true }),
  ).toHaveAttribute("aria-current", "page");
});

test("an approved posting lists its applications and links to the criteria", async ({
  page,
}) => {
  const api = await mockApplications(page);
  await page.goto(`/recruiter/job-postings/${postingId}`);

  await expect(
    page.getByRole("heading", { level: 1, name: "Backend Engineer" }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { level: 2, name: "Applications" }),
  ).toBeVisible();
  const table = page.getByRole("table");
  await expect(table.getByText("Linh Tran")).toBeVisible();
  await expect(
    table.getByRole("columnheader", { name: "Job posting" }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("link", { name: "Evaluation criteria" }),
  ).toHaveAttribute("href", `/recruiter/job-postings/${postingId}/evaluation`);
  await expect(page.getByText("Interview slots:")).toContainText("5");

  await page.getByLabel("Search by name or email").fill("bao@");
  // Scoped to the form: the sidebar also has a "Search pages" button.
  await page
    .getByRole("search", { name: "Search applications" })
    .getByRole("button", { name: "Search", exact: true })
    .click();
  await expect(table.getByText("Bao To")).toBeVisible();
  await expect(table.getByText("Linh Tran")).toHaveCount(0);
  expect(api.queries.some((q) => q.q === "bao@")).toBe(true);
});
