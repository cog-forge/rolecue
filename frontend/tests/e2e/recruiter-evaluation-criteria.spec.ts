import { test, expect, type Page } from "@playwright/test";

const recruiter = {
  id: "22222222-2222-4222-8222-222222222222",
  full_name: "Rec Ruiter",
  email: "rec@example.com",
  image: null,
  role: "recruiter",
  email_verified: true,
  is_locked: false,
  onboarding_role_selected: true,
  onboarding_completed: true,
};
const postingId = "33333333-3333-4333-8333-333333333333";
const weightsPath = `/job-postings/${postingId}/evaluation-weights`;
const accuracy = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const depth = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
const clarity = "cccccccc-cccc-4ccc-8ccc-cccccccccccc";
const catalog = [
  { id: accuracy, name: "Technical accuracy" },
  { id: depth, name: "Depth of knowledge" },
  { id: clarity, name: "Clarity" },
];

async function mockWeights(
  page: Page,
  options: {
    editable?: boolean;
    saved?: { metric_id: string; name: string; percentage: number }[];
  } = {},
) {
  const editable = options.editable ?? true;
  let saved = options.saved ?? [];
  const bodies: unknown[] = [];
  let resets = 0;
  const view = () => ({
    configured: saved.length > 0,
    using_defaults: saved.length === 0,
    editable,
    posting_status: editable ? "pending" : "open",
    total: 100,
    weights:
      saved.length > 0
        ? saved
        : catalog.map((m) => ({
            metric_id: m.id,
            name: m.name,
            percentage: 100 / 3,
          })),
  });
  await page.route("http://localhost:8080/**", async (route) => {
    const url = new URL(route.request().url());
    const method = route.request().method();
    const headers = {
      "access-control-allow-origin": "http://127.0.0.1:3100",
      "access-control-allow-credentials": "true",
      "access-control-allow-methods": "GET,PUT,DELETE,OPTIONS",
      "access-control-allow-headers": "Content-Type",
    };
    if (method === "OPTIONS") return route.fulfill({ status: 204, headers });
    if (url.pathname === "/auth/me")
      return route.fulfill({
        headers,
        json: { success: true, data: recruiter },
      });
    if (url.pathname === "/evaluation-metrics")
      return route.fulfill({
        headers,
        json: { success: true, data: { items: catalog } },
      });
    if (url.pathname === weightsPath && method === "GET")
      return route.fulfill({ headers, json: { success: true, data: view() } });
    if (url.pathname === weightsPath && method === "PUT") {
      const body = route.request().postDataJSON() as {
        weights: { metric_id: string; percentage: number }[];
      };
      bodies.push(body);
      saved = body.weights.map((w) => ({
        ...w,
        name: catalog.find((m) => m.id === w.metric_id)?.name ?? "",
      }));
      return route.fulfill({ headers, json: { success: true, data: view() } });
    }
    if (url.pathname === weightsPath && method === "DELETE") {
      resets += 1;
      saved = [];
      return route.fulfill({ headers, json: { success: true, data: view() } });
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
  return { bodies, resets: () => resets };
}

test("recruiter replaces the default weights with their own", async ({
  page,
}) => {
  const api = await mockWeights(page);
  await page.goto(`/recruiter/job-postings/${postingId}/evaluation`);

  await expect(
    page.getByRole("heading", { level: 1, name: "Evaluation criteria" }),
  ).toBeVisible();
  await expect(page.getByText("Using the default weights")).toBeVisible();
  const save = page.getByRole("button", { name: "Save criteria" });
  await expect(save).toBeDisabled();

  // 50 + 30 leaves 20 unassigned: the total says so and Save stays off.
  await page.getByLabel("Technical accuracy").fill("50");
  await page.getByLabel("Depth of knowledge").fill("30");
  await page.getByLabel("Clarity").fill("0");
  await expect(
    page.getByText("The weights add up to 80, but they must add up to 100."),
  ).toBeVisible();
  await expect(save).toBeDisabled();

  await page.getByLabel("Clarity").fill("20");
  await expect(save).toBeEnabled();
  await save.click();
  await expect(page.getByText("Using the default weights")).toHaveCount(0);
  expect(api.bodies).toEqual([
    {
      weights: [
        { metric_id: accuracy, percentage: 50 },
        { metric_id: depth, percentage: 30 },
        { metric_id: clarity, percentage: 20 },
      ],
    },
  ]);

  // Reloading shows the saved weights, not the defaults.
  await page.reload();
  await expect(page.getByLabel("Technical accuracy")).toHaveValue("50");
  await expect(page.getByLabel("Clarity")).toHaveValue("20");
});

test("a metric left at 0% is not sent", async ({ page }) => {
  const api = await mockWeights(page);
  await page.goto(`/recruiter/job-postings/${postingId}/evaluation`);
  await page.getByLabel("Technical accuracy").fill("60");
  await page.getByLabel("Depth of knowledge").fill("40");
  await page.getByLabel("Clarity").fill("0");
  await page.getByRole("button", { name: "Save criteria" }).click();
  await expect.poll(() => api.bodies.length).toBe(1);
  expect(api.bodies[0]).toEqual({
    weights: [
      { metric_id: accuracy, percentage: 60 },
      { metric_id: depth, percentage: 40 },
    ],
  });
});

test("a field that cannot be sent is explained next to it", async ({
  page,
}) => {
  await mockWeights(page);
  await page.goto(`/recruiter/job-postings/${postingId}/evaluation`);
  await page.getByLabel("Clarity").fill("-4");
  await expect(page.getByText("A weight can't be negative.")).toBeVisible();
  await expect(page.getByLabel("Clarity")).toHaveAttribute(
    "aria-invalid",
    "true",
  );
  await expect(
    page.getByRole("button", { name: "Save criteria" }),
  ).toBeDisabled();
});

test("an approved posting shows its criteria read-only", async ({ page }) => {
  await mockWeights(page, { editable: false });
  await page.goto(`/recruiter/job-postings/${postingId}/evaluation`);
  await expect(
    page.getByText(/so its evaluation criteria can no longer be edited/),
  ).toBeVisible();
  await expect(page.getByRole("spinbutton")).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Save criteria" })).toHaveCount(
    0,
  );
});

test("recruiter resets saved weights to the equal-split defaults", async ({
  page,
}) => {
  const api = await mockWeights(page, {
    saved: [
      { metric_id: accuracy, name: "Technical accuracy", percentage: 70 },
      { metric_id: depth, name: "Depth of knowledge", percentage: 30 },
    ],
  });
  await page.goto(`/recruiter/job-postings/${postingId}/evaluation`);
  await expect(page.getByLabel("Technical accuracy")).toHaveValue("70");
  await expect(page.getByText("Using the default weights")).toHaveCount(0);

  await page.getByRole("button", { name: "Reset to default weights" }).click();
  const dialog = page.getByRole("dialog", {
    name: "Reset to the default weights?",
  });
  await expect(dialog).toBeVisible();
  await dialog.getByRole("button", { name: "Reset weights" }).click();

  await expect(page.getByText("Using the default weights")).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Reset to default weights" }),
  ).toHaveCount(0);
  expect(api.resets()).toBe(1);
  expect(
    Number(await page.getByLabel("Technical accuracy").inputValue()),
  ).toBeCloseTo(100 / 3, 10);
});

test("unsaved weights are guarded against in-app links and the Back button", async ({
  page,
}) => {
  await mockWeights(page);
  const evaluation = `/recruiter/job-postings/${postingId}/evaluation`;
  await page.goto("/recruiter/applications");
  await page.goto(evaluation);
  await page.getByLabel("Clarity").fill("5");
  const leaveDialog = page.getByRole("dialog", {
    name: "Leave without saving?",
  });

  // A sidebar link asks first; staying keeps the page and the draft.
  await page
    .getByRole("link", { name: "Received Applications" })
    .first()
    .click();
  await expect(leaveDialog).toBeVisible();
  await leaveDialog.getByRole("button", { name: "Stay on this page" }).click();
  await expect(leaveDialog).toHaveCount(0);
  await expect(page).toHaveURL(evaluation);
  await expect(page.getByLabel("Clarity")).toHaveValue("5");

  // The browser's Back button asks too; leaving goes back.
  await page.goBack();
  await expect(leaveDialog).toBeVisible();
  await expect(page).toHaveURL(evaluation);
  await expect(page.getByLabel("Clarity")).toHaveValue("5");
  await leaveDialog
    .getByRole("button", { name: "Leave without saving" })
    .click();
  await expect(page).toHaveURL("/recruiter/applications");
});
