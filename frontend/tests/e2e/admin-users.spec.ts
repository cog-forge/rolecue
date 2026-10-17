import { expect, test, type Page } from "@playwright/test";
import { z } from "zod";
import { banRequestSchema } from "../../src/features/admin/accounts/schemas/account-lock";

const adminId = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const candidateId = "11111111-1111-4111-8111-111111111111";
const recruiterId = "22222222-2222-4222-8222-222222222222";
const candidate = {
  id: candidateId,
  name: "Alex Candidate",
  email: "alex@example.com",
  role: "candidate",
  banned: false,
  banReason: null as string | null,
  banExpires: null as string | null,
  image: null,
  createdAt: "2026-10-01T12:00:00Z",
};
function wireAccount(user: typeof candidate) {
  return {
    id: user.id,
    name: user.name,
    email: user.email,
    role: user.role,
    image: user.image,
    is_locked: user.banned,
    lock_reason: user.banReason,
    lock_expires_at: user.banExpires,
    created_at: user.createdAt,
    updated_at: user.createdAt,
    email_verified: true,
  };
}
async function mockAccounts(
  page: Page,
  role: "admin" | "candidate" | "recruiter" = "admin",
) {
  let users = [
    candidate,
    {
      ...candidate,
      id: recruiterId,
      name: "Sam Recruiter",
      email: "sam@example.com",
      role: "recruiter",
      banned: true,
      banReason: "Repeated spam",
      banExpires: null,
    },
    {
      ...candidate,
      id: adminId,
      name: "Platform Admin",
      email: "admin@example.com",
      role: "admin",
    },
  ];
  const mutations: Array<z.infer<typeof banRequestSchema>> = [];
  let listCalls = 0;
  await page.route("**/auth/me", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        success: true,
        data: {
          id: adminId,
          full_name: "Administrator",
          email: "admin@example.com",
          role,
          email_verified: true,
          is_locked: false,
          onboarding_role_selected: true,
          onboarding_completed: true,
          image: null,
        },
      }),
    }),
  );
  await page.route("**/admin/users**", async (route) => {
    if (route.request().resourceType() === "document") return route.continue();
    const url = new URL(route.request().url());
    if (url.pathname.endsWith("/users")) {
      listCalls++;
      let filtered = [...users];
      const search = url.searchParams.get("search");
      if (search) {
        filtered = filtered.filter(
          (user) =>
            user.name.toLowerCase().includes(search.toLowerCase()) ||
            user.email.toLowerCase().includes(search.toLowerCase()),
        );
      }
      const filter = url.searchParams.get("filter");
      if (filter && filter !== "all" && filter !== "locked")
        filtered = filtered.filter((user) => user.role === filter);
      if (filter === "locked")
        filtered = filtered.filter((user) => user.banned);
      const offset = Number(url.searchParams.get("offset"));
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          success: true,
          data: {
            users: filtered.slice(offset, offset + 20).map(wireAccount),
            total: filtered.length,
          },
        }),
      });
    }
    if (route.request().method() === "GET") {
      const user = users.find(
        (entry) => entry.id === url.pathname.split("/")[3],
      );
      return route.fulfill({
        status: user ? 200 : 404,
        contentType: "application/json",
        body: JSON.stringify(
          user
            ? { success: true, data: wireAccount(user) }
            : { success: false, error: { message: "Account not found" } },
        ),
      });
    }
    const body = route.request().postDataJSON();
    const lock = url.pathname.endsWith("/lock");
    const userId = z.uuid().parse(url.pathname.split("/")[3]);
    const lockBody = lock
      ? banRequestSchema.parse({
          userId,
          banReason: body.reason,
          banExpiresIn: body.expires_in,
        })
      : null;
    const parsed = lockBody ?? { userId };
    if (lockBody) mutations.push(lockBody);
    users = users.map((user) =>
      user.id === parsed.userId
        ? {
            ...user,
            banned: lock,
            banReason: lockBody?.banReason ?? null,
            banExpires: lockBody?.banExpiresIn
              ? new Date(
                  Date.now() + lockBody.banExpiresIn * 1000,
                ).toISOString()
              : null,
          }
        : user,
    );
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        success: true,
        data: {
          user: wireAccount(
            users.find((user) => user.id === parsed.userId) ?? candidate,
          ),
          changed: true,
          notification: { status: "accepted" },
        },
      }),
    });
  });
  return { mutations, listCalls: () => listCalls };
}

test("admin searches, filters, confirms a preset lock and unlocks an account", async ({
  page,
}) => {
  const api = await mockAccounts(page);
  await page.goto("/admin/users");
  await expect(
    page.getByRole("heading", { name: "Accounts", exact: true }),
  ).toBeVisible();
  const table = page.getByRole("table", { name: "Platform accounts" });
  await expect(
    table.getByText("Platform Admin", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Lock Platform Admin" }),
  ).toHaveCount(0);
  await page.getByLabel("Search accounts").fill("Alex");
  await expect(table.getByText("Sam Recruiter")).toHaveCount(0);
  await page
    .getByRole("button", { name: "Actions for Alex Candidate", exact: true })
    .click();
  await page
    .getByRole("menuitem", { name: "Lock account", exact: true })
    .click();
  await expect(page.getByRole("dialog")).toContainText("alex@example.com");
  await page
    .getByLabel("Reason sent to user", { exact: true })
    .fill("Repeated abuse");
  await page.getByRole("button", { name: "24 hours", exact: true }).click();
  await page.getByRole("button", { name: "Confirm lock" }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "Actions for Alex Candidate" }),
  ).toBeEnabled();
  expect(api.mutations).toEqual([
    { userId: candidateId, banReason: "Repeated abuse", banExpiresIn: 86400 },
  ]);
  await page.getByRole("combobox", { name: "Filter accounts" }).click();
  await page
    .getByRole("option", { name: "Locked accounts", exact: true })
    .click();
  await expect(table.getByText("Repeated abuse")).toBeVisible();
  await page
    .getByRole("button", { name: "Actions for Alex Candidate", exact: true })
    .click();
  await page
    .getByRole("menuitem", { name: "Unlock account", exact: true })
    .click();
  await expect(page.getByRole("dialog")).toContainText(
    "revoked sessions will stay invalid",
  );
  await page.getByRole("button", { name: "Confirm unlock" }).click();
  await expect(
    page.getByText("No matching accounts", { exact: true }),
  ).toBeVisible();
});

test("custom datetime validates the future; indefinite checkbox preserves custom draft", async ({
  page,
}) => {
  const api = await mockAccounts(page);
  await page.goto("/admin/users");
  await page
    .getByRole("button", { name: "Actions for Alex Candidate", exact: true })
    .click();
  await page
    .getByRole("menuitem", { name: "Lock account", exact: true })
    .click();
  await page.getByLabel("Reason sent to user", { exact: true }).fill("Abuse");
  await page.getByRole("button", { name: "Custom date/time" }).click();
  await page.getByLabel("Lock until", { exact: true }).fill("2020-01-01T12:00");
  await page.getByRole("button", { name: "Confirm lock" }).click();
  await expect(page.getByRole("dialog").getByRole("alert")).toContainText(
    "future",
  );
  expect(api.mutations).toHaveLength(0);
  await page.getByLabel("Lock until", { exact: true }).fill("2099-10-10T12:00");
  await page.getByLabel("Lock indefinitely", { exact: true }).check();
  await expect(page.getByLabel("Lock until", { exact: true })).toBeDisabled();
  await page.getByLabel("Lock indefinitely", { exact: true }).uncheck();
  await expect(page.getByLabel("Lock until", { exact: true })).toHaveValue(
    "2099-10-10T12:00",
  );
  await page.getByRole("button", { name: "Confirm lock" }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  expect(api.mutations[0].banExpiresIn).toBeGreaterThan(3600);
});

test("mobile indefinite lock and cancellation keep accessible focus and layout", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const api = await mockAccounts(page);
  await page.goto("/admin/users");
  const trigger = page.getByRole("button", {
    name: "Actions for Alex Candidate",
  });
  await trigger.click();
  await page
    .getByRole("menuitem", { name: "Lock account", exact: true })
    .click();
  await page.keyboard.press("Escape");
  await expect(trigger).toBeFocused();
  expect(api.mutations).toHaveLength(0);
  await trigger.click();
  await page
    .getByRole("menuitem", { name: "Lock account", exact: true })
    .click();
  await page
    .getByLabel("Reason sent to user", { exact: true })
    .fill("Policy violation");
  await page.getByLabel("Lock indefinitely", { exact: true }).check();
  await page.getByRole("button", { name: "Confirm lock" }).click();
  await expect(
    page.getByRole("button", { name: "Actions for Alex Candidate" }),
  ).toBeEnabled();
  await expect(
    page.getByRole("table").getByText("Indefinitely").first(),
  ).toBeVisible();
  expect(api.mutations[0]).toEqual({
    userId: candidateId,
    banReason: "Policy violation",
  });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
});

for (const role of ["candidate", "recruiter"] as const)
  test(`${role} cannot mount the admin table or invoke listing`, async ({
    page,
  }) => {
    const api = await mockAccounts(page, role);
    await page.goto("/admin/users");
    await expect(page).toHaveURL(
      role === "candidate" ? /\/dashboard$/ : /\/recruiter\/dashboard$/,
    );
    await expect(
      page.getByRole("table", { name: "Platform accounts" }),
    ).toHaveCount(0);
    expect(api.listCalls()).toBe(0);
  });
