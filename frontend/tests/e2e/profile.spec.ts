import { test, expect, type Page } from "@playwright/test";

const id = "11111111-1111-4111-8111-111111111111";
const initial = {
  id,
  full_name: "Alex Doe",
  email: "alex@example.com",
  image: null as string | null,
  role: "candidate" as "candidate" | "recruiter" | "admin",
  email_verified: true,
  company_name: null as string | null,
  company_website: null as string | null,
  created_at: "2026-10-07T16:00:00Z",
  updated_at: "2026-10-07T16:00:00Z",
};
async function mockProfile(
  page: Page,
  role: typeof initial.role = "candidate",
) {
  let profile = { ...initial, role };
  let failSave = false;
  let failRead = false;
  let sessionStatus = 200;
  let roleChangedDuringSave = false;
  let calls = 0;
  await page.route("http://localhost:8080/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    const headers = {
      "access-control-allow-origin": "http://127.0.0.1:3100",
      "access-control-allow-credentials": "true",
      "access-control-allow-methods": "GET,PATCH,OPTIONS",
      "access-control-allow-headers": "Content-Type",
    };
    if (route.request().method() === "OPTIONS") {
      await route.fulfill({ status: 204, headers });
      return;
    }
    if (path === "/auth/me") {
      await route.fulfill({
        status: sessionStatus,
        headers,
        json:
          sessionStatus === 200
            ? { success: true, data: { ...profile, is_locked: false } }
            : {
                success: false,
                error: { code: "INVALID_TOKEN", message: "Session expired" },
              },
      });
      return;
    }
    if (path === "/profile") {
      if (route.request().method() === "PATCH") {
        calls++;
        if (roleChangedDuringSave) {
          roleChangedDuringSave = false;
          profile = { ...profile, role: "recruiter", company_name: "Acme" };
          await route.fulfill({
            status: 403,
            headers,
            json: {
              success: false,
              error: { code: "FORBIDDEN", message: "Role changed during save" },
            },
          });
          return;
        }
        if (failSave) {
          await route.fulfill({
            status: 500,
            headers,
            json: {
              success: false,
              error: {
                code: "INTERNAL_ERROR",
                message: "Unable to save profile",
              },
            },
          });
          return;
        }
        if (sessionStatus !== 200) {
          await route.fulfill({
            status: 401,
            headers,
            json: {
              success: false,
              error: { code: "INVALID_TOKEN", message: "Session expired" },
            },
          });
          return;
        }
        profile = {
          ...profile,
          ...route.request().postDataJSON(),
          updated_at: "2026-10-08T00:00:00Z",
        };
      } else if (failRead) {
        await route.fulfill({
          status: 500,
          headers,
          json: {
            success: false,
            error: { code: "INTERNAL_ERROR", message: "Read failed" },
          },
        });
        return;
      }
      await route.fulfill({ headers, json: { success: true, data: profile } });
      return;
    }
    await route.fulfill({
      status: 404,
      headers,
      json: {
        success: false,
        error: { code: "NOT_FOUND", message: "Not found" },
      },
    });
  });
  return {
    failSave: () => {
      failSave = true;
    },
    failRead: (value: boolean) => {
      failRead = value;
    },
    expire: () => {
      sessionStatus = 401;
    },
    calls: () => calls,
    rejectSaveAfterRoleChange: () => {
      roleChangedDuringSave = true;
    },
  };
}
test("a 403 save recovers after the session and profile confirm the new role", async ({
  page,
}) => {
  const api = await mockProfile(page);
  await page.goto("/profile");
  await page.getByLabel("Display name").fill("Rejected draft");
  api.rejectSaveAfterRoleChange();
  await page.getByRole("button", { name: "Save changes" }).click();
  await expect(page.getByLabel("Company name", { exact: true })).toHaveValue(
    "Acme",
  );
  await expect(page.getByLabel("Display name")).toHaveValue("Alex Doe");
  expect(api.calls()).toBe(1);
  await page.getByLabel("Display name").fill("Reviewed name");
  await page.getByRole("button", { name: "Save changes" }).click();
  await expect(
    page.getByRole("form", { name: "Edit your profile" }).getByRole("status"),
  ).toHaveText(/Your profile has been saved/);
  await page.reload();
  await expect(page.getByLabel("Display name")).toHaveValue("Reviewed name");
});
for (const role of ["candidate", "recruiter", "admin"] as const) {
  test(`${role} own profile saves and refreshes in its role shell`, async ({
    page,
  }) => {
    await mockProfile(page, role);
    await page.goto("/profile");
    await expect(
      page.getByRole("heading", { name: "Your profile", exact: true }),
    ).toBeVisible();
    await expect(
      page.getByRole("navigation", {
        name: `${role === "admin" ? "Administrator" : role === "recruiter" ? "Recruiter" : "Candidate"} navigation`,
      }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Save changes" }),
    ).toBeDisabled();
    await page.getByLabel("Display name").fill("Updated Alex");
    if (role === "recruiter") {
      await page.getByLabel("Company name", { exact: true }).fill("Acme");
      await page
        .getByLabel("Company website", { exact: true })
        .fill("https://acme.example");
    } else
      await expect(
        page.getByLabel("Company name", { exact: true }),
      ).toHaveCount(0);
    await page.getByRole("button", { name: "Save changes" }).click();
    await expect(
      page.getByRole("form", { name: "Edit your profile" }).getByRole("status"),
    ).toHaveText(/Your profile has been saved/);
    await expect(
      page.getByRole("button", { name: "Open account menu" }),
    ).toContainText("Updated Alex");
    await page.reload();
    await expect(page.getByLabel("Display name")).toHaveValue("Updated Alex");
    if (role === "recruiter")
      await expect(
        page.getByLabel("Company name", { exact: true }),
      ).toHaveValue("Acme");
  });
}
test("failed save keeps draft, discard restores baseline and validation blocks invalid avatar", async ({
  page,
}) => {
  const api = await mockProfile(page);
  api.failSave();
  await page.goto("/profile");
  await page.getByLabel("Display name").fill("Unsaved draft");
  await page.getByRole("button", { name: "Save changes" }).click();
  await expect(
    page.getByRole("form", { name: "Edit your profile" }).getByRole("alert"),
  ).toHaveText("Unable to save profile");
  await expect(page.getByLabel("Display name")).toHaveValue("Unsaved draft");
  await page.getByRole("button", { name: "Discard" }).click();
  await expect(page.getByLabel("Display name")).toHaveValue("Alex Doe");
  await page.getByLabel("Avatar URL").fill("http://example.com/avatar.png");
  await expect(
    page.getByRole("alert").filter({ hasText: "HTTPS image" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Save changes" }),
  ).toBeDisabled();
  expect(api.calls()).toBe(1);
});
test("initial load error has retry and no editable placeholder data", async ({
  page,
}) => {
  const api = await mockProfile(page);
  api.failRead(true);
  await page.goto("/profile");
  await expect(page.getByRole("button", { name: "Retry" })).toBeVisible();
  await expect(page.getByLabel("Display name")).toHaveCount(0);
  api.failRead(false);
  await page.getByRole("button", { name: "Retry" }).click();
  await expect(page.getByLabel("Display name")).toHaveValue("Alex Doe");
});
test("expired session during save hides profile and returns to login", async ({
  page,
}) => {
  const api = await mockProfile(page);
  await page.goto("/profile");
  await page.getByLabel("Display name").fill("Draft");
  api.expire();
  await page.getByRole("button", { name: "Save changes" }).click();
  await expect(page).toHaveURL(/\/login$/);
  await expect(page.getByLabel("Display name")).toHaveCount(0);
});
for (const width of [390, 768, 1440]) {
  test(`profile is accessible without overflow at ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 900 });
    await mockProfile(page, "recruiter");
    await page.goto("/profile");
    await expect(
      page.getByLabel("Company website", { exact: true }),
    ).toBeVisible();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    ).toBe(true);
    await page.getByLabel("Display name").focus();
    await page.keyboard.press("ControlOrMeta+A");
    await page.keyboard.type("Keyboard user");
    await page.keyboard.press("Tab");
    await expect(page.getByLabel("Avatar URL")).toBeFocused();
  });
}
test("failed avatar falls back to Peek in profile and sidebar", async ({
  page,
}) => {
  await mockProfile(page);
  await page.route("https://avatar.example/**", (route) =>
    route.fulfill({ status: 404, body: "missing" }),
  );
  await page.goto("/profile");
  await page
    .getByLabel("Avatar URL")
    .fill("https://avatar.example/missing.png");
  await page.getByRole("button", { name: "Save changes" }).click();
  await expect(
    page.getByRole("form", { name: "Edit your profile" }).getByRole("status"),
  ).toHaveText(/Your profile has been saved/);
  await expect(page.locator("span[aria-hidden] > svg")).toHaveCount(2);
  await expect(
    page.locator('img[src="https://avatar.example/missing.png"]'),
  ).toHaveCount(0);
});

test("Peek avatars animate and stop when reduced motion is enabled", async ({
  page,
}) => {
  await page.emulateMedia({ reducedMotion: "no-preference" });
  await mockProfile(page);
  await page.goto("/profile");
  const avatar = page.locator("main span[aria-hidden] > svg");
  await expect(avatar).toBeVisible();
  const before = await avatar.innerHTML();
  await expect.poll(() => avatar.innerHTML()).not.toBe(before);
  await page.emulateMedia({ reducedMotion: "reduce" });
  // Let the media change event and its static pose reach the next paint.
  await page.evaluate(
    () =>
      new Promise<void>((resolve) => {
        requestAnimationFrame(() => requestAnimationFrame(() => resolve()));
      }),
  );
  const still = await avatar.innerHTML();
  // Sampling across frames verifies the animation really stops.
  await page.waitForTimeout(300);
  expect(await avatar.innerHTML()).toBe(still);
});
