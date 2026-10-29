import { expect, test, type Page } from "@playwright/test";

type Role = "candidate" | "recruiter" | "admin";
const recruiterId = "22222222-2222-4222-8222-222222222222";
const avatarId = "55555555-5555-4555-8555-555555555555";
const voiceId = "66666666-6666-4666-8666-666666666666";
const id = (suffix: number) =>
  `33333333-3333-4333-8333-${String(suffix).padStart(12, "0")}`;
const createdAt = "2026-10-12T08:00:00Z";
const headers = {
  "access-control-allow-origin": "http://127.0.0.1:3100",
  "access-control-allow-credentials": "true",
  "access-control-allow-methods": "GET,POST,OPTIONS",
  "access-control-allow-headers": "Content-Type",
};
const user = (role: Role) => ({
  id: recruiterId,
  full_name: "Product Reviewer",
  email: "reviewer@example.test",
  image: null,
  role,
  email_verified: true,
  is_locked: false,
  onboarding_role_selected: true,
  onboarding_completed: true,
});
const summary = (
  suffix: number,
  title: string,
  status: string,
  slot: number | null,
  submitted = true,
) => ({
  id: id(suffix),
  title,
  status,
  reject_reason: status === "rejected" ? "Clarify responsibilities." : null,
  location: "Ho Chi Minh City",
  employment_type: "Full-time",
  interview_slot: slot,
  core_question_count: submitted ? 50 : 0,
  review_submitted: submitted,
  created_at: createdAt,
});
const postings = [
  summary(1, "Approved backend role awaiting publication", "open", null),
  summary(2, "Live backend role", "open", 2),
  summary(3, "Role with closed intake", "intake_closed", 2),
  summary(4, "Draft backend role", "pending", null, false),
  summary(5, "Backend role pending moderation", "pending", null),
];
const avatar = {
  id: avatarId,
  name: "Linh · Technical interviewer",
  personality: "Calm and precise",
  created_at: createdAt,
  updated_at: createdAt,
};
const recruiter = {
  id: recruiterId,
  name: "Product Reviewer",
  email: "reviewer@example.test",
};
const weights = {
  configured: false,
  using_defaults: true,
  total: 100,
  weights: [
    {
      metric_id: "77777777-7777-4777-8777-777777777771",
      name: "Technical correctness",
      percentage: 60,
    },
    {
      metric_id: "77777777-7777-4777-8777-777777777772",
      name: "Communication",
      percentage: 40,
    },
  ],
};

async function mockWorkspace(page: Page, role: Role = "recruiter") {
  const queries: URLSearchParams[] = [];
  const previews: string[] = [];
  await page.route("http://localhost:8080/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    if (request.method() === "OPTIONS")
      return route.fulfill({ status: 204, headers });
    const ok = (data: unknown) =>
      route.fulfill({ headers, json: { success: true, data } });
    const pageOf = (items: unknown[], total = items.length, limit = 20) =>
      ok({ items, pagination: { total, limit, offset: 0 } });
    if (url.pathname === "/auth/me") return ok(user(role));
    if (url.pathname === "/wallet")
      return ok({ id: "wallet-reviewer", user_id: recruiterId, balance: 200 });
    if (url.pathname === "/voices")
      return pageOf([{ id: voiceId, name: "Ava" }]);
    if (url.pathname === "/inventory/avatars") return pageOf([avatar]);
    if (url.pathname === `/inventory/avatars/${avatarId}`) {
      previews.push(url.pathname);
      return ok({
        ...avatar,
        model_access: {
          url: "https://assets.example.test/linh.vrm",
          method: "GET",
          headers: {},
          expires_at: "2099-10-29T12:05:00Z",
        },
      });
    }
    if (url.pathname === "/job-postings") {
      queries.push(url.searchParams);
      const status = url.searchParams.get("status");
      const funded = url.searchParams.get("funded");
      const submitted = url.searchParams.get("review_submitted");
      const selected = postings.filter(
        (posting) =>
          (!status || posting.status === status) &&
          (funded === null ||
            String(posting.interview_slot !== null) === funded) &&
          (submitted === null ||
            String(posting.review_submitted) === submitted),
      );
      return pageOf(selected);
    }
    if (url.pathname === "/admin/job-postings")
      return pageOf(
        [
          {
            id: id(5),
            title: "Backend role pending moderation",
            status: "pending",
            recruiter,
            created_at: createdAt,
          },
        ],
        7,
        1,
      );
    if (url.pathname === `/admin/job-postings/${id(5)}`)
      return ok({
        ...postings[4],
        recruiter,
        company_name: "Cog Forge",
        company_website: "https://cogforge.example",
        seniority_level: "senior",
        refinement_note: "Focus on production debugging.",
        salary_min: 1500,
        salary_max: 2500,
        interview_duration: 30,
        required_avatar: { id: avatarId, name: avatar.name },
        required_voice: { id: voiceId, name: "Ava" },
        content_path: "",
        document_url: null,
        document_expires_at: null,
        raw_text: "Build reliable Go services with PostgreSQL.",
        requirements: [
          { name: "Go", target: "programming_language", type: "required" },
          { name: "PostgreSQL", target: "database", type: "preferred" },
        ],
        weights_configured: true,
        evaluation: { ...weights, configured: true, using_defaults: false },
      });
    if (url.pathname === "/job-board") {
      queries.push(url.searchParams);
      const board = [
        {
          id: id(2),
          title: "Live backend role",
          company_name: "Cog Forge",
          location: "Ho Chi Minh City",
          employment_type: "Full-time",
          salary_min: 1500,
          salary_max: 2500,
          seniority_level: "senior",
          created_at: createdAt,
          published_at: "2026-10-29T08:00:00Z",
        },
      ];
      return pageOf(
        board,
        url.searchParams.has("published_from") ? 3 : 1,
        Number(url.searchParams.get("limit") ?? 20),
      );
    }
    return route.fulfill({
      status: 404,
      headers,
      json: {
        success: false,
        error: { code: "NOT_FOUND", message: "Unmocked test endpoint" },
      },
    });
  });
  return { queries, previews };
}

test("recruiter distinguishes approved, live and intake-closed postings and filters the server dataset", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  const api = await mockWorkspace(page);
  await page.goto("/recruiter/job-postings");

  const approved = page.getByRole("row").filter({
    has: page.getByRole("link", { name: postings[0].title, exact: true }),
  });
  await expect(approved).toContainText("Approved · Funding required");
  await expect(
    approved.getByRole("link", { name: `Fund & publish ${postings[0].title}` }),
  ).toHaveAttribute(
    "href",
    `/recruiter/job-postings/${id(1)}#interview-capacity`,
  );
  await expect(
    page.getByRole("row").filter({ hasText: postings[1].title }),
  ).toContainText("Live");
  await expect(
    page.getByRole("row").filter({ hasText: postings[2].title }),
  ).toContainText("Intake closed");

  const chooseStatus = async (name: string) => {
    await page.getByRole("combobox", { name: "Status" }).click();
    await page.getByRole("option", { name, exact: true }).click();
  };
  await chooseStatus("Needs funding");
  await expect.poll(() => api.queries.at(-1)?.get("funded")).toBe("false");
  expect(api.queries.at(-1)?.get("status")).toBe("open");
  await expect(
    page.getByRole("link", { name: postings[1].title, exact: true }),
  ).toHaveCount(0);
  await chooseStatus("Live");
  await expect.poll(() => api.queries.at(-1)?.get("funded")).toBe("true");
  await expect(
    page.getByRole("link", { name: postings[1].title, exact: true }),
  ).toBeVisible();
  await chooseStatus("Preparing");
  await expect
    .poll(() => api.queries.at(-1)?.get("review_submitted"))
    .toBe("false");
  expect(api.queries.at(-1)?.get("status")).toBe("pending");
});

test("Created header and dates share an alignment and two-line rows stay vertically centered", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  await mockWorkspace(page);
  await page.goto("/recruiter/job-postings");

  const header = page.getByRole("columnheader", {
    name: "Created",
    exact: true,
  });
  const row = page.getByRole("row").filter({
    has: page.getByRole("link", { name: postings[0].title, exact: true }),
  });
  const date = row.locator("time");
  await expect(header).toBeVisible();
  await expect(date).toBeVisible();
  const headerBox = await header.boundingBox();
  const dateBox = await date.boundingBox();
  const rowBox = await row.boundingBox();
  expect(headerBox).not.toBeNull();
  expect(dateBox).not.toBeNull();
  expect(rowBox).not.toBeNull();
  expect(Math.abs(headerBox!.x - dateBox!.x)).toBeLessThanOrEqual(2);
  expect(
    Math.abs(dateBox!.y + dateBox!.height / 2 - rowBox!.y - rowBox!.height / 2),
  ).toBeLessThanOrEqual(2);
});

test("recruiter statuses remain readable at 390px without horizontal overflow", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await mockWorkspace(page);
  await page.goto("/recruiter/job-postings");

  await expect(
    page.getByText("Approved · Funding required", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("row").filter({ hasText: postings[1].title }),
  ).toContainText("Live");
  await expect(
    page.getByText("Intake closed", { exact: true }).last(),
  ).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
});

test("new posting gives input examples and the selected model has a usable 3D fallback", async ({
  page,
}) => {
  await page.addInitScript(() => {
    HTMLCanvasElement.prototype.getContext = () => null;
  });
  await page.setViewportSize({ width: 1440, height: 1000 });
  const api = await mockWorkspace(page);
  await page.goto("/recruiter/job-postings/new");

  await expect(page.getByLabel("Title (required)")).toHaveAttribute(
    "placeholder",
    "e.g. Senior Backend Engineer",
  );
  await expect(page.getByLabel("Location", { exact: true })).toHaveAttribute(
    "placeholder",
    "e.g. Ho Chi Minh City or Remote",
  );
  await expect(
    page.getByRole("heading", { name: "Candidates allowed into interview" }),
  ).toHaveCount(0);
  await expect(
    page.getByText(/funding interview capacity isn't available yet/i),
  ).toHaveCount(0);
  const selector = page.getByRole("combobox", {
    name: "Interviewer model (required)",
  });
  await selector.click();
  await page.getByRole("option", { name: avatar.name }).click();
  const preview = page.getByRole("region", {
    name: "Selected interviewer preview",
  });
  await expect(preview.getByRole("alert")).toContainText(
    "3D preview isn't available on this device or browser.",
  );
  await expect(selector).toContainText(avatar.name);
  await expect.poll(() => api.previews.length).toBe(1);
});

test("admin sees the complete read-only dossier and a total pending badge", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  await mockWorkspace(page, "admin");
  await page.goto(`/admin/job-postings/${id(5)}`);

  await expect(
    page.getByRole("link", { name: "Job Posting Moderation", exact: true }),
  ).toHaveAccessibleDescription("7 job postings waiting for review");
  const requirements = page.getByRole("region", {
    name: "Confirmed requirements",
  });
  await expect(requirements.getByText("Go", { exact: true })).toBeVisible();
  await expect(
    requirements.getByText("PostgreSQL", { exact: true }),
  ).toBeVisible();
  await expect(
    requirements.getByText("Focus on production debugging."),
  ).toBeVisible();
  const evaluation = page.getByRole("region", { name: "Evaluation criteria" });
  await expect(evaluation).toContainText("Custom evaluation criteria");
  await expect(evaluation).toContainText("60%");
  await expect(evaluation.getByRole("textbox")).toHaveCount(0);
  await expect(page.getByText("Cog Forge", { exact: true })).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Approve", exact: true }),
  ).toBeEnabled();
  await expect(
    page.getByRole("button", { name: "Reject", exact: true }),
  ).toBeEnabled();
});

test("candidate sidebar badge uses a global dated count rather than visible rows", async ({
  page,
}) => {
  await mockWorkspace(page, "candidate");
  await page.goto("/jobs");

  await expect(
    page.getByRole("link", { name: "Job Board", exact: true }).first(),
  ).toHaveAccessibleDescription("3 job postings new today");
  await expect(page.getByText(/new today/)).toHaveCount(1);
  await expect(page.getByLabel("Job title")).toHaveAttribute(
    "placeholder",
    "e.g. Backend Engineer",
  );
  await expect(
    page.getByRole("link", { name: "Live backend role", exact: true }),
  ).toBeVisible();
});
