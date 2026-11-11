import { expect, test, type Page } from "@playwright/test";
const id = "11111111-1111-4111-8111-111111111111";
const metric = "22222222-2222-4222-8222-222222222222";
const headers = {
  "access-control-allow-origin": "http://127.0.0.1:3100",
  "access-control-allow-credentials": "true",
  "access-control-allow-methods": "GET,POST,PATCH,DELETE,OPTIONS",
  "access-control-allow-headers": "Content-Type",
};
async function mock(
  page: Page,
  role: "admin" | "recruiter",
  failure?: "RUBRIC_STALE" | "FORBIDDEN",
  locked = false,
) {
  let current = {
    id,
    name: "Mechanism rubric",
    description: "Clear scoring standards",
    question_type: "concept_explanation",
    scope: role === "admin" ? "system" : "recruiter",
    is_active: true,
    updated_at: "2026-11-11T06:00:00Z",
    created_at: "2026-11-11T06:00:00Z",
    criteria_count: 1,
    max_points: 3,
    is_own: role === "recruiter",
    can_manage: true,
    locked,
    reference_count: locked ? 2 : 0,
    criteria: [
      {
        id: metric,
        metric_id: metric,
        name: "Accuracy",
        max_points: 3,
        scoring_guide: "0 absent, 1 partial, 2 mostly right, 3 complete",
      },
    ],
  };
  let fail = failure;
  let deleted = false;
  const writes: unknown[] = [];
  await page.route("http://localhost:8080/**", async (route) => {
    const path = new URL(route.request().url()).pathname,
      method = route.request().method();
    const ok = (data: unknown) =>
      route.fulfill({ headers, json: { success: true, data } });
    if (method === "OPTIONS") return route.fulfill({ status: 204, headers });
    if (path === "/auth/me")
      return ok({
        id,
        full_name: "Review user",
        email: "review@example.com",
        image: null,
        role,
        email_verified: true,
        is_locked: false,
        onboarding_role_selected: true,
        onboarding_completed: true,
      });
    if (path === "/evaluation-metrics")
      return ok({ items: [{ id: metric, name: "Technical Accuracy" }] });
    if (path === "/rubrics" && method === "GET")
      return ok({
        items: deleted ? [] : [current],
        pagination: { total: deleted ? 0 : 1, limit: 20, offset: 0 },
      });
    if (path.endsWith("/copies")) {
      current = {
        ...current,
        id: "33333333-3333-4333-8333-333333333333",
        name: "Copy of Mechanism rubric",
        locked: false,
        reference_count: 0,
      };
      return ok(current);
    }
    if (
      path.startsWith("/rubrics") &&
      (method === "PATCH" || method === "POST")
    ) {
      const body = route.request().postDataJSON();
      writes.push(body);
      if (fail) {
        const code = fail;
        fail = undefined;
        return route.fulfill({
          status: code === "FORBIDDEN" ? 403 : 409,
          headers,
          json: {
            success: false,
            error: { code, message: "server private text" },
          },
        });
      }
      current = { ...current, ...body };
      return ok(current);
    }
    if (method === "DELETE") {
      deleted = true;
      return ok(null);
    }
    if (path.startsWith("/rubrics/")) return ok(current);
    return ok({ items: [] });
  });
  return writes;
}
async function open(page: Page, role: string) {
  await page.goto(`/${role}/rubrics`);
  await expect(
    page.getByRole("button", { name: "Mechanism rubric", exact: true }),
  ).toBeVisible();
}
for (const role of ["admin", "recruiter"] as const) {
  test(`${role}: create, edit, archive and delete`, async ({ page }) => {
    const writes = await mock(page, role);
    await open(page, role);
    await page
      .getByRole("button", { name: "Create rubric", exact: true })
      .click();
    await page.getByLabel("Rubric name", { exact: true }).fill("New rubric");
    await page.getByLabel("Criterion name", { exact: true }).fill("Clarity");
    await page
      .getByRole("combobox", { name: "Evaluation metric", exact: true })
      .click();
    await page
      .getByRole("option", { name: "Technical Accuracy", exact: true })
      .click();
    await page
      .getByLabel("Scoring guide", { exact: true })
      .fill("0 unclear, 1 clear");
    await page
      .getByRole("button", { name: "Save rubric", exact: true })
      .click();
    await expect(
      page.getByRole("button", { name: "New rubric", exact: true }),
    ).toBeVisible();
    await page.getByRole("button", { name: "Edit", exact: true }).click();
    await page.getByLabel("Rubric name", { exact: true }).fill("Edited rubric");
    await page
      .getByRole("button", { name: "Save rubric", exact: true })
      .click();
    await expect(
      page.getByRole("button", { name: "Edited rubric", exact: true }),
    ).toBeVisible();
    expect(writes).toHaveLength(2);
    expect(writes[1]).toHaveProperty("updated_at");
    await page
      .getByRole("button", { name: "Archive: Edited rubric", exact: true })
      .click();
    await page.getByRole("button", { name: "Confirm", exact: true }).click();
    await expect(page.getByRole("dialog")).not.toBeVisible();
    await page
      .getByRole("button", { name: "Delete: Edited rubric", exact: true })
      .click();
    await page.getByRole("button", { name: "Confirm", exact: true }).click();
    await expect(
      page.getByText("Your next evaluation starts here"),
    ).toBeVisible();
  });
  test(`${role}: locked rubric explains references and can be copied`, async ({
    page,
  }) => {
    await mock(page, role, undefined, true);
    await open(page, role);
    await expect(
      page.getByRole("button", { name: "Delete: Mechanism rubric" }),
    ).toBeDisabled();
    await page.getByRole("button", { name: "View", exact: true }).click();
    await expect(
      page.getByLabel("Rubric name", { exact: true }),
    ).toBeDisabled();
    await expect(page.getByText(/Used by 2 questions/)).toBeVisible();
    await page
      .getByRole("button", { name: "Copy and edit", exact: true })
      .click();
    await expect(page.getByLabel("Rubric name", { exact: true })).toHaveValue(
      "Copy of Mechanism rubric",
    );
    await expect(page.getByLabel("Rubric name", { exact: true })).toBeEnabled();
  });
}
test("stale edits remain until the user explicitly reapplies", async ({
  page,
}) => {
  const writes = await mock(page, "recruiter", "RUBRIC_STALE");
  await open(page, "recruiter");
  await page.getByRole("button", { name: "Edit", exact: true }).click();
  await page
    .getByLabel("Rubric name", { exact: true })
    .fill("My retained draft");
  await page.getByRole("button", { name: "Save rubric", exact: true }).click();
  await expect(page.getByText(/Your edits are still here/)).toBeVisible();
  await expect(page.getByLabel("Rubric name", { exact: true })).toHaveValue(
    "My retained draft",
  );
  expect(writes).toHaveLength(1);
  await page
    .getByRole("button", { name: "Keep edits with latest revision" })
    .click();
  await page.getByRole("button", { name: "Save rubric", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "My retained draft", exact: true }),
  ).toBeVisible();
});
test("permission errors retain draft and never display raw server messages", async ({
  page,
}) => {
  await mock(page, "recruiter", "FORBIDDEN");
  await open(page, "recruiter");
  await page.getByRole("button", { name: "Edit", exact: true }).click();
  await page.getByLabel("Rubric name", { exact: true }).fill("Retained draft");
  await page.getByRole("button", { name: "Save rubric", exact: true }).click();
  await expect(
    page.getByRole("alert").filter({ hasText: "permission" }),
  ).toBeVisible();
  await expect(page.getByLabel("Rubric name", { exact: true })).toHaveValue(
    "Retained draft",
  );
  await expect(page.getByText("server private text")).toHaveCount(0);
});
test("Vietnamese mobile form, validation and discard protection", async ({
  page,
}) => {
  await page
    .context()
    .addCookies([
      { name: "NEXT_LOCALE", value: "vi", domain: "127.0.0.1", path: "/" },
    ]);
  await page.setViewportSize({ width: 390, height: 844 });
  await mock(page, "admin");
  await open(page, "admin");
  await page.getByRole("button", { name: "Tạo barem", exact: true }).click();
  await page.getByRole("button", { name: "Lưu barem", exact: true }).click();
  await expect(
    page.getByRole("alert").filter({ hasText: "Một số trường" }),
  ).toBeVisible();
  await page.getByLabel("Tên barem", { exact: true }).fill("Barem mới");
  await page
    .getByRole("button", { name: "Về thư viện", exact: true })
    .first()
    .click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await page.getByRole("button", { name: "Hủy", exact: true }).click();
  await expect(page.getByLabel("Tên barem", { exact: true })).toHaveValue(
    "Barem mới",
  );
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
});
test("Recruiter system templates are read-only and expose copy", async ({
  page,
}) => {
  await mock(page, "recruiter");
  await page.route(`http://localhost:8080/rubrics/${id}`, (route) =>
    route.fulfill({
      headers,
      json: {
        success: true,
        data: {
          id,
          name: "System template",
          description: "Shared template",
          question_type: "concept_explanation",
          scope: "system",
          is_active: true,
          updated_at: "2026-11-11T06:00:00Z",
          created_at: "2026-11-11T06:00:00Z",
          criteria_count: 1,
          max_points: 3,
          is_own: false,
          can_manage: false,
          locked: false,
          criteria: [
            {
              id: metric,
              metric_id: metric,
              name: "Accuracy",
              max_points: 3,
              scoring_guide: "0 absent; 3 complete",
            },
          ],
        },
      },
    }),
  );
  await open(page, "recruiter");
  await page.getByRole("button", { name: "Edit", exact: true }).click();
  await expect(page.getByLabel("Rubric name", { exact: true })).toBeDisabled();
  await expect(
    page.getByRole("button", { name: "Save rubric", exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "Copy and edit" }),
  ).toBeVisible();
});
test("dirty navigation asks before leaving and respects reduced motion", async ({
  page,
}) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await mock(page, "admin");
  await open(page, "admin");
  await page
    .getByRole("button", { name: "Create rubric", exact: true })
    .click();
  await page.getByLabel("Rubric name", { exact: true }).fill("Draft");
  await page.getByRole("link", { name: "Dashboard", exact: true }).click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await page.getByRole("button", { name: "Cancel", exact: true }).click();
  await expect(page.getByLabel("Rubric name", { exact: true })).toHaveValue(
    "Draft",
  );
  expect(
    await page
      .locator(".t-panel-slide")
      .evaluate((el) => getComputedStyle(el).transitionDuration),
  ).toBe("0s");
});

test("metric Select supports keyboard selection and touch target sizing", async ({
  page,
}) => {
  await mock(page, "admin");
  await open(page, "admin");
  await page
    .getByRole("button", { name: "Create rubric", exact: true })
    .click();
  const metricSelect = page.getByRole("combobox", {
    name: "Evaluation metric",
    exact: true,
  });
  await metricSelect.focus();
  await metricSelect.press("ArrowDown");
  await page
    .getByRole("option", { name: "Technical Accuracy", exact: true })
    .press("Enter");
  await expect(metricSelect).toHaveText("Technical Accuracy");
  expect((await metricSelect.boundingBox())?.height).toBeGreaterThanOrEqual(44);
});

test("create actions remain reachable and removal has destructive hover feedback", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await mock(page, "admin");
  await open(page, "admin");
  await page
    .getByRole("button", { name: "Create rubric", exact: true })
    .click();
  await page
    .getByRole("button", { name: "Add criterion", exact: true })
    .click();
  await page.getByLabel("Maximum points", { exact: true }).nth(1).fill("3");
  await expect(
    page
      .getByRole("complementary", { name: "At a glance" })
      .getByText("4", { exact: true }),
  ).toBeVisible();
  const remove = page.getByRole("button", {
    name: "Remove criterion 2",
    exact: true,
  });
  await remove.hover();
  const destructiveColor = await page.evaluate(() => {
    const probe = document.createElement("span");
    probe.style.color = "var(--destructive)";
    document.body.append(probe);
    const color = getComputedStyle(probe).color;
    probe.remove();
    return color;
  });
  await expect(remove).toHaveCSS("color", destructiveColor);
  await expect(remove).not.toHaveCSS("background-color", "rgba(0, 0, 0, 0)");
  await remove.click();
  await expect(
    page
      .getByRole("complementary", { name: "At a glance" })
      .getByText("1", { exact: true }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Add criterion", exact: true })
    .scrollIntoViewIfNeeded();
  const save = page.getByRole("button", { name: "Save rubric", exact: true });
  await save.focus();
  await expect(save).toBeInViewport();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
});
