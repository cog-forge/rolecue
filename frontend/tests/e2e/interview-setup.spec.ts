import { expect, test, type Page, type Route } from "@playwright/test";

// KAN-51: setup ends in exactly one real operation, a start or a resume, then the room URL. Every
// API call is mocked here; nothing reaches a real wallet.
test.use({
  launchOptions: {
    executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH,
    args: [
      "--use-fake-device-for-media-stream",
      "--use-fake-ui-for-media-stream",
    ],
  },
  permissions: ["microphone"],
});

const candidate = {
  id: "11111111-1111-4111-8111-111111111111",
  full_name: "Cam Candidate",
  email: "cam@example.com",
  image: null,
  role: "candidate",
  email_verified: true,
  is_locked: false,
  onboarding_role_selected: true,
  onboarding_completed: true,
};
// The Go API's CORS answer for the page's own origin. The start sends Idempotency-Key, so the
// preflight must allow it, as the API's default cors.allow_headers does.
const corsHeaders = (route: Route) => ({
  "access-control-allow-origin":
    route.request().headers().origin ?? "http://127.0.0.1:3100",
  "access-control-allow-credentials": "true",
  "access-control-allow-methods": "GET,POST,OPTIONS",
  "access-control-allow-headers": "Content-Type, Idempotency-Key",
});

const profileId = "22222222-2222-4222-8222-000000000001";
const draftId = "22222222-2222-4222-8222-000000000002";
const miaId = "44444444-4444-4444-8444-000000000001";
const avaId = "55555555-5555-4555-8555-000000000001";
const sessionId = "66666666-6666-4666-8666-000000000001";
const applicationId = "77777777-7777-4777-8777-000000000001";
const uuidPattern =
  /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

function session(over: Record<string, unknown> = {}) {
  return {
    id: sessionId,
    type: "practice",
    role_profile_id: profileId,
    application_id: null,
    avatar_id: null,
    voice_id: null,
    started_at: new Date(Date.now() - 20 * 60_000).toISOString(),
    completed_at: null,
    evaluation_status: "pending",
    question_count: 8,
    answered_count: 3,
    current_question: { position: 4, content: "Hidden from setup." },
    turns: [],
    ...over,
  };
}

const roleProfile = {
  id: profileId,
  title: "Frontend Engineer – Design System",
  seniorityLevel: "mid",
  requirements: [],
  refinementNote: "",
  rawText: "Build the design system.",
  hasOriginalDocument: false,
  status: "confirmed",
  createdAt: "2026-10-20T08:00:00Z",
};

type Start = { body: unknown; key: string | null; path: string };
type Options = {
  balance?: number;
  unfinished?: boolean;
  start?: (attempt: number) => { status: number; body: unknown };
  application?: Record<string, unknown>;
};

async function mockApi(page: Page, options: Options = {}) {
  const starts: Start[] = [];
  const reply = (route: Route, data: unknown, status = 200) =>
    route.fulfill({
      status,
      headers: corsHeaders(route),
      json: status < 300 ? { success: true, data } : data,
    });
  await page.route("http://localhost:8080/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname;
    const method = request.method();
    if (method === "OPTIONS")
      return route.fulfill({ status: 204, headers: corsHeaders(route) });
    if (path === "/auth/me") return reply(route, candidate);
    if (path === "/wallet")
      return reply(route, {
        id: "wallet",
        user_id: candidate.id,
        balance: options.balance ?? 9,
      });
    if (path === "/role-profiles")
      return reply(route, {
        items: [
          {
            id: profileId,
            title: roleProfile.title,
            seniorityLevel: "mid",
            status: "confirmed",
            createdAt: roleProfile.createdAt,
          },
          {
            id: draftId,
            title: "Data Engineer",
            seniorityLevel: null,
            status: "review",
            createdAt: roleProfile.createdAt,
          },
        ],
        pagination: { limit: 20, offset: 0, total: 2 },
      });
    if (path === `/role-profiles/${profileId}`)
      return reply(route, roleProfile);
    if (path === "/inventory/avatars")
      return reply(route, {
        items: [
          {
            id: miaId,
            name: "Mia",
            personality: "Calm and precise; asks about trade-offs.",
            created_at: "2026-10-20T08:00:00Z",
            updated_at: "2026-10-20T08:00:00Z",
          },
        ],
        pagination: { limit: 12, offset: 0, total: 1 },
      });
    if (path === `/inventory/avatars/${miaId}`)
      return reply(route, {
        id: miaId,
        name: "Mia",
        personality: "Calm and precise; asks about trade-offs.",
        created_at: "2026-10-20T08:00:00Z",
        updated_at: "2026-10-20T08:00:00Z",
        model_access: {
          // Answered with 404 below: the portrait falls back, the name still shows.
          url: "http://localhost:8080/models/mia.vrm",
          method: "GET",
          headers: {},
          expires_at: new Date(Date.now() + 300_000).toISOString(),
        },
      });
    if (path === "/voices")
      return reply(route, {
        items: [
          { id: avaId, name: "Ava – English" },
          {
            id: "55555555-5555-4555-8555-000000000002",
            name: "Minh – Vietnamese",
          },
        ],
        pagination: { limit: 100, offset: 0, total: 2 },
      });
    if (path === "/interviews" && method === "GET")
      return reply(route, {
        items: options.unfinished
          ? [
              {
                id: sessionId,
                type: "practice",
                started_at: session().started_at,
                completed_at: null,
                evaluation_status: "pending",
                status: "in_progress",
                score: null,
                role_profile_title: roleProfile.title,
                job_posting_title: null,
                application_id: null,
              },
            ]
          : [],
        total: options.unfinished ? 1 : 0,
        page: 1,
        page_size: 100,
      });
    if (path === `/interviews/${sessionId}` && method === "GET")
      return reply(route, session());
    if (path === `/applications/${applicationId}`)
      return reply(route, {
        id: applicationId,
        job_posting_id: "88888888-8888-4888-8888-000000000001",
        job_posting_title: "Go Backend Engineer",
        status: "interview_eligible",
        reject_reason: null,
        interview_deadline: new Date(Date.now() + 5 * 3_600_000).toISOString(),
        can_start_interview: true,
        created_at: "2026-10-27T08:00:00Z",
        interview: null,
        ...options.application,
      });
    if (method === "POST" && path.startsWith("/interviews")) {
      starts.push({
        body: request.postDataJSON() ?? null,
        key: request.headers()["idempotency-key"] ?? null,
        path,
      });
      const result = options.start?.(starts.length) ?? {
        status: 201,
        body: session({ answered_count: 0 }),
      };
      return reply(route, result.body, result.status);
    }
    return reply(
      route,
      { success: false, error: { code: "NOT_FOUND", message: "unmocked" } },
      404,
    );
  });
  return { starts };
}

async function testMicrophone(page: Page) {
  await page
    .getByRole("button", { name: "Test microphone", exact: true })
    .click();
  await expect(
    page.getByText("Microphone ready", { exact: true }),
  ).toBeVisible();
}

test("practice: choose a Role Profile, an avatar and a voice, check the microphone, then start", async ({
  page,
}) => {
  const { starts } = await mockApi(page);
  await page.goto("/interviews/new/setup");

  const profiles = page.getByRole("radiogroup", {
    name: "Confirmed Role Profiles",
  });
  await expect(profiles.getByRole("radio")).toHaveCount(1);
  await expect(page.getByText("Data Engineer")).toHaveCount(0);
  await page.getByRole("radio", { name: /frontend engineer/i }).click();
  await expect(page).toHaveURL(new RegExp(`roleProfileId=${profileId}`));
  await page.getByRole("link", { name: /continue to interviewer/i }).click();

  await expect(
    page.getByRole("heading", { level: 1, name: "Choose your interviewer" }),
  ).toBeVisible();
  await expect(
    page.getByRole("radio", { name: /2d interviewer/i }),
  ).toBeChecked();
  await page.getByRole("radio", { name: /mia/i }).click();
  await page.getByRole("combobox", { name: "Voice" }).click();
  await page.getByRole("option", { name: "Ava – English" }).click();
  await page.getByRole("button", { name: /continue to preflight/i }).click();

  await expect(page).toHaveURL(
    `/interviews/new/preflight?roleProfileId=${profileId}&avatarId=${miaId}&voiceId=${avaId}`,
  );
  const summary = page.getByRole("region", { name: "Your interview" });
  await expect(summary).toContainText(roleProfile.title);
  await expect(summary).toContainText("Mia");
  await expect(summary).toContainText("Ava – English");
  await expect(page.getByText("9 coins")).toBeVisible();
  const start = page.getByRole("button", { name: "Start interview · 5 coins" });
  await expect(start).toBeDisabled();
  await expect(
    page.getByText("Turn on your microphone to start."),
  ).toBeVisible();

  await testMicrophone(page);
  await page.getByRole("button", { name: "Mute microphone" }).click();
  await expect(start).toBeDisabled();
  await page.getByRole("button", { name: "Unmute microphone" }).click();
  await expect(start).toBeEnabled();
  await start.click();

  await expect(page).toHaveURL(`/interviews/${sessionId}/room`);
  expect(starts).toHaveLength(1);
  expect(starts[0].path).toBe("/interviews");
  expect(starts[0].body).toEqual({
    type: "practice",
    role_profile_id: profileId,
    avatar_id: miaId,
    voice_id: avaId,
  });
  expect(starts[0].key).toMatch(uuidPattern);
});

test("practice: an unfinished interview resumes for free through the resume endpoint", async ({
  page,
}) => {
  const { starts } = await mockApi(page, {
    unfinished: true,
    start: () => ({ status: 200, body: session() }),
  });
  await page.goto(`/interviews/new/setup?roleProfileId=${profileId}`);
  await expect(
    page.getByText("Unfinished interview", { exact: true }),
  ).toBeVisible();
  const callout = page.getByRole("region", { name: "Unfinished interview" });
  await expect(callout).toContainText("3 of 8");
  await expect(callout).toContainText("Resuming is free");
  await callout.getByRole("link", { name: /resume interview/i }).click();

  await expect(
    page.getByRole("heading", { level: 1, name: "Resume your interview" }),
  ).toBeVisible();
  await expect(page.getByText("No coins are charged to resume.")).toBeVisible();
  await expect(
    page.getByRole("button", { name: /start interview/i }),
  ).toHaveCount(0);
  await expect(page.getByText(/charged once/)).toHaveCount(0);
  await testMicrophone(page);
  await page.getByRole("button", { name: "Resume interview" }).click();

  await expect(page).toHaveURL(`/interviews/${sessionId}/room`);
  expect(starts.map((start) => start.path)).toEqual([
    `/interviews/${sessionId}/resume`,
  ]);
});

test("practice: a refused balance offers Add coins, and an unknown outcome retries with the same key", async ({
  page,
}) => {
  const { starts } = await mockApi(page, {
    start: (attempt) =>
      attempt === 1
        ? { status: 502, body: { error: "Upstream service unavailable" } }
        : {
            status: 422,
            body: {
              success: false,
              error: {
                code: "INSUFFICIENT_BALANCE",
                message: "insufficient balance",
              },
            },
          },
  });
  await page.goto(`/interviews/new/preflight?roleProfileId=${profileId}`);
  await testMicrophone(page);
  const panel = page.getByRole("region", { name: "Practice interview" });
  const start = page.getByRole("button", { name: "Start interview · 5 coins" });
  await start.click();
  await expect(panel.getByRole("alert")).toContainText(
    "You won't be charged twice.",
  );
  await start.click();
  await expect(panel.getByRole("alert")).toContainText("You need 5 coins");
  expect(starts).toHaveLength(2);
  expect(starts[1].key).toBe(starts[0].key);

  await page
    .getByRole("alert")
    .getByRole("button", { name: "Add coins" })
    .click();
  await expect(page.getByRole("dialog")).toBeVisible();
});

test("recruitment: start from the Application with the company's interviewer, for free", async ({
  page,
}) => {
  const { starts } = await mockApi(page, {
    start: (attempt) =>
      attempt === 1
        ? {
            status: 409,
            body: {
              success: false,
              error: { code: "INTERVIEW_CAPACITY_FULL", message: "full" },
            },
          }
        : {
            status: 201,
            body: session({
              type: "recruitment",
              role_profile_id: null,
              application_id: applicationId,
              answered_count: 0,
            }),
          },
  });
  await page.goto(`/applications/${applicationId}`);
  await page.getByRole("link", { name: "Start interview" }).click();

  await expect(page).toHaveURL(
    `/interviews/new/preflight?applicationId=${applicationId}`,
  );
  await expect(
    page.getByRole("navigation", { name: "Interview creation steps" }),
  ).toHaveCount(0);
  await expect(
    page.getByText("Interviewer chosen by the company"),
  ).toBeVisible();
  await expect(
    page.getByText("Free for you. The company covers this interview."),
  ).toBeVisible();
  const panel = page.getByRole("region", { name: "Recruitment interview" });
  await expect(panel.getByText(/coins/)).toHaveCount(0);
  await testMicrophone(page);
  const start = page.getByRole("button", { name: "Start interview" });
  await start.click();
  await expect(panel.getByRole("alert")).toContainText(
    "All interview slots for this posting are taken.",
  );
  await expect(
    page.getByRole("link", { name: "Back to application" }).first(),
  ).toBeVisible();

  await start.click();
  await expect(page).toHaveURL(`/interviews/${sessionId}/room`);
  for (const attempt of starts)
    expect(attempt.body).toEqual({
      type: "recruitment",
      application_id: applicationId,
    });
});

test("setup steps fit a 390 px phone without sideways scrolling", async ({
  page,
}, testInfo) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await mockApi(page);
  for (const [name, url] of [
    ["setup", `/interviews/new/setup?roleProfileId=${profileId}`],
    [
      "interviewer",
      `/interviews/new/interviewer?roleProfileId=${profileId}&avatarId=${miaId}`,
    ],
    [
      "preflight",
      `/interviews/new/preflight?roleProfileId=${profileId}&avatarId=${miaId}&voiceId=${avaId}`,
    ],
    ["recruitment", `/interviews/new/preflight?applicationId=${applicationId}`],
  ] as const) {
    await page.goto(url);
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    await page.waitForLoadState("networkidle");
    const overflow = await page.evaluate(
      () =>
        document.documentElement.scrollWidth -
        document.documentElement.clientWidth,
    );
    expect(overflow, name).toBeLessThanOrEqual(0);
    await page.screenshot({
      path: testInfo.outputPath(`${name}-390.png`),
      fullPage: true,
    });
  }
});
