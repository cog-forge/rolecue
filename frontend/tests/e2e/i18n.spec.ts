import { test, expect } from "@playwright/test";

test.describe("language", () => {
  test("a new visitor gets the browser language, and English when it is unsupported", async ({
    browser,
    baseURL,
  }) => {
    const vi = await browser.newContext({
      baseURL,
      locale: "vi-VN",
    });
    const viPage = await vi.newPage();
    await viPage.goto("/");
    await expect(viPage.locator("html")).toHaveAttribute("lang", "vi");
    await expect(
      viPage.getByRole("link", { name: "Đăng nhập" }).first(),
    ).toBeVisible();
    await vi.close();

    const fr = await browser.newContext({
      baseURL,
      locale: "fr-FR",
    });
    const frPage = await fr.newPage();
    await frPage.goto("/");
    await expect(frPage.locator("html")).toHaveAttribute("lang", "en");
    await fr.close();
  });

  test("switching language keeps the page and the typed input, and survives a reload", async ({
    page,
  }) => {
    await page.goto("/login");
    await page.getByLabel("Email address").fill("typed@example.com");
    await page.getByRole("button", { name: /Change language/ }).click();
    await expect(page.getByRole("menuitemradio")).toHaveText([
      "English",
      "Tiếng Việt",
    ]);
    await page.getByRole("menuitemradio", { name: "Tiếng Việt" }).click();

    await expect(page.locator("html")).toHaveAttribute("lang", "vi");
    await expect(page).toHaveURL(/\/login$/);
    await expect(page.getByLabel("Email address")).toHaveValue(
      "typed@example.com",
    );

    await page.reload();
    await expect(page.locator("html")).toHaveAttribute("lang", "vi");
    await expect(
      page.getByRole("button", { name: "Đổi ngôn ngữ: Tiếng Việt" }),
    ).toBeVisible();
  });

  test("form validation speaks the chosen language", async ({ page }) => {
    await page
      .context()
      .addCookies([
        { name: "NEXT_LOCALE", value: "vi", url: "http://127.0.0.1:3100" },
      ]);
    await page.goto("/login");
    await page.getByRole("button", { name: "Sign in" }).click();
    await expect(page.getByText("Vui lòng nhập mật khẩu")).toBeVisible();
  });
});
