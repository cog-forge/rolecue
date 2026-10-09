import { expect, test } from "@playwright/test";

test("public navigation and protected routes enforce login", async ({
  page,
}) => {
  await page.goto("/");
  await expect(
    page.getByRole("heading", {
      name: "Walk in ready.",
    }),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Start now" }).first(),
  ).toBeVisible();
  await expect(
    page.getByText("Practice the role before the room."),
  ).toBeVisible();
  await expect(page.getByText("Real interview practice").first()).toBeVisible();
  await expect(page.getByText("ROLE-BASED").first()).toBeVisible();
  await page.keyboard.press("Tab");
  await expect(page.locator(":focus")).toHaveAttribute("href", "#main-content");
  await page.getByRole("button", { name: "Show Set up your session" }).click();
  await expect(
    page.getByRole("button", { name: "Show Set up your session" }),
  ).toHaveAttribute("aria-pressed", "true");
  await page.getByRole("button", { name: "Can I start for free?" }).click();
  await expect(
    page.getByText(/you can create an account and start practicing for free/i),
  ).toBeVisible();
  await page.route("**/auth/me", (route) =>
    route.fulfill({
      status: 401,
      contentType: "application/json",
      body: JSON.stringify({
        success: false,
        error: {
          code: "INVALID_TOKEN",
          message: "a valid session is required",
        },
      }),
    }),
  );
  await page.goto("/interviews");
  await expect(page).toHaveURL(/\/login$/);
  await page.goto("/admin/users");
  await expect(page).toHaveURL(/\/login$/);
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

  await page.getByRole("button", { name: "Can I start for free?" }).click();

  const content = page
    .locator('[data-slot="accordion-content"]')
    .filter({
      hasText: /you can create an account and start practicing for free/i,
    });

  await expect(content).toBeVisible();
  await expect(content).toHaveCSS("animation-name", "none");
});
