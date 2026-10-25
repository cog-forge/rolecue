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

const applicationId = "cccccccc-cccc-4ccc-8ccc-000000000001";
const practiceId = "bbbbbbbb-bbbb-4bbb-8bbb-000000000001";
const recruitmentId = "bbbbbbbb-bbbb-4bbb-8bbb-000000000002";

// Mirrors one row of GET /interviews (snake_case). Practice rows carry the Role Profile title;
// recruitment rows carry the Job Posting title and the Application they belong to.
function item(over: Record<string, unknown>) {
  return {
    id: practiceId,
    type: "practice",
    started_at: "2026-10-10T08:00:00Z",
    completed_at: "2026-10-10T08:30:00Z",
    evaluation_status: "completed",
    status: "completed",
    score: 82,
    role_profile_title: "Frontend Developer",
    job_posting_title: null,
    application_id: null,
    ...over,
  };
}

const practiceRow = item({});
const recruitmentRow = item({
  id: recruitmentId,
  type: "recruitment",
  score: 74,
  role_profile_title: "Backend Role",
  job_posting_title: "Backend Engineer",
  application_id: applicationId,
});

function paddedId(i: number) {
  return `bbbbbbbb-bbbb-4bbb-8bbb-${String(i).padStart(12, "0")}`;
}

function respond(route: Route, data: unknown, status = 200) {
  return route.fulfill({
    status,
    headers,
    json: status === 200 ? { success: true, data } : data,
  });
}

// Serves GET /interviews the way Go does: the type filter, page and page size come from the query
// string, and rows are newest first as they are given here.
async function mockHistory(page: Page, rows: ReturnType<typeof item>[]) {
  const requests: Array<{ type: string | null; page: string | null }> = [];
  await page.route("http://localhost:8080/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    if (request.method() === "OPTIONS")
      return route.fulfill({ status: 204, headers });
    if (url.pathname === "/auth/me") return respond(route, candidate);
    if (url.pathname === "/interviews" && request.method() === "GET") {
      const type = url.searchParams.get("type");
      const pageNumber = Number(url.searchParams.get("page") ?? "1");
      const pageSize = Number(url.searchParams.get("page_size") ?? "20");
      requests.push({ type, page: url.searchParams.get("page") });
      const filtered = rows.filter((row) => !type || row.type === type);
      const start = (pageNumber - 1) * pageSize;
      return respond(route, {
        items: filtered.slice(start, start + pageSize),
        total: filtered.length,
        page: pageNumber,
        page_size: pageSize,
      });
    }
    return respond(route, { code: "NOT_FOUND", message: "unmocked" }, 404);
  });
  return { requests };
}

test("history lists practice and recruitment interviews with their scores", async ({
  page,
}) => {
  await mockHistory(page, [practiceRow, recruitmentRow]);
  await page.goto("/history");

  await expect(
    page.getByRole("heading", { level: 1, name: "Interview history" }),
  ).toBeVisible();
  const list = page.getByRole("list", { name: "Your interviews" });
  const practice = list
    .getByRole("listitem")
    .filter({ hasText: "Frontend Developer" });
  await expect(practice).toContainText("Practice interview");
  await expect(practice).toContainText("82 / 100");
  const recruitment = list
    .getByRole("listitem")
    .filter({ hasText: "Backend Engineer" });
  await expect(recruitment).toContainText("Recruitment interview");
  await expect(recruitment).toContainText("74 / 100");
  await expect(recruitment).toContainText("Completed");
});

test("the type filter asks the server for that type and keeps the view in the URL", async ({
  page,
}) => {
  const { requests } = await mockHistory(page, [practiceRow, recruitmentRow]);
  await page.goto("/history");

  const filter = page.getByRole("navigation", {
    name: "Filter interviews by type",
  });
  const recruitmentFilter = filter.getByRole("link", {
    name: "Recruitment",
    exact: true,
  });
  await expect(recruitmentFilter).not.toHaveAttribute("aria-current", "page");
  await recruitmentFilter.click();

  await expect(page).toHaveURL(/\/history\?type=recruitment$/);
  await expect(recruitmentFilter).toHaveAttribute("aria-current", "page");
  const list = page.getByRole("list", { name: "Your interviews" });
  await expect(list.getByRole("listitem")).toHaveCount(1);
  await expect(
    list.getByRole("listitem").filter({ hasText: "Backend Engineer" }),
  ).toBeVisible();
  await expect.poll(() => requests.at(-1)?.type).toBe("recruitment");
});

test("pagination moves between pages and disables Next on the last page", async ({
  page,
}) => {
  const rows = Array.from({ length: 25 }, (_, i) =>
    item({
      id: paddedId(i + 1),
      role_profile_title: `Role ${String(i + 1).padStart(2, "0")}`,
    }),
  );
  await mockHistory(page, rows);
  await page.goto("/history");

  const pagination = page.getByRole("navigation", {
    name: "Interview history pagination",
  });
  await expect(pagination).toContainText("Showing 1–20 of 25");
  await expect(
    pagination.getByRole("button", { name: "Previous" }),
  ).toBeDisabled();
  await pagination.getByRole("link", { name: "Next" }).click();

  await expect(page).toHaveURL(/\/history\?page=2$/);
  await expect(pagination).toContainText("Showing 21–25 of 25");
  await expect(
    page.getByRole("list", { name: "Your interviews" }).getByRole("listitem"),
  ).toHaveCount(5);
  await expect(pagination.getByRole("button", { name: "Next" })).toBeDisabled();
  await pagination.getByRole("link", { name: "Previous" }).click();
  await expect(pagination).toContainText("Showing 1–20 of 25");
});

test("a filter with no interviews shows the empty state for that type", async ({
  page,
}) => {
  await mockHistory(page, [practiceRow]);
  await page.goto("/history?type=recruitment");

  await expect(
    page.getByRole("heading", {
      name: "You have no recruitment interviews yet.",
    }),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Go to the dashboard" }),
  ).toBeVisible();
  await expect(page.getByRole("list", { name: "Your interviews" })).toHaveCount(
    0,
  );
});

test("a practice row links to its report", async ({ page }) => {
  await mockHistory(page, [practiceRow]);
  await page.goto("/history");

  const link = page
    .getByRole("list", { name: "Your interviews" })
    .getByRole("link", { name: "Frontend Developer" });
  await expect(link).toHaveAttribute("href", `/reports/${practiceId}`);
});

test("a recruitment row links to its Application and shows only score and status", async ({
  page,
}) => {
  await mockHistory(page, [recruitmentRow]);
  await page.goto("/history");

  const row = page
    .getByRole("list", { name: "Your interviews" })
    .getByRole("listitem")
    .filter({ hasText: "Backend Engineer" });
  await expect(
    row.getByRole("link", { name: "View application" }),
  ).toHaveAttribute("href", `/applications/${applicationId}`);
  await expect(row).toContainText("Completed");
  await expect(row).toContainText("74 / 100");
  await expect(row.getByText(/transcript|recording|video|audio/i)).toHaveCount(
    0,
  );
});
