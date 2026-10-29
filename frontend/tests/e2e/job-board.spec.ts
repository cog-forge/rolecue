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
const goId = "33333333-3333-4333-8333-333333333333";
const closedId = "44444444-4444-4444-8444-444444444444";
const postings = [
  {
    id: goId,
    title: "Go Backend Engineer",
    company_name: "Acme",
    location: "Hanoi",
    employment_type: "Full-time",
    salary_min: 10_000_000,
    salary_max: 20_000_000,
    seniority_level: "senior",
    created_at: "2026-10-12T08:00:00Z",
  },
  {
    id: closedId,
    title: "React Developer",
    company_name: "Beta",
    location: "Da Nang",
    employment_type: "Contract",
    salary_min: null,
    salary_max: null,
    seniority_level: "junior",
    created_at: "2026-10-10T08:00:00Z",
  },
];

async function mockBoard(
  page: Page,
  alreadyApplied = false,
  role: "candidate" | "recruiter" | "admin" = "candidate",
) {
  const listQueries: URLSearchParams[] = [];
  const boardCalls: string[] = [];
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
        json: { success: true, data: { ...candidate, role } },
      });
    if (url.pathname.startsWith("/job-board")) boardCalls.push(url.pathname);
    if (url.pathname === "/job-board") {
      listQueries.push(url.searchParams);
      const search = url.searchParams.get("search")?.toLowerCase() ?? "";
      const items = postings.filter((p) =>
        p.title.toLowerCase().includes(search),
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
    if (url.pathname === `/job-board/${goId}`)
      return route.fulfill({
        headers,
        json: {
          success: true,
          data: {
            ...postings[0],
            company_website: "https://acme.example.com",
            interview_duration: 30,
            raw_text: "We build reliable APIs in Go.",
            requirements: [
              { name: "Go", target: "programming_language", type: "required" },
              { name: "Kubernetes", target: "tool", type: "preferred" },
            ],
            already_applied: alreadyApplied,
          },
        },
      });
    // Closed intake looks exactly like a missing posting.
    return route.fulfill({
      status: 404,
      headers,
      json: {
        success: false,
        error: {
          code: "JOB_POSTING_NOT_FOUND",
          message: "job posting not found",
        },
      },
    });
  });
  return { listQueries, boardCalls };
}

test("candidate searches the board, opens a posting and comes back to the same results", async ({
  page,
}) => {
  const api = await mockBoard(page);
  await page.goto("/jobs");

  await expect(
    page.getByRole("heading", { level: 1, name: "Job Board" }),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "React Developer" }),
  ).toBeVisible();

  const search = page.getByRole("search", { name: "Search open job postings" });
  await search.getByLabel("Job title").fill("go");
  await search.getByRole("button", { name: "Search" }).click();
  await expect(page).toHaveURL(/\/jobs\?q=go$/);
  await expect(page.getByRole("link", { name: "React Developer" })).toHaveCount(
    0,
  );
  expect(api.listQueries.at(-1)?.get("search")).toBe("go");

  await page.getByRole("link", { name: "Go Backend Engineer" }).click();
  await expect(page).toHaveURL(new RegExp(`/jobs/${goId}\\?back=q%3Dgo$`));
  await expect(
    page.getByRole("heading", { level: 1, name: "Go Backend Engineer" }),
  ).toBeVisible();
  await expect(page.getByText("We build reliable APIs in Go.")).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Choose your CV (PDF)" }),
  ).toBeVisible();

  await page.getByRole("link", { name: "Back to Job Board" }).click();
  await expect(page).toHaveURL(/\/jobs\?q=go$/);
  await expect(search.getByLabel("Job title")).toHaveValue("go");
});

test("browser Back and Forward move between searches", async ({ page }) => {
  await mockBoard(page);
  await page.goto("/jobs");
  await expect(
    page.getByRole("link", { name: "React Developer" }),
  ).toBeVisible();
  const search = page.getByRole("search", { name: "Search open job postings" });
  await search.getByLabel("Job title").fill("go");
  await search.getByRole("button", { name: "Search" }).click();
  await expect(page.getByRole("link", { name: "React Developer" })).toHaveCount(
    0,
  );

  await page.goBack();
  await expect(page).toHaveURL(/\/jobs$/);
  await expect(
    page.getByRole("link", { name: "React Developer" }),
  ).toBeVisible();
  await expect(search.getByLabel("Job title")).toHaveValue("");

  await page.goForward();
  await expect(page).toHaveURL(/\/jobs\?q=go$/);
  await expect(page.getByRole("link", { name: "React Developer" })).toHaveCount(
    0,
  );
  await expect(search.getByLabel("Job title")).toHaveValue("go");
});

for (const [role, home] of [
  ["recruiter", /\/recruiter\/dashboard$/],
  ["admin", /\/admin\/dashboard$/],
] as const)
  test(`a ${role} never reaches the board or its apply action`, async ({
    page,
  }) => {
    const api = await mockBoard(page, false, role);
    for (const path of ["/jobs", `/jobs/${goId}`]) {
      await page.goto(path);
      await expect(page).toHaveURL(home);
      await expect(page.getByRole("button", { name: /apply/i })).toHaveCount(0);
      await expect(page.getByRole("link", { name: "Job Board" })).toHaveCount(
        0,
      );
    }
    expect(api.boardCalls).toEqual([]);
  });

test("a posting the candidate already applied to shows Applied", async ({
  page,
}) => {
  await mockBoard(page, true);
  await page.goto(`/jobs/${goId}`);
  await expect(page.getByRole("button", { name: "Applied" })).toHaveAttribute(
    "aria-disabled",
    "true",
  );
  await expect(
    page.getByText("You already applied to this posting."),
  ).toBeVisible();
  await expect(page.getByRole("button", { name: "Apply" })).toHaveCount(0);
});

test("a closed or removed posting explains it is no longer available", async ({
  page,
}) => {
  await mockBoard(page);
  await page.goto(`/jobs/${closedId}`);
  await expect(
    page.getByRole("heading", { name: "This posting is no longer available" }),
  ).toBeVisible();
  await expect(page.getByRole("button", { name: /apply/i })).toHaveCount(0);
  await page.getByRole("link", { name: "Browse open postings" }).click();
  await expect(page).toHaveURL(/\/jobs$/);
});
