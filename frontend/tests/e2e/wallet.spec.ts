import { expect, test, type Page } from "@playwright/test";

async function walletSession(
  page: Page,
  role: "candidate" | "recruiter" | "admin",
) {
  const user = {
    id: "9b9e9289-994d-4e62-9e65-034b9a907330",
    role,
    email: "wallet@example.com",
    full_name: "Wallet Review",
    email_verified: true,
    is_locked: false,
    onboarding_role_selected: true,
    onboarding_completed: true,
  };
  await page.route("**/auth/me", (route) =>
    route.fulfill({
      headers: {
        "access-control-allow-origin": "http://127.0.0.1:3100",
        "access-control-allow-credentials": "true",
      },
      json: { success: true, data: user },
    }),
  );
  await page.route("**/wallet**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path.endsWith("/checkout"))
      return route.fulfill({
        headers: {
          "access-control-allow-origin": "http://127.0.0.1:3100",
          "access-control-allow-credentials": "true",
        },
        status: 503,
        json: { success: false, message: "Checkout unavailable" },
      });
    const data = path.endsWith("/packages")
      ? [
          {
            id: "pack",
            name: "Practice Pack",
            coin_amount: 100,
            price_vnd: 50000,
            is_active: true,
          },
        ]
      : path.endsWith("/transactions")
        ? {
            items: [
              {
                id: "tx",
                to: user.id,
                amount: 100,
                status: "success",
                payos_order_code: 123,
                created_at: "0001-01-01T00:00:00Z",
              },
            ],
            total: 1,
            page: 1,
            page_size: 20,
          }
        : path.endsWith("/sync")
          ? {
              id: "return",
              amount: 100,
              status: "cancelled",
              payos_order_code: 456,
            }
          : { id: "wallet", user_id: user.id, balance: 250 };
    return route.fulfill({
      headers: {
        "access-control-allow-origin": "http://127.0.0.1:3100",
        "access-control-allow-credentials": "true",
      },
      json: { success: true, data },
    });
  });
}

for (const role of ["candidate", "recruiter"] as const) {
  test(`${role} can open coin packages from the collapsed sidebar and return to transactions`, async ({
    page,
  }) => {
    await walletSession(page, role);
    const path = role === "recruiter" ? "/recruiter/billing" : "/billing";
    await page.goto(path);
    await expect(
      page.getByRole("table", { name: "Coin transactions" }),
    ).toBeVisible();
    await expect(page.getByText("Practice Pack", { exact: true })).toHaveCount(
      0,
    );
    await page.getByRole("button", { name: "Collapse sidebar" }).click();
    const trigger = page.getByRole("button", {
      name: "Buy coins",
      exact: true,
    });
    await trigger.hover();
    await expect(page.getByRole("tooltip")).toHaveText("Add coins");
    await trigger.click();
    const dialog = page.getByRole("dialog", { name: "Add coins" });
    await expect(dialog).toBeVisible();
    const requestPromise = page.waitForRequest(
      (req) =>
        req.url().endsWith("/wallet/deposit/checkout") &&
        req.method() === "POST",
    );
    await dialog
      .getByRole("button", { name: "Buy Practice Pack with PayOS" })
      .click();
    const request = await requestPromise;
    expect(request.postDataJSON()).toEqual({
      package_id: "pack",
      return_url: `http://127.0.0.1:3100${path}?status=success`,
      cancel_url: `http://127.0.0.1:3100${path}?cancel=true`,
    });
    await expect(dialog.getByRole("alert")).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(dialog).toHaveCount(0);
    await expect(trigger).toBeFocused();
    await page.goto(`${path}?status=success&orderCode=456`);
    await expect(
      page.getByText("Payment Cancelled", { exact: true }),
    ).toBeVisible();
    await expect(page.getByText("Payment Completed Successfully")).toHaveCount(
      0,
    );
  });
}

test("mobile coin dialog traps focus and closes back into navigation", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.emulateMedia({ reducedMotion: "reduce", colorScheme: "dark" });
  await walletSession(page, "candidate");
  await page.goto("/billing");
  await page.getByRole("button", { name: "Open workspace navigation" }).click();
  const trigger = page.getByRole("button", { name: "Buy coins", exact: true });
  await trigger.click();
  const dialog = page.getByRole("dialog", { name: "Add coins" });
  await expect(
    dialog.getByRole("button", { name: "Buy Practice Pack with PayOS" }),
  ).toBeVisible();
  const bounds = await dialog.boundingBox();
  expect(bounds!.width).toBeLessThanOrEqual(390);
  expect(bounds!.height).toBeLessThanOrEqual(844);
  for (let i = 0; i < 5; i++) {
    await page.keyboard.press("Tab");
    expect(
      await dialog.evaluate((el) => el.contains(document.activeElement)),
    ).toBe(true);
  }
  await page.keyboard.press("Escape");
  await expect(trigger).toBeFocused();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
});

test("admin has no personal purchase control or wallet requests", async ({
  page,
}) => {
  let walletRequests = 0;
  page.on("request", (req) => {
    if (new URL(req.url()).pathname.includes("/wallet")) walletRequests++;
  });
  await walletSession(page, "admin");
  await page.goto("/admin/dashboard");
  await expect(
    page.getByRole("button", { name: "Open account menu" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Buy coins", exact: true }),
  ).toHaveCount(0);
  expect(walletRequests).toBe(0);
});
