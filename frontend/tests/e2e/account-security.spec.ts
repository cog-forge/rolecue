import { expect, test, type Page } from "@playwright/test";

const user = {
  id: "11111111-1111-4111-8111-111111111111",
  full_name: "Alex Doe",
  email: "alex@example.com",
  image: null,
  role: "candidate",
  email_verified: true,
  is_locked: false,
  onboarding_role_selected: true,
  onboarding_completed: true,
};

async function mockSession(page: Page, status: 200 | 401) {
  await page.route("http://localhost:8080/**", async (route) => {
    const headers = {
      "access-control-allow-origin": "http://127.0.0.1:3100",
      "access-control-allow-credentials": "true",
      "access-control-allow-methods": "GET,OPTIONS",
      "access-control-allow-headers": "Content-Type",
    };
    if (route.request().method() === "OPTIONS") {
      await route.fulfill({ status: 204, headers });
      return;
    }
    await route.fulfill(
      status === 200
        ? { status, headers, json: { success: true, data: user } }
        : {
            status,
            headers,
            json: {
              success: false,
              error: { code: "INVALID_TOKEN", message: "Session expired" },
            },
          },
    );
  });
}

test("changes the password through Better Auth without exposing it in a URL", async ({
  page,
}) => {
  await mockSession(page, 200);
  await page.route("**/api/auth/list-accounts", (route) =>
    route.fulfill({ json: [{ id: "a", providerId: "credential" }] }),
  );
  const bodies: unknown[] = [];
  await page.route("**/api/auth/change-password", async (route) => {
    bodies.push(route.request().postDataJSON());
    await route.fulfill({ json: { token: "t", user: { id: user.id } } });
  });
  const urls: string[] = [];
  page.on("request", (request) => urls.push(request.url()));

  await page.goto("/settings");
  await page.getByRole("button", { name: "Change password" }).click();
  const dialog = page.getByRole("dialog", { name: "Change password" });
  // exact: the show/hide buttons are labelled "Show current password" and so on.
  await dialog
    .getByLabel("Current password", { exact: true })
    .fill("Current-Secret-1");
  await dialog
    .getByLabel("New password", { exact: true })
    .fill("Brand-New-Secret-2");
  await dialog
    .getByLabel("Confirm new password", { exact: true })
    .fill("Brand-New-Secret-2");
  await dialog.getByRole("button", { name: "Update password" }).click();

  await expect(dialog).toBeHidden();
  expect(bodies).toEqual([
    {
      currentPassword: "Current-Secret-1",
      newPassword: "Brand-New-Secret-2",
      revokeOtherSessions: true,
    },
  ]);
  expect(urls.join(" ")).not.toContain("Secret");
  expect(page.url()).not.toContain("Secret");
});
