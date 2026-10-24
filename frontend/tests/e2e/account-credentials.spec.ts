import { test, expect, type Page } from "@playwright/test";

const id = "11111111-1111-4111-8111-111111111111";
const timestamp = "2026-10-10T10:00:00.000Z";
async function security(page: Page, role = "candidate", password = true) {
  let signedIn = true;
  let enabled = false;
  let currentId = "current-session";
  let accounts = [
    ...(password
      ? [{ id: "credential-record", providerId: "credential", accountId: id }]
      : []),
    { id: "google-record", providerId: "google", accountId: "external-google" },
  ].map((account) => ({
    ...account,
    userId: id,
    createdAt: timestamp,
    updatedAt: timestamp,
    scopes: [],
  }));
  const session = (sessionId: string) => ({
    id: sessionId,
    userId: id,
    token: `MOCK-PRIVATE-${sessionId}`,
    createdAt: timestamp,
    updatedAt: timestamp,
    expiresAt: "2030-01-01T00:00:00.000Z",
    ipAddress: "192.0.2.1",
    userAgent: "Mozilla/5.0 Chrome/123.0 Linux " + "long-agent ".repeat(30),
  });
  let sessions = [session(currentId), session("other-session")];
  const user = () => ({
    id,
    name: "Alex Doe",
    email: "alex@example.test",
    emailVerified: true,
    role,
    banned: false,
    twoFactorEnabled: enabled,
    createdAt: timestamp,
    updatedAt: timestamp,
  });
  const requests: { path: string; body: Record<string, unknown> }[] = [];
  await page.route("**/auth/me", (route) =>
    route.fulfill({
      status: signedIn ? 200 : 401,
      headers: {
        "access-control-allow-origin": "http://127.0.0.1:3100",
        "access-control-allow-credentials": "true",
      },
      json: signedIn
        ? {
            success: true,
            data: {
              id,
              full_name: "Alex Doe",
              email: "alex@example.test",
              role,
              email_verified: true,
              is_locked: false,
              onboarding_role_selected: true,
              onboarding_completed: true,
            },
          }
        : {
            success: false,
            error: { code: "INVALID_TOKEN", message: "Unauthorized" },
          },
    }),
  );
  await page.route("**/api/auth/**", async (route) => {
    const path = new URL(route.request().url()).pathname.replace(
      "/api/auth",
      "",
    );
    const body = route.request().postDataJSON() ?? {};
    if (route.request().method() === "POST") requests.push({ path, body });
    const reply = (json: unknown, status = 200) =>
      route.fulfill({ status, json });
    switch (path) {
      case "/get-session":
        return reply(
          signedIn ? { user: user(), session: session(currentId) } : null,
        );
      case "/list-accounts":
        return reply(accounts);
      case "/list-sessions":
        return reply(sessions);
      case "/sign-in/email":
        signedIn = false;
        return reply({ twoFactorRedirect: true, twoFactorMethods: ["totp"] });
      case "/two-factor/enable":
        return reply({
          method: "totp",
          totpURI:
            "otpauth://totp/RoleCue:mock?secret=MOCKSETUPKEY&issuer=RoleCue",
          backupCodes: ["MOCK1-ABCDE", "MOCK2-FGHIJ"],
        });
      case "/two-factor/verify-totp":
        if (body.code !== "012345")
          return reply({ code: "INVALID_CODE", message: "Invalid code" }, 401);
        enabled = true;
        signedIn = true;
        currentId = "rotated-session";
        sessions = [session(currentId), session("other-session")];
        return reply({ status: true });
      case "/two-factor/disable":
        enabled = false;
        return reply({ status: true });
      case "/two-factor/generate-backup-codes":
        return reply({ backupCodes: ["REPLACEMENT-ABCDE"] });
      case "/change-password":
        sessions = [session(currentId)];
        return reply({ user: user(), token: "MOCK-PRIVATE-NEW" });
      case "/unlink-account":
        accounts = accounts.filter((account) => account.id !== body.accountId);
        return reply({ status: true });
      case "/revoke-session":
        sessions = sessions.filter((item) => item.token !== body.token);
        return reply({ status: true });
      case "/revoke-other-sessions":
        sessions = [session(currentId)];
        return reply({ status: true });
      case "/revoke-sessions":
        signedIn = false;
        sessions = [];
        return reply({ status: true });
      case "/sign-out":
        signedIn = false;
        return reply({ success: true });
      default:
        return reply({ code: "NOT_FOUND" }, 404);
    }
  });
  return requests;
}

for (const role of ["candidate", "recruiter", "admin"]) {
  test(`${role} can access shared credentials and only disconnect a provider after confirmation`, async ({
    page,
  }) => {
    const requests = await security(page, role);
    await page.goto("/settings");
    await expect(
      page.getByRole("heading", { name: "Account security" }),
    ).toBeVisible();
    await expect(page.getByText("This session", { exact: true })).toBeVisible();
    expect(await page.locator("body").innerHTML()).not.toContain(
      "MOCK-PRIVATE",
    );
    await page.getByRole("button", { name: "Disconnect Google" }).click();
    expect(requests.some((request) => request.path === "/unlink-account")).toBe(
      false,
    );
    await page.getByRole("button", { name: "Confirm disconnect" }).click();
    // goey-toast renders the title in both the toast and its live region.
    await expect(page.getByText("Google disconnected").first()).toBeVisible();
    expect(
      requests.find((request) => request.path === "/unlink-account")?.body,
    ).toEqual({ accountId: "google-record" });
  });
}

test("enrollment is verified before backup codes, restores focus, and refreshes the rotated current session", async ({
  page,
}) => {
  await security(page);
  await page.goto("/settings");
  const trigger = page.getByRole("button", { name: "Enable authenticator" });
  await trigger.click();
  await expect(
    page.getByLabel("Current password", { exact: true }),
  ).toBeFocused();
  await page
    .getByLabel("Current password", { exact: true })
    .fill("test-password");
  await page.getByRole("button", { name: "Continue", exact: true }).click();
  await expect(page.getByText("MOCKSETUPKEY")).toBeVisible();
  await expect(
    page.getByRole("img", {
      name: "Scan this QR code with your authenticator",
    }),
  ).toBeVisible();
  await page.getByLabel("Authenticator code").fill("123456");
  await page.getByRole("button", { name: "Verify authenticator" }).click();
  await expect(
    page.getByRole("alert").filter({ hasText: "code is invalid" }),
  ).toBeVisible();
  await expect(page.getByText("MOCK1-ABCDE")).toHaveCount(0);
  await page.getByLabel("Authenticator code").fill("012345");
  await page.getByRole("button", { name: "Verify authenticator" }).click();
  await expect(page.getByText("MOCK1-ABCDE")).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Done", exact: true }),
  ).toBeDisabled();
  await page.getByLabel("I have saved my backup codes").check();
  await page.getByRole("button", { name: "Done", exact: true }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(
    page
      .getByRole("region", { name: "Two-factor authentication" })
      .getByText("On", { exact: true }),
  ).toBeVisible();
  await expect(page.getByText("This session", { exact: true })).toHaveCount(1);
  expect(await page.locator("body").innerHTML()).not.toContain("MOCKSETUPKEY");
});

test("password change clears fields and revokes other sessions", async ({
  page,
}) => {
  await security(page);
  await page.goto("/settings");
  const trigger = page.getByRole("button", {
    name: "Change password",
    exact: true,
  });
  await trigger.click();
  await page
    .getByLabel("Current password", { exact: true })
    .fill("test-password");
  await page
    .getByLabel("New password", { exact: true })
    .fill("replacement-password");
  await page
    .getByLabel("Confirm new password", { exact: true })
    .fill("replacement-password");
  await page.getByRole("button", { name: "Update password" }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);

  await expect(
    page.getByRole("button", { name: "Sign out other sessions" }),
  ).toBeDisabled();
  await trigger.click();
  await expect(
    page.getByLabel("Current password", { exact: true }),
  ).toHaveValue("");
  await page.keyboard.press("Escape");
});

test("challenge rejects invalid codes and honors only the local security return destination", async ({
  page,
}) => {
  await security(page);
  await page.goto("/login?returnTo=%2Fsettings");
  await page.getByLabel("Email address").fill("alex@example.test");
  await page.getByLabel("Password", { exact: true }).fill("test-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page).toHaveURL(/\/two-factor\?returnTo=%2Fsettings$/);
  await expect(
    page.getByLabel("Remember this browser for 30 days"),
  ).not.toBeChecked();
  await page.getByLabel("Authenticator code").fill("123456");
  await page.getByRole("button", { name: "Verify and sign in" }).click();
  await expect(
    page.getByRole("alert").filter({ hasText: "code is invalid" }),
  ).toBeVisible();
  await expect(page).toHaveURL(/\/two-factor/);
  await page.getByLabel("Authenticator code").fill("012345");
  await page.getByRole("button", { name: "Verify and sign in" }).click();
  await expect(page).toHaveURL(/\/settings(?:\?auth=signed-in)?$/);
  await expect(
    page.getByRole("heading", { name: "Account security" }),
  ).toBeVisible();
});

test("OAuth-only eligibility and long user agents fit mobile and desktop in both themes", async ({
  page,
}) => {
  await security(page, "candidate", false);
  await page.goto("/settings");
  await expect(
    page.getByRole("link", { name: "Forgot password" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Enable authenticator" }),
  ).toHaveCount(0);
  for (const theme of ["Light", "Dark"] as const) {
    for (const width of [390, 1280, 1440]) {
      await page.setViewportSize({ width, height: 900 });
      if (width < 768)
        await page
          .getByRole("button", { name: "Open workspace navigation" })
          .click();
      await page.getByRole("button", { name: "Open account menu" }).click();
      await page
        .getByRole("menuitemradio", { name: theme, exact: true })
        .click();
      if (width < 768) {
        await page
          .getByRole("dialog", { name: "Workspace navigation" })
          .getByRole("button", { name: "Close", exact: true })
          .click();
        await expect(
          page.getByRole("dialog", { name: "Workspace navigation" }),
        ).toHaveCount(0);
      }
      await page
        .getByRole("button", { name: "Session details" })
        .first()
        .click();
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= window.innerWidth,
        ),
      ).toBe(true);
    }
  }
});
