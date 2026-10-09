import { test, expect, type Page } from "@playwright/test";
import { randomUUID } from "node:crypto";
import { Pool } from "pg";
import { hashPassword } from "better-auth/crypto";

test.skip(
  process.env.ONBOARDING_LIVE_TEST !== "1" || !process.env.DATABASE_URL,
  "Requires explicit ONBOARDING_LIVE_TEST=1 and the local Docker DATABASE_URL",
);
const pool = new Pool({ connectionString: process.env.DATABASE_URL });
const fixtures: string[] = [];
const password = `Onboarding-${randomUUID()}`;
const api = "http://localhost:8080";
const headers = { Origin: "http://localhost:3000" };

test.afterAll(async () => {
  // Delete only UUIDs this test created; account/session FKs cascade.
  if (fixtures.length)
    await pool.query("DELETE FROM users WHERE id = ANY($1::uuid[])", [
      fixtures,
    ]);
  await pool.end();
});

async function createUser(role = "candidate", name = "New Candidate") {
  const id = randomUUID();
  const email = `onboarding-${id}@example.test`;
  const hash = await hashPassword(password);
  await pool.query(
    "INSERT INTO users(id,name,email,email_verified,role) VALUES($1,$2,$3,true,$4)",
    [id, name, email, role],
  );
  fixtures.push(id);
  await pool.query(
    "INSERT INTO accounts(account_id,provider_id,user_id,password,updated_at) VALUES($1::text,'credential',$1::uuid,$2,now())",
    [id, hash],
  );
  return { id, email };
}

async function login(page: Page, email: string) {
  await page.goto("/login");
  await page.getByLabel("Email address", { exact: true }).fill(email);
  await page.getByLabel("Password", { exact: true }).fill(password);
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
}

async function markers(id: string) {
  const result = await pool.query(
    "SELECT name,role,image,company_name,company_website,onboarding_status FROM users WHERE id=$1",
    [id],
  );
  return result.rows[0];
}

test("real credential login: candidate selects once, resumes and completes with Peek", async ({
  page,
}) => {
  const user = await createUser();
  await login(page, user.email);
  await expect(page.getByRole("dialog")).toBeVisible();
  expect((await markers(user.id)).onboarding_status).toBe("pending");
  const denied = await page.request.get(`${api}/jds`);
  expect(denied.status()).toBe(403);
  await page.screenshot({ path: "/tmp/rolecue-onboarding-role.png" });
  await page.getByRole("radio", { name: /^candidate /i }).press("Space");
  await page.getByRole("button", { name: "Continue as candidate" }).click();
  await expect(page.getByLabel("Display name")).toBeVisible();
  await page.reload();
  await expect(page.getByRole("radio")).toHaveCount(0);
  const selected = await markers(user.id);
  expect(selected.onboarding_status).toBe("role_selected");
  const sameRole = await page.request.post(`${api}/onboarding/role`, {
    headers,
    data: { role: "candidate" },
  });
  expect(sameRole.status()).toBe(200);
  expect((await markers(user.id)).onboarding_status).toBe("role_selected");
  const changedRole = await page.request.post(`${api}/onboarding/role`, {
    headers,
    data: { role: "recruiter" },
  });
  expect(changedRole.status()).toBe(403);
  await page.getByLabel("Display name").fill("Alex Candidate");
  await page.getByRole("button", { name: "Open my workspace" }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(
    page.getByRole("navigation", { name: "Candidate navigation" }),
  ).toBeVisible();
  expect(await markers(user.id)).toMatchObject({
    name: "Alex Candidate",
    role: "candidate",
    image: null,
  });
  expect((await markers(user.id)).onboarding_status).toBe("completed");
  await page.reload();
  await expect(page.getByRole("dialog")).toHaveCount(0);
});

test("real recruiter: preview never writes; website and metadata complete atomically", async ({
  page,
}) => {
  // A real name still requires onboarding: onboarding status is the source of truth.
  const user = await createUser("candidate", "Riley Recruiter");
  await login(page, user.email);
  await expect(page.getByRole("dialog")).toBeVisible();
  await page.getByRole("radio", { name: /^recruiter /i }).press("Space");
  await page.getByRole("button", { name: "Continue as recruiter" }).click();
  await expect(page.getByLabel("Display name")).toHaveValue("Riley Recruiter");
  const missingWebsite = await page.request.patch(`${api}/profile`, {
    headers,
    data: { full_name: "Riley Recruiter", company_name: "Acme" },
  });
  expect(missingWebsite.status()).toBe(400);
  await page.getByLabel("Company name", { exact: true }).fill("Acme");
  await page
    .getByLabel("Company website", { exact: true })
    .fill("https://example.com");
  await expect(
    page.getByRole("complementary", { name: "Company website preview" }),
  ).toBeVisible();
  await expect(
    page
      .getByRole("complementary", { name: "Company website preview" })
      .getByRole("link"),
  ).toHaveAttribute("href", "https://example.com/");
  expect(await markers(user.id)).toMatchObject({
    company_website: null,
    onboarding_status: "role_selected",
  });
  await page.screenshot({ path: "/tmp/rolecue-onboarding-recruiter.png" });
  await page.setViewportSize({ width: 320, height: 720 });
  await page.screenshot({
    path: "/tmp/rolecue-onboarding-mobile.png",
    fullPage: true,
  });
  await page.setViewportSize({ width: 1280, height: 720 });
  await page.getByRole("button", { name: "Open my workspace" }).click();
  await expect(page).toHaveURL(/\/recruiter\/dashboard$/);
  await expect(
    page.getByRole("navigation", { name: "Recruiter navigation" }),
  ).toBeVisible();
  const completed = await markers(user.id);
  expect(completed).toMatchObject({
    role: "recruiter",
    company_name: "Acme",
    company_website: "https://example.com",
  });
  expect(completed.onboarding_status).toBe("completed");
  const changedRole = await page.request.post(`${api}/onboarding/role`, {
    headers,
    data: { role: "candidate" },
  });
  expect(changedRole.status()).toBe(403);
});

test("admin bypasses onboarding with pending status", async ({ page }) => {
  const user = await createUser("admin", "Admin fixture");
  await login(page, user.email);
  await expect(
    page.getByRole("navigation", { name: "Administrator navigation" }),
  ).toBeVisible();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  expect((await markers(user.id)).onboarding_status).toBe("pending");
  const role = await page.request.post(`${api}/onboarding/role`, {
    headers,
    data: { role: "candidate" },
  });
  expect(role.status()).toBe(403);
});
