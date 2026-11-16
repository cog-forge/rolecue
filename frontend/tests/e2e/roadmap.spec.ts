import { expect, test, type Page } from "@playwright/test";
import {
  roadmapFixture,
  profileFixture,
  profileId,
  itemId,
  secondItemId,
  sessionId,
  sessionFixture,
} from "../fixtures/roadmap";
import type { Roadmap } from "@/features/roadmap/api/roadmap";

test.use({
  launchOptions: {
    executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH,
    args: [
      "--use-fake-device-for-media-stream",
      "--use-fake-ui-for-media-stream",
    ],
  },
  permissions: ["microphone"],
});
const candidate = {
  id: "11111111-1111-4111-8111-111111111111",
  full_name: "Roadmap Candidate",
  email: "roadmap@example.test",
  image: null,
  role: "candidate",
  email_verified: true,
  is_locked: false,
  onboarding_role_selected: true,
  onboarding_completed: true,
};
const row = {
  interview_id: sessionId,
  question_number: 1,
  result: "scored",
  score: 84,
  question_text: "How does an index improve a query?",
  evaluated_at: "2026-11-13T05:10:00Z",
  in_current_window: true,
};
async function mockApi(
  page: Page,
  initial: Roadmap = roadmapFixture(),
  options: { resumed?: boolean; startError?: string } = {},
) {
  const state = {
    roadmap: initial,
    starts: [] as { body: Record<string, unknown>; key: string | undefined }[],
    errors: [] as string[],
    regenerates: 0,
  };
  page.on("pageerror", (error) => state.errors.push(error.message));
  await page.route(
    `${process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080"}/**`,
    async (route) => {
      const request = route.request();
      const url = new URL(request.url());
      const path = url.pathname;
      const headers = {
        "access-control-allow-origin": "http://127.0.0.1:3100",
        "access-control-allow-credentials": "true",
        "access-control-allow-methods": "GET,POST,OPTIONS",
        "access-control-allow-headers": "Content-Type,Idempotency-Key",
      };
      const ok = (data: unknown, status = 200) =>
        route.fulfill({ status, headers, json: { success: true, data } });
      if (request.method() === "OPTIONS")
        return route.fulfill({ status: 204, headers });
      if (path === "/auth/me") return ok(candidate);
      if (path === `/role-profiles/${profileId}`) return ok(profileFixture);
      if (path === "/role-profiles")
        return ok({
          items: [
            {
              id: profileFixture.id,
              title: profileFixture.title,
              seniorityLevel: profileFixture.seniorityLevel,
              status: profileFixture.status,
              createdAt: profileFixture.createdAt,
            },
          ],
          pagination: { limit: 20, offset: 0, total: 1 },
        });
      if (path === `/role-profiles/${profileId}/roadmap`)
        return ok(state.roadmap);
      if (
        path.endsWith("/roadmap/retry") ||
        path.endsWith("/roadmap/regenerate")
      ) {
        state.regenerates++;
        state.roadmap = roadmapFixture({ practice_started: false });
        return ok({
          roadmap_id: state.roadmap.id,
          role_profile_id: profileId,
          generation_status: "ready",
        });
      }
      if (path.endsWith("/history")) {
        const isFirst = path.includes(itemId);
        const offset = Number(url.searchParams.get("offset") ?? "0");
        return ok({
          items: isFirst
            ? [
                offset === 0
                  ? row
                  : { ...row, question_number: 2, in_current_window: false },
              ]
            : [],
          pagination: {
            offset,
            limit: Number(url.searchParams.get("limit")),
            total: isFirst ? 21 : 0,
          },
        });
      }
      if (path === "/wallet")
        return ok({ id: "wallet", user_id: candidate.id, balance: 20 });
      if (path === "/inventory/avatars")
        return ok({
          items: [],
          pagination: { page: 1, page_size: 20, total: 0, total_pages: 0 },
        });
      if (path === "/voices")
        return ok({ items: [], pagination: { total: 0 } });
      if (path === "/interviews" && request.method() === "GET")
        return ok({
          items: [],
          pagination: { page: 1, page_size: 100, total: 0, total_pages: 0 },
        });
      if (path === "/interviews" && request.method() === "POST") {
        const body: Record<string, unknown> = request.postDataJSON();
        state.starts.push({ body, key: request.headers()["idempotency-key"] });
        if (options.startError && state.starts.length === 1)
          return route.fulfill({
            status: 409,
            headers,
            json: {
              success: false,
              error: {
                code: options.startError,
                message: "backend private text",
              },
            },
          });
        return ok(
          {
            ...sessionFixture,
            focus_item_id: options.resumed
              ? null
              : (body.focus_item_id ?? null),
            resumed: Boolean(options.resumed),
          },
          options.resumed ? 200 : 201,
        );
      }
      if (path === `/interviews/${sessionId}`) return ok(sessionFixture);
      if (path === `/interviews/${sessionId}/report`)
        return ok({
          interview_id: sessionId,
          type: "practice",
          status: "completed",
          overall_score: 84,
          started_at: sessionFixture.started_at,
          completed_at: "2026-11-13T05:10:00Z",
          summary: "Practice complete.",
          breakdown: [],
          questions: [],
        });
      return route.abort();
    },
  );
  return state;
}
async function locale(page: Page, value: "en" | "vi") {
  await page
    .context()
    .addCookies([
      { name: "NEXT_LOCALE", value, domain: "127.0.0.1", path: "/" },
    ]);
}
const href = `/role-profiles/${profileId}/roadmap`;
for (const language of ["en", "vi"] as const) {
  test(`${language}: roadmap, accessible selection and paginated topic evidence`, async ({
    page,
  }) => {
    await locale(page, language);
    const state = await mockApi(page);
    await page.goto(href);
    await expect(
      page.getByRole("heading", {
        name:
          language === "en"
            ? "Interview practice roadmap"
            : "Lộ trình luyện phỏng vấn",
      }),
    ).toBeVisible();
    await expect(page.getByRole("progressbar")).toHaveAttribute(
      "aria-valuenow",
      "33.33",
    );
    const first = page.getByRole("button", { name: /SQL & data modelling/ });
    await first.focus();
    await page.keyboard.press("Enter");
    await expect(first).toHaveAttribute("aria-pressed", "true");
    await page
      .getByRole("button", {
        name: language === "en" ? "View topic history" : "Xem lịch sử chủ đề",
      })
      .click();
    const dialog = page.getByRole("dialog");
    await expect(dialog.getByText(row.question_text)).toBeVisible();
    await expect(
      dialog.getByText(
        language === "en"
          ? "In the current score window"
          : "Đang được tính vào điểm",
        { exact: true },
      ),
    ).toBeVisible();
    await dialog
      .getByRole("button", {
        name: language === "en" ? "Next" : "Tiếp",
        exact: true,
      })
      .click();
    await expect(
      dialog.getByText(
        language === "en"
          ? "Outside the current window"
          : "Không thuộc cửa sổ điểm hiện tại",
        { exact: true },
      ),
    ).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(dialog).not.toBeVisible();
    expect(state.errors).toEqual([]);
  });
  test(`${language}: mobile roadmap fits and links the chosen topic`, async ({
    page,
  }, testInfo) => {
    await locale(page, language);
    await page.setViewportSize({ width: 375, height: 812 });
    await page.emulateMedia({ reducedMotion: "reduce" });
    await mockApi(page);
    await page.goto(href);
    await page.getByRole("button", { name: /System design/ }).click();
    await expect(
      page.getByRole("link", {
        name: language === "en" ? "Practice this topic" : "Luyện chủ đề này",
        exact: true,
      }),
    ).toHaveAttribute("href", /focusItemId=55555555/);
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    await page.screenshot({
      path: testInfo.outputPath(`roadmap-${language}-375.png`),
      fullPage: true,
    });
  });
  test(`${language}: report identifies only topics with actual session evidence`, async ({
    page,
  }) => {
    await locale(page, language);
    const state = await mockApi(page);
    await page.goto(`/reports/${sessionId}`);
    const result = page.getByRole("region", {
      name:
        language === "en"
          ? "Your roadmap after this session"
          : "Lộ trình sau phiên luyện tập",
    });
    await expect(
      result.getByRole("heading", { name: "SQL & data modelling" }),
    ).toBeVisible();
    await expect(
      result.getByRole("heading", { name: "Go concurrency" }),
    ).toHaveCount(0);
    await expect(
      result.getByRole("link", {
        name:
          language === "en"
            ? "View learning roadmap"
            : "Xem lộ trình luyện tập",
      }),
    ).toHaveAttribute("href", href);
    expect(state.errors).toEqual([]);
  });
}
test("generation states, retry and destructive regeneration confirmation", async ({
  page,
}) => {
  const state = await mockApi(
    page,
    roadmapFixture({
      generation_status: "failed",
      progress_percent: null,
      practice_started: false,
    }),
  );
  await page.goto(href);
  await expect(
    page.getByText("Your roadmap is not ready", { exact: true }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Prepare roadmap", exact: true })
    .click();
  await expect(page.getByRole("progressbar")).toBeVisible();
  await page
    .getByRole("button", { name: "Regenerate roadmap", exact: true })
    .click();
  await expect(page.getByRole("dialog")).toBeVisible();
  expect(state.regenerates).toBe(1);
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Regenerate roadmap", exact: true })
    .click();
  await expect(page.getByRole("dialog")).not.toBeVisible();
  expect(state.regenerates).toBe(2);
});
for (const language of ["en", "vi"] as const) {
  test(`${language}: focus survives setup, new-key retry and a server-returned general session`, async ({
    page,
  }) => {
    await locale(page, language);
    if (language === "vi")
      await page.setViewportSize({ width: 375, height: 812 });
    const state = await mockApi(page, roadmapFixture(), {
      resumed: true,
      startError: "INTERVIEW_START_INTENT_CONFLICT",
    });
    await page.goto(href);
    await page.getByRole("button", { name: /Go concurrency/ }).click();
    await page
      .getByRole("link", {
        name: language === "en" ? "Practice this topic" : "Luyện chủ đề này",
        exact: true,
      })
      .click();
    await expect(
      page.getByText(
        language === "en"
          ? "Focused practice · Go concurrency"
          : "Luyện tập trung · Go concurrency",
        { exact: true },
      ),
    ).toBeVisible();
    await expect(page).toHaveURL(
      new RegExp(
        `/interviews/new/interviewer\\?roleProfileId=${profileId}&focusItemId=${secondItemId}`,
      ),
    );
    await expect(
      page.getByRole("heading", {
        name: language === "en" ? "Choose a Role Profile" : "Chọn hồ sơ vị trí",
        exact: true,
      }),
    ).toHaveCount(0);
    await page
      .getByRole("button", {
        name:
          language === "en"
            ? "Continue to preflight"
            : "Tiếp tục kiểm tra trước",
        exact: true,
      })
      .click();
    await page
      .getByRole("button", {
        name: language === "en" ? "Test microphone" : "Kiểm tra micro",
        exact: true,
      })
      .click();
    const start = page.getByRole("button", {
      name:
        language === "en"
          ? "Start interview · 5 coins"
          : "Bắt đầu phỏng vấn · 5 xu",
      exact: true,
    });
    await expect(start).toBeEnabled();
    await start.click();
    await expect(
      page.getByText(
        language === "en"
          ? /This attempt belongs to a different practice focus/
          : /Lượt bắt đầu này đã gắn/,
      ),
    ).toBeVisible();
    await start.click();
    const dialog = page.getByRole("dialog");
    await expect(
      dialog.getByText(
        language === "en"
          ? "General practice · all topics"
          : "Luyện tổng quát · tất cả chủ đề",
        { exact: true },
      ),
    ).toBeVisible();
    expect(state.starts).toHaveLength(2);
    expect(state.starts[0].body.focus_item_id).toBe(secondItemId);
    expect(state.starts[1].key).not.toBe(state.starts[0].key);
    await expect(page).toHaveURL(/preflight/);
  });
}
test("desktop chart at 1440, 1024 and 768, including dark mode", async ({
  page,
}, testInfo) => {
  await mockApi(page);
  await page.goto(href);
  await expect(page.getByRole("progressbar")).toBeVisible();
  for (const width of [1440, 1024, 768, 390, 320]) {
    await page.setViewportSize({ width, height: 900 });
    await page.evaluate(() =>
      Promise.all(
        document
          .getAnimations()
          .filter(
            (animation) =>
              animation.effect?.getComputedTiming().iterations !== Infinity,
          )
          .map((animation) => animation.finished.catch(() => undefined)),
      ),
    );
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    await page.screenshot({
      path: testInfo.outputPath(`roadmap-en-${width}.png`),
      fullPage: true,
    });
  }
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.evaluate(() => document.documentElement.classList.add("dark"));
  const textColor = await page
    .locator("h1")
    .evaluate((element) => getComputedStyle(element).color);
  await expect(
    page.getByRole("button", { name: /SQL & data modelling/ }),
  ).toHaveCSS("color", textColor);
  await page.evaluate(() =>
    Promise.all(
      document
        .getAnimations()
        .filter(
          (animation) =>
            animation.effect?.getComputedTiming().iterations !== Infinity,
        )
        .map((animation) => animation.finished.catch(() => undefined)),
    ),
  );
  await page.screenshot({
    path: testInfo.outputPath("roadmap-dark-1440.png"),
    fullPage: true,
  });
});

test("overview navigation, progress and focused practice preserve the roadmap profile", async ({
  page,
}) => {
  const state = await mockApi(page);
  await page.goto(href);
  await page
    .getByRole("link", { name: "Learning roadmaps", exact: true })
    .click();
  await expect(page).toHaveURL(/\/roadmaps$/);
  await expect(
    page.getByRole("heading", { level: 1, name: "Learning roadmaps" }),
  ).toBeVisible();
  await expect(page.getByRole("progressbar")).toHaveAttribute(
    "aria-valuenow",
    "33.33",
  );
  await page
    .getByRole("link", { name: "Practice this topic", exact: true })
    .click();
  await expect(page).toHaveURL(
    new RegExp(
      `interviewer\\?roleProfileId=${profileId}&focusItemId=${secondItemId}`,
    ),
  );
  expect(state.errors).toEqual([]);
});

test("overview supports Vietnamese, narrow screens and reduced motion", async ({
  page,
}) => {
  await mockApi(page);
  await page
    .context()
    .addCookies([
      { name: "NEXT_LOCALE", value: "vi", domain: "127.0.0.1", path: "/" },
    ]);
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/roadmaps");
  await expect(
    page.getByRole("heading", { level: 1, name: "Lộ trình luyện tập" }),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Luyện chủ đề này", exact: true }),
  ).toHaveAttribute(
    "href",
    `/interviews/new/interviewer?roleProfileId=${profileId}&focusItemId=${secondItemId}`,
  );
  for (const width of [1440, 1024, 768, 390, 320]) {
    await page.setViewportSize({ width, height: 900 });
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    await expect(page.getByRole("progressbar")).toBeVisible();
  }
});

test("overview hover reveals the rail, retains selection context and closes with Escape", async ({
  page,
}) => {
  const state = await mockApi(page);
  await page.goto("/roadmaps");
  const row = page.locator("article[data-roadmap-preview]").first();
  const toggle = row.getByRole("button", {
    name: "Preview topics for Backend Engineer",
  });
  await expect(toggle).toHaveAttribute("aria-expanded", "false");
  await row
    .getByRole("heading", { name: "Backend Engineer", exact: true })
    .hover();
  await expect(toggle).toHaveAttribute("aria-expanded", "true");
  await expect(
    row.getByRole("button", { name: "View topic: SQL & data modelling" }),
  ).toContainText("Achieved");
  await expect(
    row.getByRole("button", { name: "View topic: Go concurrency" }),
  ).not.toContainText("Achieved");
  await page.getByRole("heading", { level: 1 }).hover();
  await expect(toggle).toHaveAttribute("aria-expanded", "false");
  await toggle.focus();
  await toggle.press("Enter");
  await expect(toggle).toHaveAttribute("aria-expanded", "true");
  await row
    .getByRole("button", { name: "View topic: SQL & data modelling" })
    .click();
  await expect(
    row.getByText("Indexes, joins, transactions and query performance."),
  ).toBeVisible();
  await expect(
    row.getByRole("link", { name: "Practice this topic", exact: true }),
  ).toHaveAttribute(
    "href",
    `/interviews/new/interviewer?roleProfileId=${profileId}&focusItemId=${itemId}`,
  );
  await page.keyboard.press("Escape");
  await expect(toggle).toBeFocused();
  await expect(toggle).toHaveAttribute("aria-expanded", "false");
  expect(state.errors).toEqual([]);
});

test("touch overview opens on the first tap and scrolls topics without page overflow", async ({
  browser,
}) => {
  const context = await browser.newContext({
    viewport: { width: 390, height: 844 },
    hasTouch: true,
    reducedMotion: "reduce",
  });
  const page = await context.newPage();
  try {
    await mockApi(page);
    await context.addCookies([
      { name: "NEXT_LOCALE", value: "vi", domain: "127.0.0.1", path: "/" },
    ]);
    await page.goto("http://127.0.0.1:3100/roadmaps");
    const row = page.locator("article[data-roadmap-preview]").first();
    const toggle = row.getByRole("button", {
      name: "Xem chủ đề của Backend Engineer",
    });
    await toggle.tap();
    await expect(toggle).toHaveAttribute("aria-expanded", "true");
    await expect(
      row.getByRole("button", { name: "Xem chủ đề: SQL & data modelling" }),
    ).toBeVisible();
    const rail = row.getByRole("group", { name: /Các chủ đề · cuộn ngang/ });
    await rail.scrollIntoViewIfNeeded();
    await rail.evaluate((element) => {
      element.scrollLeft = element.scrollWidth;
    });
    await expect(
      row.getByRole("button", { name: "Xem chủ đề: System design" }),
    ).toBeInViewport();
    await row.getByRole("button", { name: "Xem chủ đề: System design" }).tap();
    await expect(
      row.getByRole("link", { name: "Luyện chủ đề này", exact: true }),
    ).toHaveAttribute(
      "href",
      `/interviews/new/interviewer?roleProfileId=${profileId}&focusItemId=55555555-5555-4555-8555-555555555555`,
    );
    for (const width of [390, 320]) {
      await page.setViewportSize({ width, height: 844 });
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth,
        ),
      ).toBe(true);
    }
    await toggle.tap();
    await expect(toggle).toHaveAttribute("aria-expanded", "false");
  } finally {
    await context.close();
  }
});
