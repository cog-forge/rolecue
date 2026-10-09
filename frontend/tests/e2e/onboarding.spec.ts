import { test, expect, type Page, type Locator } from "@playwright/test";
async function fixture(
  page: Page,
  role: "candidate" | "recruiter" = "candidate",
  selected = false,
) {
  let user = {
    id: "11111111-1111-4111-8111-111111111111",
    full_name: "New user",
    email: "new@example.com",
    role,
    email_verified: true,
    is_locked: false,
    image: null as string | null,
    onboarding_role_selected: selected,
    onboarding_completed: false,
  };
  let profile = {
    ...user,
    company_name: null as string | null,
    company_website: null as string | null,
    created_at: "2026-10-08T00:00:00Z",
    updated_at: "2026-10-08T00:00:00Z",
  };
  let profilePatchCalls = 0;
  await page.route("**/auth/me", (r) =>
    r.fulfill({ json: { success: true, data: user } }),
  );
  await page.route("**/onboarding/role", async (r) => {
    const body = r.request().postDataJSON();
    user = { ...user, role: body.role, onboarding_role_selected: true };
    profile = { ...profile, ...user };
    await r.fulfill({ json: { success: true, data: user } });
  });
  await page.route("**/profile**", async (r) => {
    if (r.request().method() === "PATCH") {
      profilePatchCalls++;
      const patch = r.request().postDataJSON();
      profile = { ...profile, ...patch, onboarding_completed: true };
      user = { ...user, ...patch, onboarding_completed: true };
    }
    await r.fulfill({ json: { success: true, data: profile } });
  });
  return () => ({ user, profile, profilePatchCalls });
}
test("candidate: mandatory dialog, Peek, completion and reload", async ({
  page,
}) => {
  const state = await fixture(page);
  await page.goto("/dashboard");
  await expect(page.getByRole("dialog")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toBeVisible();
  await expect(
    page.getByRole("navigation", { name: "Candidate navigation" }),
  ).toHaveCount(0);
  await page.getByRole("radio", { name: /^candidate /i }).press("Space");
  await page.getByRole("button", { name: "Continue as candidate" }).click();
  await page.getByLabel("Display name").fill("Alex Candidate");
  await expect(page.getByText(/Peek avatar is ready/)).toBeVisible();
  await page.getByRole("button", { name: "Open my workspace" }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(
    page.getByRole("navigation", { name: "Candidate navigation" }),
  ).toBeVisible();
  expect(state().profile.full_name).toBe("Alex Candidate");
  expect(state().profile.image).toBeNull();
  await page.reload();
  await expect(page.getByRole("dialog")).toHaveCount(0);
});
test("recruiter: resume metadata, website preview, correct sidebar", async ({
  page,
}) => {
  const state = await fixture(page, "recruiter", true);
  await page.goto("/dashboard");
  await expect(page.getByRole("radio")).toHaveCount(0);
  await page.getByLabel("Display name").fill("Riley Recruiter");
  await page.getByLabel("Company name").fill("Acme");
  await page.getByRole("button", { name: "Open my workspace" }).click();
  await expect(page.getByText("Enter your company website.")).toBeVisible();
  await page
    .getByLabel("Company website", { exact: true })
    .fill("acme.example");
  await page.getByLabel("Company website", { exact: true }).press("Tab");
  await expect(
    page
      .getByRole("complementary", { name: "Company website preview" })
      .getByRole("link"),
  ).toHaveAttribute("href", "https://acme.example/");
  expect(state().profilePatchCalls).toBe(0);
  expect(state().user.onboarding_completed).toBe(false);
  await page.getByRole("button", { name: "Open my workspace" }).click();
  await expect(page).toHaveURL(/\/recruiter\/dashboard$/);
  await expect(
    page.getByRole("navigation", { name: "Recruiter navigation" }),
  ).toBeVisible();
  expect(state().profilePatchCalls).toBe(1);
  expect(state().profile.company_website).toBe("https://acme.example");
});
test("mobile: all fields fit and dialog keeps outside click out", async ({
  page,
}) => {
  await page.setViewportSize({ width: 320, height: 720 });
  await fixture(page, "recruiter", true);
  await page.goto("/dashboard");
  await expect(page.getByRole("dialog")).toBeVisible();
  await expect(page.getByLabel("Display name")).toBeVisible();
  const box = await page.getByRole("dialog").boundingBox();
  expect(box?.width).toBeLessThanOrEqual(296);
  expect(box?.x).toBeGreaterThanOrEqual(12);
  await page.mouse.click(2, 2);
  await expect(page.getByRole("dialog")).toBeVisible();
  await page
    .getByRole("button", { name: "Open my workspace" })
    .scrollIntoViewIfNeeded();
  await expect(
    page.getByRole("button", { name: "Open my workspace" }),
  ).toBeVisible();
});

test("expired session while loading metadata returns to login", async ({
  page,
}) => {
  const state = await fixture(page, "candidate", true);
  let authCalls = 0;
  await page.route("**/auth/me", (route) => {
    authCalls++;
    return route.fulfill(
      authCalls === 1
        ? { json: { success: true, data: state().user } }
        : { status: 401, json: { success: false } },
    );
  });
  await page.route("**/profile", (route) =>
    route.fulfill({ status: 401, json: { success: false } }),
  );
  await page.goto("/dashboard");
  await expect(page).toHaveURL(/\/login$/);
  await expect(page.getByRole("dialog")).toHaveCount(0);
});

async function readRgb(element: Locator, property: string) {
  return element.evaluate((element, property) => {
    const context = new OffscreenCanvas(1, 1).getContext("2d");
    if (!context) throw new Error("Unable to read rendered colors");
    context.fillStyle = getComputedStyle(element).getPropertyValue(property);
    context.fillRect(0, 0, 1, 1);
    return Array.from(context.getImageData(0, 0, 1, 1).data).slice(0, 3);
  }, property);
}

async function expectReadableText(text: Locator, surface: Locator) {
  const foreground = await readRgb(text, "color");
  const background = await readRgb(surface, "background-color");
  const luminance = (channels: number[]) => {
    return channels.reduce((total, channel, index) => {
      const value = channel / 255;
      const linear =
        value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
      return total + linear * [0.2126, 0.7152, 0.0722][index];
    }, 0);
  };
  const a = luminance(foreground);
  const b = luminance(background);
  expect(
    (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05),
  ).toBeGreaterThanOrEqual(4.5);
}

for (const theme of ["light", "dark"] as const) {
  test(`${theme} theme: role labels, metadata and preview stay readable`, async ({
    page,
  }) => {
    await page.addInitScript((theme) => {
      localStorage.setItem(
        "rolecue.workspace-preferences",
        JSON.stringify({ state: { theme, collapsed: false }, version: 0 }),
      );
    }, theme);
    await page.route("https://acme.example/favicon.ico", (route) =>
      route.abort(),
    );
    await fixture(page);
    await page.goto("/dashboard");
    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible();
    if (theme === "dark")
      await expect(page.locator("html")).toHaveClass(/dark/);
    else await expect(page.locator("html")).not.toHaveClass(/dark/);
    const frame = dialog.locator('[data-slot="onboarding-form"]');
    const frameColor = await readRgb(frame, "background-color");
    for (const channel of frameColor) {
      if (theme === "dark") expect(channel).toBeLessThan(80);
      else expect(channel).toBeGreaterThan(240);
    }
    for (const role of ["candidate", "recruiter"]) {
      const card = dialog
        .locator("label")
        .filter({
          has: page.getByRole("radio", { name: new RegExp(`^${role} `, "i") }),
        })
        .locator("input + span");
      await expectReadableText(dialog.getByText(role, { exact: true }), card);
    }
    await page.screenshot({
      path: `/tmp/rolecue-onboarding-${theme}-roles.png`,
    });
    await dialog.getByRole("radio", { name: /^recruiter /i }).press("Space");
    await dialog.getByRole("button", { name: "Continue as recruiter" }).click();
    const name = dialog.getByLabel("Display name");
    await name.fill("Riley Recruiter");
    await expectReadableText(name, name);
    await dialog
      .getByLabel("Company website", { exact: true })
      .fill("acme.example");
    const preview = dialog.getByRole("complementary", {
      name: "Company website preview",
    });
    await expect(preview.getByRole("link")).toHaveAttribute(
      "href",
      "https://acme.example/",
    );
    await expectReadableText(
      preview.getByText("acme.example", { exact: true }),
      preview,
    );
    await page.screenshot({
      path: `/tmp/rolecue-onboarding-${theme}-metadata.png`,
    });
  });
}
