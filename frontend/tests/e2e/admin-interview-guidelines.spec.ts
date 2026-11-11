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

const defaults = {
  tone: "neutral",
  question_length: "standard",
  follow_up_max: null as number | null,
  follow_up_focus: "standard",
  avoid_topics: [] as string[],
};

type Values = typeof defaults;
const state = (version: number, values: Values, by = "") => ({
  version,
  values,
  defaults,
  allowed: {},
  effective_follow_up_ceiling: 2,
  updated_at: version > 0 ? "2026-11-11T06:00:00Z" : null,
  updated_by_name: by,
});

const headers = {
  "access-control-allow-origin": "http://127.0.0.1:3100",
  "access-control-allow-credentials": "true",
  "access-control-allow-methods": "GET,PUT,OPTIONS",
  "access-control-allow-headers": "Content-Type",
};

// The Go API for /admin/interview-guidelines. `stale` makes the first PUT answer 409 and moves the
// stored version on, as another Admin's save would.
async function mockGuidelines(
  page: Page,
  options: { user?: typeof admin; stale?: boolean } = {},
) {
  const user = options.user ?? admin;
  let stale = options.stale ?? false;
  let current = state(2, { ...defaults, tone: "formal" }, "Grace");
  const saves: unknown[] = [];
  await page.route("http://localhost:8080/**", async (route) => {
    const url = new URL(route.request().url());
    const method = route.request().method();
    if (method === "OPTIONS") return route.fulfill({ status: 204, headers });
    if (url.pathname === "/auth/me")
      return route.fulfill({ headers, json: { success: true, data: user } });
    if (url.pathname === "/admin/interview-guidelines" && method === "GET")
      return route.fulfill({
        headers,
        json: { success: true, data: current },
      });
    if (url.pathname === "/admin/interview-guidelines" && method === "PUT") {
      const body = route.request().postDataJSON() as {
        expected_version: number;
        values: Values;
      };
      saves.push(body);
      if (stale) {
        stale = false;
        current = state(
          current.version + 1,
          { ...defaults, tone: "challenging" },
          "Linus",
        );
        return route.fulfill({
          status: 409,
          headers,
          json: {
            success: false,
            error: {
              code: "GUIDELINE_VERSION_CONFLICT",
              message: "the guidelines changed since they were loaded",
            },
          },
        });
      }
      if (body.expected_version !== current.version)
        return route.fulfill({
          status: 409,
          headers,
          json: {
            success: false,
            error: { code: "GUIDELINE_VERSION_CONFLICT", message: "stale" },
          },
        });
      current = state(current.version + 1, body.values, "Ada Admin");
      return route.fulfill({
        headers,
        json: { success: true, data: current },
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
  return { saves };
}

const heading = (page: Page) =>
  page.getByRole("heading", {
    level: 1,
    name: "Interview Behavior Guidelines",
  });

test("admin saves a new version of the guidelines", async ({
  page,
}, testInfo) => {
  const api = await mockGuidelines(page);
  await page.goto("/admin/interview-guidelines");
  await expect(heading(page)).toBeVisible();
  await expect(page.getByText("Version 2", { exact: true })).toBeVisible();
  await expect(page.getByText(/Saved by Grace on/)).toBeVisible();
  await expect(page.getByRole("radio", { name: /^Formal/ })).toBeChecked();
  const save = page.getByRole("button", { name: "Save changes" });
  await expect(save).toBeDisabled();

  await page.screenshot({
    path: testInfo.outputPath("guidelines-desktop.png"),
    fullPage: true,
  });
  await page.getByRole("radio", { name: /^Friendly/ }).click();
  await page.getByRole("radio", { name: /^Concise/ }).click();
  await page.getByRole("radio", { name: /^1$/ }).click();
  await page.getByRole("checkbox", { name: /Salary and compensation/ }).click();
  await expect(page.getByText("Unsaved changes")).toBeVisible();
  await save.click();

  await expect(
    page.getByText(
      "Saved as version 3. It applies to interviews that start from now on.",
    ),
  ).toBeVisible();
  await expect(page.getByText("Version 3", { exact: true })).toBeVisible();
  expect(api.saves).toEqual([
    {
      expected_version: 2,
      values: {
        tone: "friendly",
        question_length: "concise",
        follow_up_max: 1,
        follow_up_focus: "standard",
        avoid_topics: ["compensation"],
      },
    },
  ]);

  // Reloading shows what the server kept.
  await page.reload();
  await expect(page.getByRole("radio", { name: /^Friendly/ })).toBeChecked();
  await expect(
    page.getByRole("checkbox", { name: /Salary and compensation/ }),
  ).toBeChecked();
});

test("a stale save is refused, the latest version is loaded and the Admin is told", async ({
  page,
}) => {
  const api = await mockGuidelines(page, { stale: true });
  await page.goto("/admin/interview-guidelines");
  await expect(page.getByText("Version 2", { exact: true })).toBeVisible();
  await page.getByRole("radio", { name: /^Friendly/ }).click();
  await page.getByRole("button", { name: "Save changes" }).click();

  await expect(
    page.getByText(/Someone else saved version 3 while you were editing/),
  ).toBeVisible();
  await expect(page.getByText("Version 3", { exact: true })).toBeVisible();
  await expect(page.getByRole("radio", { name: /^Challenging/ })).toBeChecked();
  expect(api.saves).toHaveLength(1);
});

test("reset to defaults is confirmed and saved as a new version", async ({
  page,
}) => {
  const api = await mockGuidelines(page);
  await page.goto("/admin/interview-guidelines");
  await expect(page.getByText("Version 2", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Reset defaults" }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toContainText("saves a new version");
  expect(api.saves).toHaveLength(0);
  await dialog
    .getByRole("button", { name: "Save defaults as a new version" })
    .click();
  await expect(page.getByText(/Defaults saved as version 3/)).toBeVisible();
  await expect(page.getByRole("radio", { name: /^Neutral/ })).toBeChecked();
  expect(api.saves).toEqual([{ expected_version: 2, values: defaults }]);
});

test("the page says which fields only steer the prompt", async ({ page }) => {
  await mockGuidelines(page);
  await page.goto("/admin/interview-guidelines");
  await page.getByRole("button", { name: "How these settings work" }).click();
  await expect(
    page.getByText(/Tone, follow-up focus and avoided topics steer/),
  ).toBeVisible();
  await expect(
    page.getByText(/Length and follow-up limits are enforced by the server/),
  ).toBeVisible();
});

test("a Candidate cannot open the Admin guidelines", async ({ page }) => {
  const api = await mockGuidelines(page, {
    user: { ...admin, role: "candidate" },
  });
  await page.goto("/admin/interview-guidelines");
  await expect(page).not.toHaveURL(/admin\/interview-guidelines/);
  await expect(heading(page)).toHaveCount(0);
  expect(api.saves).toHaveLength(0);
});

test("mobile keeps controls and save actions within the viewport", async ({
  page,
}, testInfo) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.emulateMedia({ reducedMotion: "reduce" });
  await mockGuidelines(page);
  await page.goto("/admin/interview-guidelines");
  await expect(page.getByText("Version 2", { exact: true })).toBeVisible();
  await page.getByRole("radio", { name: /^Friendly/ }).click();
  await expect(
    page.getByRole("button", { name: "Save changes" }),
  ).toBeEnabled();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: testInfo.outputPath("guidelines-mobile.png"),
    fullPage: true,
  });
  await page.getByRole("button", { name: "Save changes" }).click();
  await expect(page.getByText("Version 3", { exact: true })).toBeVisible();
});

test("Vietnamese labels fit mobile and desktop", async ({ page }, testInfo) => {
  await page
    .context()
    .addCookies([
      { name: "NEXT_LOCALE", value: "vi", url: "http://127.0.0.1:3100" },
    ]);
  await page.setViewportSize({ width: 390, height: 844 });
  await mockGuidelines(page);
  await page.goto("/admin/interview-guidelines");
  await expect(
    page.getByRole("heading", {
      name: "Hướng dẫn hành vi phỏng vấn",
      exact: true,
    }),
  ).toBeVisible();
  await page.getByRole("radio", { name: /^Thân thiện/ }).click();
  await expect(
    page.getByRole("button", { name: "Lưu thay đổi" }),
  ).toBeEnabled();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: testInfo.outputPath("guidelines-mobile-vi.png"),
    fullPage: true,
  });
  await page.setViewportSize({ width: 1440, height: 1000 });
  await expect(
    page.getByRole("heading", { name: "Tóm tắt thiết lập" }),
  ).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath("guidelines-desktop-vi.png"),
    fullPage: true,
  });
});
