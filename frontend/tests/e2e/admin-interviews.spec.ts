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

const practiceInterview = {
  id: "44444444-4444-4444-8444-444444444441",
  type: "practice",
  owner: {
    id: "66666666-6666-4666-8666-666666666661",
    name: "Casey Candidate",
    email: "casey@example.com",
  },
  job_posting: null,
  score: 82,
  has_report: true,
  has_recording: true,
};

const recruitmentInterview = {
  id: "44444444-4444-4444-8444-444444444442",
  type: "recruitment",
  owner: {
    id: "66666666-6666-4666-8666-666666666662",
    name: "Riley Candidate",
    email: "riley@example.com",
  },
  job_posting: {
    id: "55555555-5555-4555-8555-555555555555",
    title: "Backend Engineer",
  },
  score: null,
  has_report: false,
  has_recording: false,
};

const practiceDetail = {
  ...practiceInterview,
  role_profile: {
    id: "77777777-7777-4777-8777-777777777771",
    title: "Frontend Developer",
  },
  application_id: null,
  avatar: { id: "88888888-8888-4888-8888-888888888881", name: "Nova" },
  voice: { id: "33333333-3333-4333-8333-333333333331", name: "Ava" },
  question_count: 5,
  turn_count: 12,
  // The API never returns these; the screen must not render them if they ever arrive.
  record_path: "recordings/secret-practice.webm",
  transcript: "secret transcript body",
};

async function mockAdminInterviews(page: Page) {
  const listTypes: Array<string | null> = [];
  const interviews = [practiceInterview, recruitmentInterview];
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
      return route.fulfill({ headers, json: { success: true, data: admin } });
    if (url.pathname === "/admin/interviews" && method === "GET") {
      const type = url.searchParams.get("type");
      listTypes.push(type);
      const items = interviews.filter((row) => !type || row.type === type);
      return route.fulfill({
        headers,
        json: {
          success: true,
          data: { items, total: items.length, page: 1, page_size: 20 },
        },
      });
    }
    if (
      url.pathname === `/admin/interviews/${practiceInterview.id}` &&
      method === "GET"
    ) {
      return route.fulfill({
        headers,
        json: { success: true, data: practiceDetail },
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
  return { listTypes };
}

test("admin sees interview sessions and filters them by type", async ({
  page,
}) => {
  const { listTypes } = await mockAdminInterviews(page);
  await page.goto("/admin/interviews");
  await expect(
    page.getByRole("heading", { level: 1, name: "Interview sessions" }),
  ).toBeVisible();

  const table = page.getByRole("table", { name: "Interview sessions" });
  await expect(
    table.getByRole("row", { name: /Casey Candidate/ }),
  ).toBeVisible();
  await expect(
    table.getByRole("row", { name: /Riley Candidate/ }),
  ).toBeVisible();
  await expect(page.getByText("2 interview sessions")).toBeVisible();
  expect(listTypes[0]).toBeNull();

  await page.getByRole("combobox", { name: "Interview type" }).click();
  await page.getByRole("option", { name: "Practice" }).click();
  await page.getByRole("button", { name: "Apply filters" }).click();

  await expect(page.getByText("1 matching interview session")).toBeVisible();
  await expect(table.getByRole("row", { name: /Riley Candidate/ })).toHaveCount(
    0,
  );
  await expect(
    table.getByRole("row", { name: /Casey Candidate/ }),
  ).toBeVisible();
  await expect.poll(() => listTypes.at(-1)).toBe("practice");
});

test("admin opens an interview and sees operational fields only", async ({
  page,
}) => {
  await mockAdminInterviews(page);
  await page.goto("/admin/interviews");
  await page
    .getByRole("link", {
      name: `View details for interview ${practiceInterview.id}`,
    })
    .click();

  await expect(page).toHaveURL(
    new RegExp(`/admin/interviews/${practiceInterview.id}$`),
  );
  const detail = page.getByRole("region", {
    name: "Interview session detail",
  });
  await expect(
    detail.getByRole("heading", { level: 1, name: "Interview session" }),
  ).toBeVisible();
  await expect(detail.getByText("Frontend Developer")).toBeVisible();
  await expect(detail.getByText("Recorded")).toBeVisible();
  await expect(detail.getByText("Casey Candidate")).toBeVisible();

  await expect(detail.getByRole("button")).toHaveCount(0);
  await expect(detail.getByRole("link")).toHaveText(
    "Back to interview sessions",
  );
  await expect(
    detail.getByRole("link", { name: /record|transcript|download/i }),
  ).toHaveCount(0);
  await expect(page.getByText("secret transcript body")).toHaveCount(0);
});

test("admin sidebar lists only the confirmed Admin screens", async ({
  page,
}) => {
  await mockAdminInterviews(page);
  await page.goto("/admin/interviews");
  const nav = page.getByRole("navigation", {
    name: "Administrator navigation",
  });
  await expect(nav.getByRole("link")).toHaveText([
    "Dashboard",
    "Accounts",
    "Job Posting Moderation",
    "Interview Sessions",
    "Voice Profiles",
    "Payment Transactions",
    "Revenue Reports",
  ]);
  await expect(nav.getByRole("link", { name: "AI Behaviour" })).toHaveCount(0);
  await expect(
    nav.getByRole("link", { name: "Evaluation Criteria" }),
  ).toHaveCount(0);
  await expect(
    nav.getByRole("link", { name: "Interview Configuration" }),
  ).toHaveCount(0);
});
