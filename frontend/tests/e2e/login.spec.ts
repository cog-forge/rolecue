import { expect, test } from "@playwright/test";

test("login validates locally and OAuth links provide review preview flows", async ({
  page,
}) => {
  const loginRequests: string[] = [];
  page.on("request", (request) => {
    if (new URL(request.url()).pathname === "/auth/login")
      loginRequests.push(request.url());
  });

  await page.goto("/login");

  // Verify interactive OAuth preview links are present.
  for (const provider of ["Google", "GitHub", "Facebook"]) {
    await expect(
      page.getByRole("link", { name: `Continue with ${provider}` }),
    ).toBeVisible();
  }

  for (const provider of ["google", "github", "facebook"] as const) {
    const label = provider[0].toUpperCase() + provider.slice(1);
    await page.getByRole("link", { name: `Continue with ${label}` }).click();
    await expect(page).toHaveURL(new RegExp(`/login/oauth/${provider}$`));
    await expect(page.getByText("Authorization Preview")).toBeVisible();
    await expect(
      page.getByText("OAuth sign-in is not connected yet.").first(),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: `Preview ${label} handoff` }),
    ).toBeVisible();
    await page
      .getByRole("button", { name: `Preview ${label} handoff` })
      .click();
    await expect(page.getByText("Handoff Preview:")).toBeVisible();
    await page.getByRole("link", { name: "Back to sign in" }).click();
    await expect(page).toHaveURL(/\/login$/);
  }

  // Form local validation
  await page.getByLabel("Email address").fill("invalid-email");
  await page.getByLabel("Password", { exact: true }).fill("simple");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    page.getByText("Please enter a valid email address"),
  ).toBeVisible();
  expect(loginRequests).toHaveLength(0);
  await expect(page).toHaveURL(/\/login$/);
});
