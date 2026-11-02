import { expect, test } from "@playwright/test";

test.describe("public pages language", () => {
  test("the landing switches to Vietnamese through the language switcher", async ({
    page,
  }) => {
    await page.goto("/");
    await expect(
      page.getByRole("heading", { name: "Walk in ready." }),
    ).toBeVisible();

    await page
      .getByRole("button", { name: /Change language/ })
      .first()
      .click();
    await page.getByRole("menuitemradio", { name: "Tiếng Việt" }).click();

    await expect(page.locator("html")).toHaveAttribute("lang", "vi");
    await expect(
      page.getByRole("heading", { name: "Sẵn sàng bước vào." }),
    ).toBeVisible();
    await expect(
      page.getByRole("link", { name: "Bắt đầu ngay" }).first(),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Tôi có thể bắt đầu miễn phí không?" }),
    ).toBeVisible();
    await expect(page.getByText("Luyện phỏng vấn như thật")).toBeVisible();
  });

  test("the pricing page follows the NEXT_LOCALE cookie", async ({ page }) => {
    await page
      .context()
      .addCookies([
        { name: "NEXT_LOCALE", value: "vi", url: "http://127.0.0.1:3100" },
      ]);
    await page.goto("/pricing");
    await expect(page.getByRole("heading", { name: "Bảng giá" })).toBeVisible();
    await expect(
      page.getByRole("link", { name: "Trang chủ", exact: true }),
    ).toBeVisible();
  });
});
