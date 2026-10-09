import { expect, test, type Page } from "@playwright/test";

type Role = "candidate" | "recruiter" | "admin";
async function session(page: Page, role: Role) {
  await page.route("**/auth/me", (route) =>
    route.fulfill({
      contentType: "application/json",
      headers: {
        "access-control-allow-origin": "http://127.0.0.1:3100",
        "access-control-allow-credentials": "true",
      },
      body: JSON.stringify({
        success: true,
        data: {
          id: "9b9e9289-994d-4e62-9e65-034b9a907330",
          email: "alex@example.com",
          full_name: "Alex Doe",
          role,
          email_verified: true,
          is_locked: false,
        },
      }),
    }),
  );
}
async function chooseTheme(page: Page, theme: "Light" | "Dark" | "System") {
  await page.getByRole("button", { name: "Open account menu" }).click();
  await page.getByRole("menuitemradio", { name: theme, exact: true }).click();
}
async function expectTheme(page: Page, theme: "Light" | "Dark" | "System") {
  await page.getByRole("button", { name: "Open account menu" }).click();
  await expect(
    page.getByRole("menuitemradio", { name: theme, exact: true }),
  ).toHaveAttribute("aria-checked", "true");
  await page.keyboard.press("Escape");
}
const home = {
  candidate: "/dashboard",
  recruiter: "/recruiter/dashboard",
  admin: "/admin/dashboard",
};

for (const role of ["candidate", "recruiter", "admin"] as const) {
  test(`${role} entry resolves its menu and shared account shell`, async ({
    page,
  }) => {
    await session(page, role);
    await page.goto("/dashboard?auth=signed-in");
    await expect(page).toHaveURL(`http://127.0.0.1:3100${home[role]}`);
    const nav = page.getByRole("navigation", {
      name: `${role === "admin" ? "Administrator" : role === "recruiter" ? "Recruiter" : "Candidate"} navigation`,
    });
    await expect(
      nav.getByRole("link", { name: "Dashboard", exact: true }),
    ).toHaveAttribute("aria-current", "page");
    if (role === "recruiter") {
      await expect(
        nav.getByRole("link", { name: "My Job Postings" }),
      ).toBeVisible();
      await expect(
        nav.getByRole("link", { name: "Membership & Billing" }),
      ).toHaveCount(0);
    }
    if (role === "admin") {
      await expect(
        nav.getByRole("link", { name: "Job Posting Moderation" }),
      ).toBeVisible();
      await expect(nav.getByRole("link", { name: "Avatars" })).toHaveCount(0);
    }
    await page.getByRole("button", { name: "Open account menu" }).click();
    await page.getByRole("menuitem", { name: "Your Profile" }).click();
    await expect(page).toHaveURL(/\/profile$/);
    await expect(nav).toBeVisible();
    await expect(nav.locator('[aria-current="page"]')).toHaveCount(0);
    await page.reload();
    await expect(nav).toBeVisible();
  });
}

test("collapse, deep links, back and refresh preserve navigation state", async ({
  page,
}) => {
  await session(page, "candidate");
  await page.goto("/dashboard");
  await page.getByRole("button", { name: "Collapse sidebar" }).click();
  await expect(
    page.getByRole("complementary", { name: "Workspace sidebar" }),
  ).toHaveCSS("width", "72px");
  await page.getByRole("link", { name: "Target JDs", exact: true }).click();
  await expect(page).toHaveURL(/\/target-jds$/);
  await page.reload();
  await expect(
    page.getByRole("button", { name: "Expand sidebar" }),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Target JDs", exact: true }),
  ).toHaveAttribute("aria-current", "page");
  await page.goto("/interviews/new/setup");
  await expect(
    page.getByRole("link", { name: "Target JDs", exact: true }),
  ).toHaveAttribute("aria-current", "page");
  await page.getByRole("link", { name: "Interview History" }).click();
  await page.goBack();
  await expect(page).toHaveURL(/\/interviews\/new\/setup$/);
  await expect(
    page.getByRole("link", { name: "Target JDs", exact: true }),
  ).toHaveAttribute("aria-current", "page");
});

test("route search handles keyboard selection and only shows role destinations", async ({
  page,
}) => {
  await session(page, "recruiter");
  await page.goto(home.recruiter);
  await page.getByRole("button", { name: "Search pages" }).click();
  const input = page.getByRole("combobox", { name: "Search workspace pages" });
  await input.fill("accounts");
  await expect(page.getByText("No matching pages.")).toBeVisible();
  await input.press("ArrowDown");
  await input.press("Enter");
  await expect(page).toHaveURL(/\/recruiter\/dashboard$/);
  await input.fill("posting");
  await input.press("Enter");
  await expect(page).toHaveURL(/\/recruiter\/job-postings$/);
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await page.getByRole("button", { name: "Search pages" }).click();
  await page.keyboard.press("Escape");
  await expect(
    page.getByRole("button", { name: "Search pages" }),
  ).toBeFocused();
});

test("theme circular reveal, persistence and reduced-motion fallback work", async ({
  page,
}) => {
  await page.addInitScript(() => {
    const original = Document.prototype.startViewTransition;
    if (original)
      Document.prototype.startViewTransition = function (callback) {
        this.documentElement.dataset.testThemeTransitions = String(
          Number(this.documentElement.dataset.testThemeTransitions ?? 0) + 1,
        );
        return original.call(this, callback);
      };
  });
  await session(page, "candidate");
  await page.goto(home.candidate);
  await chooseTheme(page, "Dark");
  await expect(page.locator("html")).toHaveClass(/dark/);
  await expect(page.locator("html")).toHaveAttribute(
    "data-test-theme-transitions",
    "1",
  );
  await expect(page.locator("html")).not.toHaveAttribute("data-theme-reveal");
  await expectTheme(page, "Dark");
  await page.reload();
  await expectTheme(page, "Dark");
  await page.emulateMedia({ reducedMotion: "reduce" });
  await chooseTheme(page, "Light");
  await expect(page.locator("html")).not.toHaveClass(/dark/);
  await expect(page.locator("html")).not.toHaveAttribute(
    "data-test-theme-transitions",
  );
  await expect(page.locator("html")).not.toHaveAttribute("data-theme-reveal");
  await page.getByRole("button", { name: "Collapse sidebar" }).click();
  await expect(page.getByRole("complementary")).toHaveCSS(
    "transition-property",
    "none",
  );
});

test("unsupported theme API and blocked storage retain functional preferences", async ({
  page,
}) => {
  await page.addInitScript(() => {
    localStorage.setItem(
      "rolecue.workspace-preferences",
      JSON.stringify({ theme: "light", collapsed: false }),
    );
    Object.defineProperty(Document.prototype, "startViewTransition", {
      value: undefined,
      configurable: true,
    });
    Storage.prototype.setItem = () => {
      throw new Error("Storage disabled");
    };
  });
  await session(page, "candidate");
  await page.goto(home.candidate);
  await chooseTheme(page, "Dark");
  await expect(page.locator("html")).toHaveClass(/dark/);
  await page.getByRole("button", { name: "Collapse sidebar" }).click();
  await page.getByRole("link", { name: "Job Board" }).click();
  await expect(
    page.getByRole("button", { name: "Expand sidebar" }),
  ).toBeVisible();
  await expect(page.locator("html")).toHaveClass(/dark/);
});

for (const width of [390, 768, 1440]) {
  test(`sidebar is operable without horizontal overflow at ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 640 });
    await session(page, "admin");
    await page.goto(home.admin);
    if (width < 768) {
      await page
        .getByRole("button", { name: "Open workspace navigation" })
        .click();
      await expect(
        page.getByRole("dialog", { name: "Workspace navigation" }),
      ).toBeVisible();
      await chooseTheme(page, "Dark");
      await page.getByRole("button", { name: "Open account menu" }).click();
      await expect(
        page.getByRole("menuitem", { name: "Membership & Billing" }),
      ).toHaveCount(0);
      await page.keyboard.press("Escape");
      await page.getByRole("link", { name: "Revenue Reports" }).click();
      await expect(page.getByRole("dialog")).toHaveCount(0);
      await expect(
        page.getByRole("button", { name: "Open workspace navigation" }),
      ).toBeFocused();
    } else {
      await page.getByRole("link", { name: "Revenue Reports" }).click();
      await expect(
        page.getByRole("button", { name: "Open account menu" }),
      ).toBeVisible();
    }
    await expect(page).toHaveURL(/\/admin\/revenue$/);
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
  });
}

test("wrong-role deep links do not reveal the requested workspace", async ({
  page,
}) => {
  await session(page, "candidate");
  await page.goto("/admin/revenue");
  await expect(page).toHaveURL(/\/dashboard$/);
  await expect(
    page.getByRole("link", { name: "Accounts", exact: true }),
  ).toHaveCount(0);
  await page.goto("/recruiter/job-postings");
  await expect(page).toHaveURL(/\/dashboard$/);
  await expect(page.getByRole("link", { name: "My Job Postings" })).toHaveCount(
    0,
  );
});

test("runtime hides sidebar and public login does not inherit workspace dark mode", async ({
  page,
}) => {
  await session(page, "candidate");
  await page.goto(home.candidate);
  await chooseTheme(page, "Dark");
  await page.goto("/interviews/sample/room");
  await expect(
    page.getByRole("heading", { name: "Interview room" }),
  ).toBeVisible();
  await expect(page.getByRole("complementary")).toHaveCount(0);
  await page.goto("/login");
  await expect(page.locator("html")).not.toHaveClass(/dark/);
});

test("failed sign out remains signed in and exposes retry in account menu", async ({
  page,
}) => {
  await session(page, "candidate");
  await page.route("**/api/auth/sign-out", (route) =>
    route.fulfill({
      status: 500,
      contentType: "application/json",
      body: JSON.stringify({ message: "Unavailable" }),
    }),
  );
  await page.goto(home.candidate);
  await page.getByRole("button", { name: "Open account menu" }).click();
  await page.getByRole("menuitem", { name: "Sign out", exact: true }).click();
  await expect(
    page.getByRole("alert").filter({ hasText: "Sign out did not complete" }),
  ).toBeVisible();
  await expect(
    page.getByRole("menuitem", { name: "Sign out", exact: true }),
  ).toBeEnabled();
  await expect(page).toHaveURL(/\/dashboard$/);
});

test("mobile search returns focus to drawer and resizing clears the overlay", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await session(page, "recruiter");
  await page.goto(home.recruiter);
  await page.getByRole("button", { name: "Open workspace navigation" }).click();
  await page.getByRole("button", { name: "Search pages" }).click();
  await expect(page.getByRole("combobox")).toBeFocused();
  await page
    .locator('[data-slot="dialog-overlay"]')
    .click({ position: { x: 380, y: 10 } });
  await expect(
    page.getByRole("button", { name: "Search pages" }),
  ).toBeFocused();
  await page.setViewportSize({ width: 1024, height: 844 });
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await page.getByRole("button", { name: "Open workspace navigation" }).click();
  await page.keyboard.press("Escape");
  await expect(
    page.getByRole("button", { name: "Open workspace navigation" }),
  ).toBeFocused();
});

test("successful sign out returns to login and removes workspace chrome", async ({
  page,
}) => {
  await session(page, "candidate");
  await page.route("**/api/auth/sign-out", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ success: true }),
    }),
  );
  await page.goto(home.candidate);
  await page.getByRole("button", { name: "Open account menu" }).click();
  await page.getByRole("menuitem", { name: "Sign out", exact: true }).click();
  await expect(page).toHaveURL(/\/login$/);
  await expect(page.getByRole("complementary")).toHaveCount(0);
});

test("System defaults to OS appearance, while explicit themes persist independently", async ({
  page,
}) => {
  await page.emulateMedia({ colorScheme: "light", reducedMotion: "reduce" });
  await session(page, "candidate");
  await page.goto(home.candidate);
  await expectTheme(page, "System");
  await expect(page.locator('[data-slot="workspace-shell"]')).toHaveCSS(
    "background-color",
    "rgb(255, 255, 255)",
  );
  await page.emulateMedia({ colorScheme: "dark" });
  await expect(page.locator("html")).toHaveClass(/dark/);
  await expectTheme(page, "System");
  await chooseTheme(page, "Light");
  await expect(page.locator("html")).not.toHaveClass(/dark/);
  await page.reload();
  await expectTheme(page, "Light");
  await expect(page.locator("html")).not.toHaveClass(/dark/);
  await chooseTheme(page, "System");
  await expect(page.locator("html")).toHaveClass(/dark/);
  await page.reload();
  await expectTheme(page, "System");
  await expect(page.locator("html")).toHaveClass(/dark/);
  await page.emulateMedia({ colorScheme: "light" });
  await expect(page.locator("html")).not.toHaveClass(/dark/);
});

test("navigation icon motion responds to row hover and keyboard focus, and respects reduced motion", async ({
  page,
}) => {
  await session(page, "candidate");
  await page.goto(home.candidate);
  const link = page.getByRole("link", { name: "Target JDs", exact: true });
  const stroke = link.locator("svg path").last();
  await link.hover();
  await expect(stroke).toHaveAttribute("stroke-dasharray", /^0\./);
  await expect(stroke).toHaveAttribute("stroke-dasharray", "1 1");
  await page.getByRole("heading").first().hover();
  await link.focus();
  await expect(stroke).toHaveAttribute("stroke-dasharray", /^0\./);
  await page.emulateMedia({ reducedMotion: "reduce" });
  await expect(link.locator("svg.lucide")).toHaveCount(1);
  await expect(link.locator("svg")).toHaveAttribute("aria-hidden", "true");
});
