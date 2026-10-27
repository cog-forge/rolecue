import { expect, test, type Page } from "@playwright/test";
import { mockAvatarApi, savedAvatar, type Role } from "./fixtures/avatar-api";

// SDK protocol fixture: proves the browser integration, not a real Avaturn T2 export.
const creatorHtml = `<html><body><button id="export">Export fixture</button><script>
const bytes = new Uint8Array(12);
const header = new DataView(bytes.buffer);
header.setUint32(0, 0x46546c67, true); header.setUint32(4, 2, true); header.setUint32(8, 12, true);
const payload = { avatarId: 'fixture', sessionId: 'fixture-session', bodyId: 'fixture-body', gender: 'male',
 avatarSupportsFaceAnimations: true, urlType: 'dataURL', url: 'data:model/gltf-binary;base64,' + btoa(String.fromCharCode(...bytes)) };
window.addEventListener('message', (event) => {
 if (event.data.eventName === 'sdk_handshake') parent.postMessage({ type: 'RESPONSE', source: 'v1.avaturn-sdk-server', key: 'sdk_handshake', isOk: true, data: {} }, '*');
});
parent.postMessage({ type: 'RESPONSE', source: 'v1.avaturn-sdk-server', key: 'sdk_handshake', isOk: true, data: {} }, '*');
document.querySelector('#export').onclick = () => {
 const message = { type: 'MESSAGE', source: 'v1.avaturn-sdk-server', eventName: 'callback_event', key: 'export', data: payload };
 parent.postMessage(message, '*'); parent.postMessage(message, '*');
};
</script></body></html>`;

async function setup(
  page: Page,
  role: Role,
  options: Omit<Parameters<typeof mockAvatarApi>[1], "role"> = {},
) {
  const calls = await mockAvatarApi(page, { role, ...options });
  await page.route("https://rolecue.avaturn.dev/**", (route) =>
    route.fulfill({ contentType: "text/html", body: creatorHtml }),
  );
  await page.goto("/avatar-studio/new");
  return calls;
}

async function openCreator(page: Page) {
  await expect(
    page.getByRole("button", { name: "Open Avaturn creator" }),
  ).toBeEnabled();
  await expect(page.locator("iframe")).toHaveCount(0);
  await page.getByLabel("Avatar name").fill(savedAvatar.name);
  await page
    .getByLabel("Personality", { exact: true })
    .fill(savedAvatar.personality);
  await page.getByRole("button", { name: "Open Avaturn creator" }).click();
  await expect(
    page.getByText(
      "Create and customize your avatar, then choose Next in Avaturn.",
    ),
  ).toBeVisible();
  await expect(page.locator('[aria-current="step"]')).toContainText("Create");
}

const exportFixture = (page: Page) =>
  page
    .frameLocator('iframe[title="Avaturn avatar creator"]')
    .getByRole("button", { name: "Export fixture" })
    .click();

for (const role of ["candidate", "recruiter"] as const) {
  test(`${role} can export and save once despite duplicate callbacks`, async ({
    page,
  }) => {
    const calls = await setup(page, role);
    await openCreator(page);
    await exportFixture(page);
    await expect(
      page.getByRole("heading", { name: "Alex has been saved" }),
    ).toBeFocused();
    await expect(page.locator('[aria-current="step"]')).toHaveCount(0);
    await expect(page.locator("iframe")).toHaveCount(0);
    await expect(page.getByText("1 of 3 avatar slots used")).toBeVisible();
    await expect(
      page.getByRole("link", { name: "Preview in 3D" }),
    ).toHaveAttribute("href", `/avatar-studio?avatar=${savedAvatar.id}`);
    expect(calls.creates).toBe(1);
  });
}

test("mobile discard removes iframe and never sends a save", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const calls = await setup(page, "candidate");
  await openCreator(page);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page
    .getByRole("button", { name: "Close creator", exact: true })
    .click();
  await page
    .getByRole("button", { name: "Discard draft", exact: true })
    .click();
  await expect(page.locator("iframe")).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "Open Avaturn creator" }),
  ).toBeVisible();
  expect(calls.creates).toBe(0);
});

test("a confirmed retryable failure keeps the details and resends only on request", async ({
  page,
}) => {
  const calls = await setup(page, "candidate", {
    create: { status: 503, code: "AVATAR_CONVERTER_UNAVAILABLE" },
  });
  await openCreator(page);
  await exportFixture(page);
  const alert = page.locator("section").getByRole("alert");
  await expect(alert).toContainText("Nothing was saved");
  await expect(page.getByText("Alex has been saved")).toHaveCount(0);
  await expect(page.getByLabel("Avatar name")).toHaveValue(savedAvatar.name);
  expect(calls.creates).toBe(1);
  await page.getByRole("button", { name: "Try saving again" }).click();
  await expect(alert).toContainText("Nothing was saved");
  expect(calls.creates).toBe(2);
});

test("an unsupported export is a confirmed failure without a retry", async ({
  page,
}) => {
  const calls = await setup(page, "candidate", {
    create: { status: 422, code: "AVATAR_SOURCE_UNSUPPORTED" },
  });
  await openCreator(page);
  await exportFixture(page);
  await expect(page.locator("section").getByRole("alert")).toContainText(
    "facial animation (T2)",
  );
  await expect(
    page.getByRole("button", { name: "Try saving again" }),
  ).toHaveCount(0);
  expect(calls.creates).toBe(1);
});

test("an unknown outcome does not report success or resend", async ({
  page,
}) => {
  const calls = await setup(page, "candidate", { create: "abort" });
  await openCreator(page);
  await exportFixture(page);
  await expect(
    page.getByRole("heading", {
      name: "Check your inventory before trying again",
    }),
  ).toBeFocused();
  await expect(page.getByText("Alex has been saved")).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "Open Avaturn creator" }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("link", { name: "Check inventory" }),
  ).toHaveAttribute("href", "/avatar-studio");
  expect(calls.creates).toBe(1);
});

test("full inventory blocks creator and points to the inventory", async ({
  page,
}) => {
  const calls = await setup(page, "candidate", {
    capacity: 1,
    items: [{ ...savedAvatar, id: "9b9e9289-994d-4e62-9e65-034b9a907339" }],
  });
  await expect(page.locator("section").getByRole("alert")).toContainText(
    "inventory is full",
  );
  await expect(
    page.getByRole("link", { name: "Manage your inventory" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Open Avaturn creator" }),
  ).toBeDisabled();
  await expect(page.locator("iframe")).toHaveCount(0);
  expect(calls.creates).toBe(0);
});

test("admin is redirected without inventory requests", async ({ page }) => {
  const calls = await setup(page, "admin");
  await expect(page).toHaveURL(/\/admin\/dashboard$/);
  expect(calls.inventory).toBe(0);
  expect(calls.creates).toBe(0);
});

test("personality shortcuts stay editable and expanding keeps the active creator", async ({
  page,
}) => {
  const calls = await setup(page, "candidate");
  await page.getByRole("button", { name: "Supportive", exact: true }).click();
  await expect(page.getByLabel("Personality", { exact: true })).toHaveValue(
    /Patient and encouraging/,
  );
  await openCreator(page);
  const frame = await page
    .locator('iframe[title="Avaturn avatar creator"]')
    .elementHandle();
  if (!frame) throw new Error("Creator iframe missing");
  await page.getByRole("button", { name: "Expand editor" }).click();
  await expect(
    page.getByRole("button", { name: "Restore layout" }),
  ).toHaveAttribute("aria-pressed", "true");
  await expect(
    page.getByRole("complementary", { name: "Your avatar details" }),
  ).toBeHidden();
  expect(await frame.evaluate((element) => element.isConnected)).toBe(true);
  await page.getByRole("button", { name: "Restore layout" }).click();
  await page.getByRole("button", { name: "Reload creator" }).click();
  await expect(page.getByRole("dialog")).toContainText(
    "Restart the Avaturn creator?",
  );
  await page.getByRole("button", { name: "Keep editing", exact: true }).click();
  expect(await frame.evaluate((element) => element.isConnected)).toBe(true);
  await page.getByRole("button", { name: "Reload creator" }).click();
  await page
    .getByRole("button", { name: "Restart creator", exact: true })
    .click();
  await expect(
    page
      .frameLocator('iframe[title="Avaturn avatar creator"]')
      .getByRole("button", { name: "Export fixture" }),
  ).toBeVisible();
  expect(await frame.evaluate((element) => element.isConnected)).toBe(false);
  expect(calls.creates).toBe(0);
});

test("save waits for confirmation and honors reduced motion", async ({
  page,
}) => {
  const pageErrors: string[] = [];
  page.on("pageerror", (error) => pageErrors.push(error.message));
  await page.emulateMedia({ reducedMotion: "reduce", colorScheme: "dark" });
  let finishSave: (() => void) | undefined;
  const pendingSave = new Promise<void>((resolve) => {
    finishSave = resolve;
  });
  await setup(page, "candidate");
  expect(pageErrors).toEqual([]);
  await expect(page.locator("html")).toHaveClass(/dark/);
  await page.route("http://localhost:8080/inventory/avatars", async (route) => {
    if (route.request().method() !== "POST") return route.fallback();
    await pendingSave;
    await route.fallback();
  });
  try {
    await openCreator(page);
    await exportFixture(page);
    const heading = page.getByRole("heading", {
      name: "Processing avatar",
      exact: true,
    });
    await expect(heading).toBeFocused();
    await expect(page.locator('[aria-current="step"]')).toContainText("Save");
    await expect(page.getByText("Alex has been saved")).toHaveCount(0);
    expect(
      await heading.evaluate(
        (element) => getComputedStyle(element, "::before").animationName,
      ),
    ).toBe("none");
    finishSave?.();
    await expect(
      page.getByRole("heading", { name: "Alex has been saved" }),
    ).toBeFocused();
  } finally {
    finishSave?.();
  }
});
