import { test, expect, type Page } from "@playwright/test";

const admin = {
  id: "11111111-1111-4111-8111-111111111111",
  full_name: "Ada Admin",
  email: "admin@example.com",
  image: null,
  role: "admin",
  email_verified: true,
  is_locked: false,
  onboarding_role_selected: true,
  onboarding_completed: true,
};

async function mockVoices(page: Page, options: { syncFails?: boolean } = {}) {
  const syncFails = options.syncFails ?? false;
  let voices = [
    {
      id: "33333333-3333-4333-8333-333333333331",
      name: "Ava",
      is_active: true,
      provider: "azure",
      provider_voice_id: "en-US-AvaNeural",
      locale: "en-US",
      gender: "Female",
      interview_count: 2,
      job_posting_count: 0,
      in_use: true,
    },
    {
      id: "33333333-3333-4333-8333-333333333332",
      name: "Ben",
      is_active: true,
      provider: null,
      provider_voice_id: null,
      locale: null,
      gender: null,
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
      "access-control-allow-methods": "GET,POST,DELETE,OPTIONS",
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
    if (url.pathname === "/admin/voices/sync" && method === "POST") {
      if (syncFails) {
        return route.fulfill({
          status: 502,
          headers,
          json: {
            success: false,
            error: {
              code: "VOICE_SYNC_FAILED",
              message: "the voice provider could not be reached",
            },
          },
        });
      }
      // The provider adds Cora and no longer offers Ava, which interviews still use.
      voices = voices.map((v) =>
        v.provider_voice_id === "en-US-AvaNeural"
          ? { ...v, is_active: false }
          : v,
      );
      voices.push({
        id: "33333333-3333-4333-8333-333333333333",
        name: "Cora",
        is_active: true,
        provider: "azure",
        provider_voice_id: "en-GB-CoraNeural",
        locale: "en-GB",
        gender: "Female",
        interview_count: 0,
        job_posting_count: 0,
        in_use: false,
      });
      return route.fulfill({
        headers,
        json: {
          success: true,
          data: {
            provider: "azure",
            fetched: 1,
            added: 1,
            updated: 0,
            deactivated: 1,
            unchanged: 0,
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

// A voice is a card in the gallery; its details and actions are in the panel beside it.
const card = (page: Page, name: RegExp) =>
  page.getByRole("list", { name: "Voices" }).getByRole("button", { name });

test("admin deactivates a voice in use and removes an unused one", async ({
  page,
}) => {
  await mockVoices(page);
  await page.goto("/admin/voices");
  await expect(
    page.getByRole("heading", { level: 1, name: "Voice Profiles" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Sync from provider" }),
  ).toBeEnabled();
  const details = page.getByRole("complementary", { name: "Voice details" });
  await expect(details).toContainText("en-US-AvaNeural");
  await expect(card(page, /^Ava/)).not.toContainText("en-US-AvaNeural");
  await expect(details.getByRole("heading", { name: "Ava" })).toBeVisible();

  await page.getByRole("button", { name: "Deactivate Ava" }).click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Deactivate voice" })
    .click();
  await expect(card(page, /^Ava/)).toContainText("Inactive");

  await card(page, /^Ben/).click();
  await page.getByRole("button", { name: "Remove Ben" }).click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Remove voice" })
    .click();
  await expect(card(page, /^Ben/)).toHaveCount(0);

  await page.getByRole("link", { name: "Inactive" }).click();
  await expect(page).toHaveURL(/status=inactive/);
});

test("admin syncs voices from the provider and sees the summary", async ({
  page,
}) => {
  await mockVoices(page);
  await page.goto("/admin/voices");
  await expect(card(page, /^Ava/)).toContainText("Active");

  await page.getByRole("button", { name: "Sync from provider" }).click();
  const result = page
    .getByRole("status")
    .filter({ hasText: "Synced 1 voices from azure" });
  await expect(result).toBeVisible();
  for (const [label, value] of [
    ["Added", "1"],
    ["Updated", "0"],
    ["Deactivated", "1"],
    ["Unchanged", "0"],
  ])
    await expect(
      result
        .locator("div", { has: page.getByText(label, { exact: true }) })
        .last(),
    ).toContainText(value);
  const cora = card(page, /^Cora/);
  await expect(cora).toContainText("en-GB");
  await expect(cora).toContainText("Female");
  await cora.click();
  await expect(
    page.getByRole("complementary", { name: "Voice details" }),
  ).toContainText("en-GB-CoraNeural");
  await expect(card(page, /^Ava/)).toContainText("Inactive");
  await expect(card(page, /^Ben/)).toContainText("Active");

  await page.getByRole("button", { name: "Dismiss" }).click();
  await expect(page.getByText(/Synced 1 voices/)).toHaveCount(0);
});

test("on a small screen a card opens the details in a sheet", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await mockVoices(page);
  await page.goto("/admin/voices");
  await expect(
    page.getByRole("complementary", { name: "Voice details" }),
  ).toHaveCount(0);
  await card(page, /^Ben/).click();
  const sheet = page.getByRole("dialog");
  await expect(sheet.getByRole("heading", { name: "Ben" })).toBeVisible();
  await expect(sheet.getByRole("button", { name: "Remove Ben" })).toBeVisible();
});

test("a failed sync says the catalog was not changed and can be retried", async ({
  page,
}) => {
  await mockVoices(page, { syncFails: true });
  await page.goto("/admin/voices");
  await page.getByRole("button", { name: "Sync from provider" }).click();
  await expect(
    page.getByRole("alert").filter({ hasText: "could not be reached" }),
  ).toContainText(
    "the voice provider could not be reached. The catalog was not changed.",
  );
  await expect(card(page, /^Ava/)).toContainText("Active");
  await expect(page.getByRole("button", { name: "Try again" })).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Sync from provider" }),
  ).toBeEnabled();
});
