import { expect, test, type Page, type Request } from "@playwright/test";
import { cors } from "./fixtures/avatar-api";
import { mockLiveServer } from "./fixtures/live-interview";
import { mockRoomPreview, roomId } from "./fixtures/room-preview-api";

// KAN-229: the UI may be Vietnamese, the interview conversation never is. The room chrome follows
// the catalogs; questions, transcripts and AI feedback stay as the engine produced them. The
// language is the NEXT_LOCALE cookie the switcher writes; the browser itself stays English so a
// browser-added Accept-Language cannot be mistaken for one the app chose.
test.use({
  locale: "en-US",
  launchOptions: {
    args: [
      "--use-fake-device-for-media-stream",
      "--use-fake-ui-for-media-stream",
    ],
  },
});

const reportId = "aaaaaaaa-aaaa-4aaa-8aaa-000000000001";
const candidate = {
  id: "11111111-1111-4111-8111-111111111111",
  full_name: "Cam Candidate",
  email: "cand@example.com",
  image: null,
  role: "candidate",
  email_verified: true,
  is_locked: false,
  onboarding_role_selected: true,
  onboarding_completed: true,
};
const report = {
  interview_id: reportId,
  type: "practice",
  status: "completed",
  overall_score: 82,
  started_at: "2026-10-10T08:00:00Z",
  completed_at: "2026-10-10T08:30:00Z",
  summary: "Clear answers with one gap in system design.",
  breakdown: [
    {
      metric_id: "dddddddd-dddd-4ddd-8ddd-000000000001",
      name: "Communication",
      score: 80,
    },
  ],
  questions: [
    {
      position: 1,
      question_number: 1,
      kind: "core",
      question: "Describe a hard bug you fixed.",
      answer: "A race in the cache invalidation.",
      feedback: "Good root-cause analysis.",
    },
  ],
};

async function useVietnamese(page: Page) {
  await page
    .context()
    .addCookies([
      { name: "NEXT_LOCALE", value: "vi", url: "http://127.0.0.1:3100" },
    ]);
}

// Every request the browser sends to the Go API, with the headers it really sent.
function recordApiRequests(page: Page) {
  const sent: { url: URL; headers: Record<string, string>; body: string }[] =
    [];
  const pending: Promise<void>[] = [];
  page.on("request", (request: Request) => {
    if (new URL(request.url()).origin !== "http://localhost:8080") return;
    pending.push(
      request.allHeaders().then((headers) => {
        sent.push({
          url: new URL(request.url()),
          headers,
          body: request.postData() ?? "",
        });
      }),
    );
  });
  return {
    sent,
    settled: async () => {
      await Promise.all(pending);
      return sent;
    },
  };
}

// The app sends no locale of its own: no parameter, no body field and no language header that
// names Vietnamese.
function expectNoLocale(
  requests: { url: URL; headers: Record<string, string>; body: string }[],
) {
  expect(requests.length).toBeGreaterThan(0);
  for (const { url, headers, body } of requests) {
    for (const name of ["locale", "lang", "language", "hl"])
      expect(url.searchParams.has(name), `${url.pathname} ?${name}`).toBe(
        false,
      );
    expect(headers["accept-language"] ?? "", url.pathname).not.toMatch(/vi/i);
    expect(headers["x-locale"], url.pathname).toBeUndefined();
    expect(headers["content-language"], url.pathname).toBeUndefined();
    expect(body, url.pathname).not.toMatch(/"(locale|lang|language)"/);
  }
}

test("the interview stays English inside a Vietnamese room and no locale reaches the API or socket", async ({
  page,
}) => {
  await useVietnamese(page);
  const requests = recordApiRequests(page);
  await mockRoomPreview(page, { live: false });
  const live = await mockLiveServer(page, {
    sessionId: roomId,
    questions: [{ text: "Walk me through a recent project." }],
  });
  await page.goto(`/interviews/${roomId}/room`);

  // The room chrome follows the UI language...
  await expect(page.locator("html")).toHaveAttribute("lang", "vi");
  await expect(page.getByText("Phòng luyện tập")).toBeVisible();
  await expect(page.getByRole("button", { name: "Bật micro" })).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Rời phòng", exact: true }),
  ).toBeVisible();
  // ...while the conversation itself stays English, exactly as the interviewer sent it.
  await expect(
    page
      .getByLabel("Bản ghi trực tiếp")
      .getByText("Walk me through a recent project."),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: /Change language|Đổi ngôn ngữ/ }),
  ).toHaveCount(0);

  expectNoLocale(await requests.settled());
  const frames = live.frames.flat();
  expect(frames.length).toBeGreaterThan(0);
  for (const frame of frames)
    expect(JSON.stringify(frame)).not.toMatch(/"(locale|lang|language)"/);
});

test("a Vietnamese report keeps the transcript and AI feedback English and sends no locale", async ({
  page,
}) => {
  await useVietnamese(page);
  const requests = recordApiRequests(page);
  await page.route("http://localhost:8080/**", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    if (request.method() === "OPTIONS")
      return route.fulfill({ status: 204, headers: cors });
    if (path === "/auth/me")
      return route.fulfill({
        headers: cors,
        json: { success: true, data: candidate },
      });
    if (path === `/interviews/${reportId}/report`)
      return route.fulfill({
        headers: cors,
        json: { success: true, data: report },
      });
    return route.fulfill({
      status: 404,
      headers: cors,
      json: {
        success: false,
        error: { code: "NOT_FOUND", message: "unmocked" },
      },
    });
  });
  await page.goto(`/reports/${reportId}`);

  // The screen around the report is Vietnamese...
  await expect(page.locator("html")).toHaveAttribute("lang", "vi");
  await expect(
    page.getByRole("heading", { name: "Điểm tổng thể" }),
  ).toBeVisible();
  await expect(page.getByText("Câu hỏi", { exact: true })).toBeVisible();
  // ...and what the AI and the interview produced is shown exactly as stored.
  await expect(
    page.getByText("Clear answers with one gap in system design."),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: /Describe a hard bug you fixed/ }),
  ).toBeVisible();
  await expect(
    page.getByText("A race in the cache invalidation."),
  ).toBeVisible();
  await expect(page.getByText("Good root-cause analysis.")).toBeVisible();
  await expect(
    page.getByRole("progressbar", { name: "Communication" }),
  ).toBeVisible();

  expectNoLocale(await requests.settled());
});
