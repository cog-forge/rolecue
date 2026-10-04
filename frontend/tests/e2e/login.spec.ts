import { expect, test } from "@playwright/test";

test("auth screens expose Better Auth entry points and validate without sending requests", async ({
  page,
}) => {
  const authRequests: string[] = [];
  page.on("request", (request) => {
    if (new URL(request.url()).pathname.startsWith("/api/auth/"))
      authRequests.push(request.url());
  });

  await page.goto("/login");
  await page.goto("/login?oauth=failed");
  await expect(page.locator('p[role="alert"]')).toHaveText(
    "Social sign-in could not be completed. Please try again.",
  );
  await page.goto("/login");
  await page.getByLabel("Email address").fill("invalid-email");
  await page.getByLabel("Password", { exact: true }).fill("simple");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    page.getByText("Please enter a valid email address"),
  ).toBeVisible();
  expect(authRequests).toHaveLength(0);

  await page.getByRole("link", { name: "Register" }).click();
  await expect(
    page.getByRole("heading", { name: "Create your RoleCue account" }),
  ).toBeVisible();
  await expect(page.getByLabel("Full name")).toHaveCount(0);
  await expect(page.getByLabel("I am a")).toHaveCount(0);

  await page.getByRole("link", { name: "Sign in", exact: true }).click();
  await page.getByRole("link", { name: "Forgot password?" }).click();
  await expect(
    page.getByRole("heading", { name: "Reset your password" }),
  ).toBeVisible();
});
