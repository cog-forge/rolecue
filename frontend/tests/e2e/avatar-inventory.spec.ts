import { expect, test, type Page } from "@playwright/test";
import { avatarItem, mockAvatarApi } from "./fixtures/avatar-api";

const noHorizontalScroll = (page: Page) =>
  page.evaluate(() => document.documentElement.scrollWidth <= innerWidth);

for (const role of ["candidate", "recruiter"] as const) {
  test(`${role} sees the studio in the sidebar and the real capacity`, async ({
    page,
  }) => {
    await mockAvatarApi(page, {
      role,
      capacity: 5,
      items: [avatarItem(1), avatarItem(2)],
    });
    await page.goto("/avatar-studio");
    await expect(page.getByText("2 of 5 used")).toBeVisible();
    await expect(page.getByText("3 free slots")).toBeVisible();
    await expect(
      page.getByRole("link", { name: "Personal Avatar Studio" }).first(),
    ).toHaveAttribute("href", "/avatar-studio");
    await expect(
      page
        .getByRole("list", { name: "Avatar slots, 2 of 5 used" })
        .getByRole("listitem"),
    ).toHaveCount(5);
  });
}

test("admin is redirected away from the studio without inventory requests", async ({
  page,
}) => {
  const calls = await mockAvatarApi(page, { role: "admin" });
  await page.goto("/avatar-studio");
  await expect(page).toHaveURL(/\/admin\/dashboard$/);
  expect(calls.inventory).toBe(0);
});

test("full inventory at 390 px offers delete and buy without overflow", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await mockAvatarApi(page, {
    role: "candidate",
    capacity: 2,
    items: [avatarItem(1), avatarItem(2)],
  });
  await page.goto("/avatar-studio");
  await expect(page.getByText("Full", { exact: true })).toBeVisible();
  await expect(page.getByRole("link", { name: "Create avatar" })).toHaveCount(
    0,
  );
  await expect(page.getByText("1 coin per slot")).toBeVisible();
  await expect(
    page.getByRole("list", { name: /^Avatar slots/ }).getByRole("listitem"),
  ).toHaveCount(3);
  expect(await noHorizontalScroll(page)).toBe(true);
  await page.getByRole("button", { name: /Add more slots/ }).click();
  await expect(
    page.getByRole("dialog", { name: "Buy avatar slots" }),
  ).toBeVisible();
});

test("buying slots applies the returned summary", async ({ page }) => {
  await mockAvatarApi(page, {
    role: "recruiter",
    capacity: 3,
    items: [avatarItem(1)],
    purchase: { status: 200 },
  });
  await page.goto("/avatar-studio");
  await page.getByRole("button", { name: "Buy slots" }).click();
  const dialog = page.getByRole("dialog", { name: "Buy avatar slots" });
  await dialog.getByRole("button", { name: "One slot more" }).click();
  await dialog.getByRole("button", { name: "Buy for 2 coins" }).click();
  await expect(page.getByText("1 of 5 used")).toBeVisible();
});

test("an avatar in use can't be deleted", async ({ page }) => {
  const calls = await mockAvatarApi(page, {
    role: "recruiter",
    items: [avatarItem(1)],
    remove: { status: 409, code: "AVATAR_IN_USE" },
  });
  await page.goto("/avatar-studio");
  await page.getByRole("button", { name: "Delete Avatar 1" }).click();
  await page.getByRole("button", { name: "Delete avatar" }).click();
  await expect(page.getByRole("dialog").getByRole("alert")).toContainText(
    "used by a job posting or an interview",
  );
  expect(calls.deletes).toBe(1);
});

test("deleting frees a slot", async ({ page }) => {
  await mockAvatarApi(page, {
    role: "candidate",
    items: [avatarItem(1), avatarItem(2)],
  });
  await page.goto("/avatar-studio");
  await expect(page.getByText("2 of 3 used")).toBeVisible();
  await page.getByRole("button", { name: "Delete Avatar 2" }).click();
  await page.getByRole("button", { name: "Delete avatar" }).click();
  await expect(page.getByText("1 of 3 used")).toBeVisible();
  await expect(page.getByText("Avatar 2")).toHaveCount(0);
});

test("an expired preview link is re-fetched once, then reported", async ({
  page,
}) => {
  const calls = await mockAvatarApi(page, {
    role: "candidate",
    items: [avatarItem(1)],
  });
  let downloads = 0;
  await page.route("https://s3.test/**", (route) => {
    downloads++;
    return route.fulfill({
      status: 403,
      headers: { "access-control-allow-origin": "*" },
      body: "Request has expired",
    });
  });
  await page.goto("/avatar-studio");
  // The card's own portrait meets the same expired link first; count only the 3D view's requests.
  await expect(page.getByText("Couldn't render the preview")).toBeVisible({
    timeout: 15_000,
  });
  const detailsBefore = calls.details;
  const downloadsBefore = downloads;
  await page.getByRole("button", { name: "Open Avatar 1 in 3D" }).click();
  await expect(page).toHaveURL(/avatar=11111111-/);
  const dialog = page.getByRole("dialog", { name: "Avatar 1" });
  await expect(dialog.getByRole("alert")).toContainText(
    "preview link expired",
    {
      timeout: 15_000,
    },
  );
  expect(calls.details - detailsBefore).toBe(2);
  expect(downloads - downloadsBefore).toBe(2);
  await page.keyboard.press("Escape");
  await expect(page).not.toHaveURL(/avatar=/);
});

test("the preview falls back to 2D without WebGL, also at 390 px", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.addInitScript(() => {
    const original = HTMLCanvasElement.prototype.getContext;
    HTMLCanvasElement.prototype.getContext = function (
      this: HTMLCanvasElement,
      type: string,
      ...rest: unknown[]
    ) {
      if (type.startsWith("webgl")) return null;
      return (original as (...args: unknown[]) => RenderingContext | null).call(
        this,
        type,
        ...rest,
      );
    } as typeof original;
  });
  await mockAvatarApi(page, { role: "candidate", items: [avatarItem(1)] });
  await page.goto(`/avatar-studio?avatar=${avatarItem(1).id}`);
  const dialog = page.getByRole("dialog", { name: "Avatar 1" });
  await expect(dialog.getByRole("alert")).toContainText(
    "3D preview isn't available on this device or browser.",
  );
  await expect(dialog.getByText(avatarItem(1).personality)).toBeVisible();
  expect(await noHorizontalScroll(page)).toBe(true);
});
