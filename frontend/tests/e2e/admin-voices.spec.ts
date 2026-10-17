import { test, expect, type Page } from "@playwright/test";

const admin = {
  id: "11111111-1111-4111-8111-111111111111",
  full_name: "Ada Admin",
  email: "admin@example.com",
  image: null,
  role: "admin",
  email_verified: true,
  is_locked: false,
};

async function mockVoices(page: Page) {
  let voices = [
    {
      id: "33333333-3333-4333-8333-333333333331",
      name: "Ava",
      is_active: true,
      interview_count: 2,
      job_posting_count: 0,
      in_use: true,
    },
    {
      id: "33333333-3333-4333-8333-333333333332",
      name: "Ben",
      is_active: true,
      interview_count: 0,
      job_posting_count: 0,
      in_use: false,
    },
  ];
  await page.route("http://localhost:8080/**", async (route) => {
    const url = new URL(route.request().url());
    const method = route.request().method();
    const headers = {
      "access-control-allow-origin": "http://127.0.0.1:3100",
      "access-control-allow-credentials": "true",
      "access-control-allow-methods": "GET,DELETE,OPTIONS",
      "access-control-allow-headers": "Content-Type",
    };
    if (method === "OPTIONS") return route.fulfill({ status: 204, headers });
    if (url.pathname === "/auth/me")
      return route.fulfill({ headers, json: { success: true, data: admin } });
    if (url.pathname === "/admin/voices" && method === "GET") {
      const status = url.searchParams.get("status") ?? "all";
      const items = voices.filter(
        (v) => status === "all" || (status === "active") === v.is_active,
      );
      return route.fulfill({
        headers,
        json: {
          success: true,
          data: {
            items,
            pagination: { limit: 20, offset: 0, total: items.length },
          },
        },
      });
    }
    const match = url.pathname.match(/^\/admin\/voices\/(.+)$/);
    if (match && method === "DELETE") {
      const voice = voices.find((v) => v.id === match[1])!;
      if (voice.in_use) {
        voice.is_active = false;
        return route.fulfill({
          headers,
          json: { success: true, data: { outcome: "deactivated", voice } },
        });
      }
      voices = voices.filter((v) => v.id !== voice.id);
      return route.fulfill({
        headers,
        json: { success: true, data: { outcome: "deleted", voice } },
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
}

test("admin deactivates a voice in use and removes an unused one", async ({
  page,
}) => {
  await mockVoices(page);
  await page.goto("/admin/voices");
  await expect(
    page.getByRole("heading", { level: 1, name: "Voice Profiles" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: /Fetch from provider/ }),
  ).toBeDisabled();

  await page.getByRole("button", { name: "Deactivate Ava" }).click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Deactivate voice" })
    .click();
  await expect(page.getByRole("row", { name: /Ava/ })).toContainText(
    "Inactive",
  );

  await page.getByRole("button", { name: "Remove Ben" }).click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Remove voice" })
    .click();
  await expect(page.getByRole("row", { name: /Ben/ })).toHaveCount(0);

  await page.getByRole("link", { name: "Inactive" }).click();
  await expect(page).toHaveURL(/status=inactive/);
});
