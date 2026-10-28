import { expect, test, type Page } from "@playwright/test";

const id = "22222222-2222-4222-8222-222222222222";
const uploadId = "33333333-3333-4333-8333-333333333333";
const pdfFixture = "../api/internal/features/roleprofile/tests/testdata/jd.pdf";
const candidate = {
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

type Profile = {
  id: string;
  title: string;
  hasOriginalDocument: boolean;
  rawText: string;
  seniorityLevel: string | null;
  requirements: Record<string, unknown>[];
  refinementNote: string;
  status: "draft" | "analyzing" | "review" | "confirmed";
  createdAt: string;
};

function roleProfile(overrides: Partial<Profile> = {}): Profile {
  return {
    id,
    title: "Backend Engineer",
    hasOriginalDocument: false,
    rawText:
      "Senior Backend Engineer. Go, PostgreSQL, concurrency and caching. ".repeat(
        4,
      ),
    seniorityLevel: "senior",
    requirements: [
      {
        name: "Go",
        target: "Build concurrent services",
        kind: "skill",
        category: "programming_language",
        type: "required",
        evidenceText: null,
      },
    ],
    refinementNote: "",
    status: "review",
    createdAt: "2026-10-10T00:00:00Z",
    ...overrides,
  };
}

function gate() {
  let open!: () => void;
  const ready = new Promise<void>((resolve) => (open = resolve));
  return { ready, open };
}

async function mockRoleProfileApi(page: Page, initial: Profile | null) {
  const state = {
    profile: initial,
    calls: [] as string[],
    errors: [] as string[],
    storagePuts: 0,
    completionFailures: 0,
    analysisFailures: 0,
    analysisGate: null as Promise<void> | null,
    confirmGate: null as Promise<void> | null,
  };
  page.on("pageerror", (error) => state.errors.push(error.message));
  await page.route("https://storage.example.test/**", async (route) => {
    const request = route.request();
    if (request.method() === "PUT") {
      state.storagePuts++;
      // Signed storage uploads must not carry API credentials.
      expect(request.headers().authorization).toBeUndefined();
      expect(request.headers().cookie).toBeUndefined();
    }
    if (request.url().endsWith("/original.pdf"))
      return route.fulfill({
        headers: { "content-type": "application/pdf" },
        path: pdfFixture,
      });
    return route.fulfill({
      headers: {
        "access-control-allow-origin": "http://127.0.0.1:3100",
        "access-control-allow-methods": "GET,PUT,OPTIONS",
        "access-control-allow-headers": "Content-Type",
      },
    });
  });
  await page.route("http://localhost:8080/**", async (route) => {
    const method = route.request().method();
    const path = new URL(route.request().url()).pathname;
    const headers = {
      "access-control-allow-origin": "http://127.0.0.1:3100",
      "access-control-allow-credentials": "true",
      "access-control-allow-methods": "GET,POST,PUT,DELETE,OPTIONS",
      "access-control-allow-headers": "Content-Type",
    };
    const ok = (data: unknown, status = 200) =>
      route.fulfill({ status, headers, json: { success: true, data } });
    const fail = (status: number, code: string, message: string) =>
      route.fulfill({
        status,
        headers,
        json: { success: false, error: { code, message } },
      });
    if (method === "OPTIONS") return route.fulfill({ status: 204, headers });
    if (path === "/auth/me") return ok(candidate);
    if (!path.startsWith("/role-profiles")) return route.abort();
    if (method !== "GET") state.calls.push(`${method} ${path}`);

    if (path === "/role-profiles" && method === "GET") {
      const items = state.profile
        ? [
            {
              id: state.profile.id,
              title: state.profile.title,
              status: state.profile.status,
              seniorityLevel: state.profile.seniorityLevel,
              createdAt: state.profile.createdAt,
            },
          ]
        : [];
      return ok({
        items,
        pagination: { limit: 20, offset: 0, total: items.length },
      });
    }
    if (path === "/role-profiles" && method === "POST") {
      const { rawText } = route.request().postDataJSON();
      state.profile = roleProfile({ rawText, status: "draft" });
      return ok(state.profile, 201);
    }
    if (path === "/role-profiles/uploads") {
      expect(route.request().postDataJSON()).toMatchObject({
        fileName: "jd.pdf",
        contentType: "application/pdf",
      });
      return ok({
        uploadId,
        request: {
          URL: "https://storage.example.test/staging.pdf",
          Method: "PUT",
          Headers: { "Content-Type": "application/pdf" },
          ExpiresAt: new Date(Date.now() + 300_000).toISOString(),
        },
      });
    }
    if (path === `/role-profiles/uploads/${uploadId}/complete`) {
      if (state.completionFailures-- > 0)
        return fail(
          503,
          "PDF_UNAVAILABLE",
          "PDF reading is unavailable. Please retry.",
        );
      state.profile = roleProfile({
        status: "draft",
        hasOriginalDocument: true,
      });
      return ok(state.profile);
    }
    if (!state.profile) return fail(404, "NOT_FOUND", "Role Profile not found");
    if (path.endsWith("/document"))
      return ok({
        URL: "https://storage.example.test/original.pdf",
        Method: "GET",
        Headers: {},
        ExpiresAt: new Date(Date.now() + 60_000).toISOString(),
      });
    if (path.endsWith("/analyze")) {
      if (state.analysisFailures-- > 0)
        return fail(
          502,
          "EXTRACTION_FAILED",
          "Analysis failed. Retry your saved draft.",
        );
      await state.analysisGate;
      state.profile = { ...state.profile, status: "review" };
    } else if (path.endsWith("/confirm")) {
      await state.confirmGate;
      state.profile = { ...state.profile, status: "confirmed" };
    } else if (method === "PUT") {
      state.profile = { ...state.profile, ...route.request().postDataJSON() };
    } else if (method === "DELETE") {
      state.profile = null;
      return ok(null);
    }
    return ok(state.profile);
  });
  return state;
}

test("pasted job description is saved, analyzed and opened for review", async ({
  page,
}) => {
  const api = await mockRoleProfileApi(page, null);
  const analysis = gate();
  api.analysisGate = analysis.ready;
  await page.goto("/role-profiles");
  await expect(
    page.getByRole("heading", { name: "Your next role starts here." }),
  ).toBeVisible();
  await page.getByRole("link", { name: "Add Role Profile" }).click();
  await expect(
    page.getByRole("heading", { name: "Bring the role into the room." }),
  ).toBeVisible();

  await page.getByLabel("Job description text").fill("Too short");
  await page.getByRole("button", { name: "Analyze role", exact: true }).click();
  await expect(
    page.getByText("Paste 100–20,000 characters of job description text."),
  ).toBeVisible();
  expect(api.calls).toEqual([]);

  await page
    .getByLabel("Job description text")
    .fill(roleProfile().rawText.trim());
  await page.getByRole("button", { name: "Analyze role", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Analyzing role…" }),
  ).toBeDisabled();
  await expect(
    page.getByText(
      "Saving your Role Profile and extracting technical requirements.",
    ),
  ).toBeVisible();
  analysis.open();

  await expect(page).toHaveURL(new RegExp(`/role-profiles/${id}$`));
  await expect(
    page.getByRole("heading", {
      name: "Confirm the role before you practice.",
    }),
  ).toBeVisible();
  expect(api.calls).toEqual([
    "POST /role-profiles",
    `POST /role-profiles/${id}/analyze`,
  ]);
  expect(api.errors).toEqual([]);
});

test("review edits are saved before confirmation, and confirmation waits for the backend", async ({
  page,
}) => {
  const api = await mockRoleProfileApi(page, roleProfile());
  const confirmation = gate();
  api.confirmGate = confirmation.ready;
  await page.goto(`/role-profiles/${id}`);

  // A target saved in the earlier free-text format stays as written.
  await expect(
    page.getByText("Build concurrent services", { exact: true }),
  ).toBeVisible();
  await page
    .getByLabel("Proficiency for requirement 1", { exact: true })
    .click();
  await page.getByRole("option", { name: "New", exact: true }).click();

  const details = page.getByRole("button", {
    name: "Details for requirement 1",
  });
  await expect(page.getByRole("combobox", { name: "Kind" })).toHaveCount(0);
  await details.click();
  await page.getByRole("combobox", { name: "Kind", exact: true }).click();
  await page
    .getByRole("option", { name: "Domain knowledge", exact: true })
    .click();
  await expect(
    page.getByRole("combobox", { name: "Skill category" }),
  ).toHaveCount(0);
  await page.getByRole("combobox", { name: "Kind", exact: true }).click();
  await page
    .getByRole("option", { name: "Technical skill", exact: true })
    .click();
  await page.getByRole("combobox", { name: "Skill category" }).click();
  await page.getByRole("option", { name: "Language", exact: true }).click();
  await page.getByRole("combobox", { name: "Priority" }).click();
  await page.getByRole("option", { name: "Unspecified", exact: true }).click();
  await details.click();

  await page.getByRole("button", { name: "Add requirement" }).click();
  await page.getByLabel("Requirement 02", { exact: true }).fill("Docker");
  await page
    .getByLabel("Proficiency for requirement 2", { exact: true })
    .click();
  await page.getByRole("option", { name: "Proficient", exact: true }).click();
  await page.getByRole("button", { name: "Details for requirement 2" }).click();
  await page.getByRole("combobox", { name: "Skill category" }).click();
  await page.getByRole("option", { name: "Tool", exact: true }).click();
  await page.getByRole("combobox", { name: "Priority" }).click();
  await page.getByRole("option", { name: "Preferred", exact: true }).click();

  await page.getByRole("button", { name: "Add requirement" }).click();
  await page.getByRole("button", { name: "Remove requirement 3" }).click();
  await expect(page.getByLabel("Requirement 03", { exact: true })).toHaveCount(
    0,
  );
  await page
    .getByLabel("Make the practice your own.")
    .fill("Focus on caching and concurrency");

  await page.getByRole("button", { name: "Confirm role" }).click();
  await expect(
    page.getByText("Preparing interview content…", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Your role review is saved." }),
  ).toHaveCount(0);
  confirmation.open();
  await expect(
    page.getByRole("heading", { name: "Your role review is saved." }),
  ).toBeVisible();

  expect(api.calls).toEqual([
    `PUT /role-profiles/${id}`,
    `POST /role-profiles/${id}/confirm`,
  ]);
  expect(api.profile).toMatchObject({
    refinementNote: "Focus on caching and concurrency",
    requirements: [
      {
        name: "Go",
        target: "New",
        kind: "skill",
        category: "programming_language",
        type: null,
      },
      {
        name: "Docker",
        target: "Proficient",
        kind: "skill",
        category: "tool",
        type: "preferred",
      },
    ],
  });
  // Candidates never see or approve the generated question bank.
  await expect(page.getByText(/question|blueprint/i)).toHaveCount(0);
  await expect(page.getByLabel("Proficiency for Docker")).toHaveText(
    "Proficient",
  );
  await expect(
    page
      .getByRole("list", { name: "Role Profile progress" })
      .locator('[aria-current="step"]'),
  ).toContainText("Confirmed");
  expect(api.errors).toEqual([]);
});

test("interview setup only opens for a confirmed Role Profile", async ({
  page,
}) => {
  const api = await mockRoleProfileApi(
    page,
    roleProfile({ status: "confirmed" }),
  );
  await page.goto(`/role-profiles/${id}`);
  await page.getByRole("link", { name: "Continue to setup" }).click();
  await expect(page).toHaveURL(/\/interviews\/new\/setup\?roleProfileId=/);
  await expect(
    page.getByRole("heading", { level: 1, name: "Choose a Role Profile" }),
  ).toBeVisible();
  await expect(page.getByRole("radio", { checked: true })).toHaveCount(1);

  api.profile = roleProfile({ status: "review" });
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Confirm this role first." }),
  ).toBeVisible();

  await page.goto("/interviews/new/setup");
  await expect(
    page.getByRole("heading", { name: "Choose your Role Profile first." }),
  ).toBeVisible();

  await page.goto(`/interviews/new/skills?roleProfileId=${id}`);
  await expect(page).toHaveURL(new RegExp(`/role-profiles/${id}$`));
  await page.goto("/target-jds");
  await expect(page).toHaveURL(/\/role-profiles$/);
  expect(api.errors).toEqual([]);
});

test("deleting from the library asks first and keeps keyboard focus", async ({
  page,
}) => {
  const api = await mockRoleProfileApi(page, roleProfile());
  await page.goto("/role-profiles");
  const remove = page.getByRole("button", {
    name: "Delete Backend Engineer",
    exact: true,
  });
  await remove.click();
  await page.getByRole("button", { name: "Keep profile", exact: true }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(remove).toBeFocused();
  expect(api.calls).toEqual([]);

  await remove.click();
  await page
    .getByRole("button", { name: "Delete Role Profile", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "Your next role starts here." }),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Add Role Profile", exact: true }),
  ).toBeFocused();
  expect(api.calls).toEqual([`DELETE /role-profiles/${id}`]);
  expect(api.errors).toEqual([]);
});

test("PDF retries reuse the uploaded file, and the original opens privately", async ({
  page,
}) => {
  const api = await mockRoleProfileApi(page, null);
  await page.goto("/role-profiles/new");
  await page.getByRole("tab", { name: "Upload PDF" }).click();
  const picker = page.getByLabel("Job description PDF");
  await picker.setInputFiles({
    name: "not-pdf.txt",
    mimeType: "text/plain",
    buffer: Buffer.from("invalid"),
  });
  await expect(page.getByText("Choose a PDF document.")).toBeVisible();
  await picker.setInputFiles(pdfFixture);
  await expect(page.getByText("jd.pdf", { exact: true })).toBeVisible();

  api.completionFailures = 1;
  api.analysisFailures = 1;
  const analyze = page.getByRole("button", {
    name: "Analyze role",
    exact: true,
  });
  await analyze.click();
  await expect(page.getByText("PDF reading is unavailable")).toBeVisible();
  await analyze.click();
  await expect(page.getByText("Analysis failed")).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Open your saved draft" }),
  ).toBeVisible();
  await analyze.click();
  await expect(page).toHaveURL(new RegExp(`/role-profiles/${id}$`));

  expect(api.storagePuts).toBe(1);
  expect(api.calls).toEqual([
    "POST /role-profiles/uploads",
    `POST /role-profiles/uploads/${uploadId}/complete`,
    `POST /role-profiles/uploads/${uploadId}/complete`,
    `POST /role-profiles/${id}/analyze`,
    `POST /role-profiles/${id}/analyze`,
  ]);

  const [, pdfRequest] = await Promise.all([
    page.waitForEvent("popup"),
    page
      .context()
      .waitForEvent(
        "request",
        (request) =>
          request.url() === "https://storage.example.test/original.pdf",
      ),
    page.getByRole("button", { name: "Open original PDF" }).click(),
  ]);
  expect(pdfRequest.method()).toBe("GET");
  expect(api.errors).toEqual([]);
});

test("Role Profile screens fit a 375px viewport", async ({ page }) => {
  await page.setViewportSize({ width: 375, height: 812 });
  const api = await mockRoleProfileApi(page, roleProfile());
  for (const path of [
    "/role-profiles",
    "/role-profiles/new",
    `/role-profiles/${id}`,
  ]) {
    await page.goto(path);
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    await expect(page.locator("body")).toHaveJSProperty("scrollWidth", 375);
  }
  expect(api.errors).toEqual([]);
});
