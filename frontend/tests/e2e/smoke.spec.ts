import { expect, test } from "@playwright/test";
test("public, wizard and admin routes render without a backend", async ({
  page,
}) => {
  await page.goto("/");
  await expect(
    page.getByRole("heading", {
      name: "Practice the role before the room.",
    }),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Start a practice" }).first(),
  ).toBeVisible();
  await page.keyboard.press("Tab");
  await expect(page.locator(":focus")).toHaveAttribute("href", "#main-content");
  await expect(
    page.getByRole("link", { name: "See the method" }),
  ).toHaveAttribute("href", "#method");
  await page.getByRole("tab", { name: "Read the role" }).focus();
  await page.keyboard.press("ArrowRight");
  await expect(
    page.getByRole("tab", { name: "Try the response" }),
  ).toHaveAttribute("aria-selected", "true");
  await page
    .getByRole("button", { name: "Is RoleCue trying to script an interview?" })
    .click();
  await expect(
    page.getByText(/centers reflection, examples, and clearer choices/i),
  ).toBeVisible();
  await page.goto("/interviews");
  await page
    .getByRole("link", { name: "Create an interview", exact: true })
    .click();
  await expect(page).toHaveURL(/\/interviews\/new\/job-description$/);
  await page.getByRole("link", { name: "2. Skills", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Review skills" }),
  ).toBeVisible();
  await page.reload();
  await expect(
    page.getByRole("link", { name: "2. Skills", exact: true }),
  ).toHaveAttribute("aria-current", "step");
  await page.goBack();
  await expect(
    page.getByRole("heading", { name: "Job description" }),
  ).toBeVisible();
  await page.goto("/admin/users");
  await expect(
    page.getByRole("heading", { name: "Users", exact: true }),
  ).toBeVisible();
});

test("mobile navigation exposes the landing anchors", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/");

  await page.getByRole("button", { name: "Open navigation" }).click();
  await expect(page.getByRole("link", { name: "Questions" })).toBeVisible();
  await page.getByRole("link", { name: "Questions" }).click();
  await expect(page.locator("#questions")).toBeVisible();
});

test("FAQ honors reduced-motion preferences", async ({ page }) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/");

  await page
    .getByRole("button", { name: "Is RoleCue trying to script an interview?" })
    .click();

  const content = page
    .locator('[data-slot="accordion-content"]')
    .filter({ hasText: /centers reflection, examples, and clearer choices/i });

  await expect(content).toBeVisible();
  await expect(content).toHaveCSS("animation-name", "none");
});
