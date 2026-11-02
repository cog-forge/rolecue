import { expect, test, type Page } from "@playwright/test";
import { cors } from "./fixtures/avatar-api";
import { mockLiveServer } from "./fixtures/live-interview";
import { mockRoomPreview, roomId } from "./fixtures/room-preview-api";
import { webmSummary } from "./fixtures/webm-summary";

const room = `/interviews/${roomId}/room`;
const recording = `/interviews/${roomId}/recording`;
const storage = "https://storage.e2e.test";
const questions = [
  { text: "Why do you want to join this team?" },
  { text: "Tell me about a hard bug you fixed." },
];

test.use({
  launchOptions: {
    args: [
      "--use-fake-device-for-media-stream",
      "--use-fake-ui-for-media-stream",
    ],
  },
});

/** The recording API and storage, behaving like the server: completion checks what the PUT stored. */
async function mockRecording(page: Page, { failPuts = 0 } = {}) {
  const api = {
    status: "none" as "none" | "uploading" | "ready" | "failed",
    stored: false,
    failPuts,
    grants: [] as { content_type: string; size: number }[],
    puts: 0,
    completes: 0,
    bodies: [] as Buffer[],
    foreign: [] as string[],
  };
  await page.route("http://localhost:8080/**", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    if (!path.includes("/recording")) return route.fallback();
    if (request.method() === "OPTIONS")
      return route.fulfill({
        status: 204,
        headers: { ...cors, "access-control-allow-methods": "GET,POST" },
      });
    const ok = (data: unknown, status = 200) =>
      route.fulfill({ status, headers: cors, json: { success: true, data } });
    const fail = (status: number, code: string) =>
      route.fulfill({
        status,
        headers: cors,
        json: { success: false, error: { code, message: code } },
      });
    if (!path.startsWith(recording)) {
      api.foreign.push(path);
      return fail(404, "INTERVIEW_NOT_FOUND");
    }
    if (request.method() === "GET" && path === recording)
      return ok({ record_status: api.status });
    if (request.method() === "POST" && path === recording) {
      if (api.status === "ready") return fail(409, "RECORDING_NOT_ALLOWED");
      api.grants.push(request.postDataJSON());
      api.status = "uploading";
      api.stored = false;
      return ok(
        {
          upload_url: `${storage}/put/${api.grants.length}`,
          method: "PUT",
          headers: { "Content-Type": "video/webm" },
          expires_at: new Date(Date.now() + 15 * 60_000).toISOString(),
          record_status: "uploading",
        },
        201,
      );
    }
    if (request.method() === "POST" && path === `${recording}/complete`) {
      api.completes++;
      if (api.status === "ready") return ok({ record_status: "ready" });
      if (api.status !== "uploading") return fail(409, "RECORDING_NOT_ALLOWED");
      if (api.stored) {
        api.status = "ready";
        return ok({ record_status: "ready" });
      }
      api.status = "failed";
      return fail(404, "RECORDING_NOT_FOUND");
    }
    return fail(405, "METHOD_NOT_ALLOWED");
  });
  await page.route(`${storage}/**`, async (route) => {
    const request = route.request();
    const headers = {
      "access-control-allow-origin": "*",
      "access-control-allow-methods": "PUT",
      "access-control-allow-headers": "content-type",
    };
    if (request.method() === "OPTIONS")
      return route.fulfill({ status: 204, headers });
    api.puts++;
    if (api.failPuts > 0) {
      api.failPuts--;
      return route.fulfill({ status: 503, headers, body: "unavailable" });
    }
    const body = request.postDataBuffer();
    if (body) api.bodies.push(body);
    api.stored = true;
    return route.fulfill({ status: 200, headers });
  });
  return api;
}

async function answer(page: Page, text: string) {
  await page.getByRole("button", { name: "Type instead" }).click();
  await page.getByLabel("Your answer").fill(text);
  await page.getByRole("button", { name: "Send answer" }).click();
}

function bufferedSlices(page: Page) {
  return page.evaluate(
    () =>
      new Promise<number>((resolve, reject) => {
        const open = indexedDB.open("rolecue-interview-recording");
        open.onerror = () => reject(open.error);
        open.onsuccess = () => {
          const db = open.result;
          if (!db.objectStoreNames.contains("chunks")) return resolve(0);
          const count = db.transaction("chunks").objectStore("chunks").count();
          count.onsuccess = () => resolve(count.result);
          count.onerror = () => reject(count.error);
        };
      }),
  );
}

test("a recruitment interview records camera and microphone across a reload and uploads one file", async ({
  page,
}) => {
  test.setTimeout(90_000);
  await mockRoomPreview(page, { recruitment: true, live: false });
  const server = await mockLiveServer(page, {
    sessionId: roomId,
    sessionType: "recruitment",
    questions,
  });
  const api = await mockRecording(page);
  await page.goto(room);
  await expect(page.getByText("Recruitment room")).toBeVisible();
  await expect(page.getByText("Recording", { exact: true })).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Camera on for the recording" }),
  ).toBeDisabled();
  await answer(page, "Because of the product.");
  await expect(page.getByText("Question 2 of 2")).toBeVisible();
  // Two 5-second slices are saved in this browser before the page reloads mid-interview.
  await expect
    .poll(() => bufferedSlices(page), { timeout: 15_000 })
    .toBeGreaterThanOrEqual(2);
  const before = Date.now();
  await page.reload();
  await expect(page.getByText("Recording", { exact: true })).toBeVisible();
  await expect(page.getByText("Question 2 of 2")).toBeVisible();
  expect(server.connections).toBe(2);
  await page.waitForTimeout(2_500);
  const afterReload = Date.now() - before;
  await answer(page, "A race in the cache.");

  await expect(page.getByText("Your recording was uploaded.")).toBeVisible({
    timeout: 20_000,
  });
  await expect(page).toHaveURL(new RegExp(`/reports/${roomId}$`));
  expect(server.completed).toBe(true);
  expect(api.grants).toHaveLength(1);
  expect(api.grants[0].content_type).toBe("video/webm");
  expect(api.bodies).toHaveLength(1);
  expect(api.grants[0].size).toBe(api.bodies[0].length);
  expect(api.foreign).toEqual([]);

  const file = webmSummary(api.bodies[0]);
  expect(file.segments).toBe(1);
  // Both page loads are on one timeline: at least the 10 s before the reload plus the time after it.
  expect(file.duration).toBeGreaterThan(10_000 + afterReload - 2_000);
  expect(file.lastBlockTime).toBeGreaterThan(10_000);
  expect(file.clusterTimes).toEqual(
    [...file.clusterTimes].sort((a, b) => a - b),
  );
  expect(await bufferedSlices(page)).toBe(0);
});

test("an upload that keeps failing ends failed, and the Candidate can retry it from the room", async ({
  page,
}) => {
  test.setTimeout(60_000);
  await mockRoomPreview(page, { recruitment: true, live: false });
  await mockLiveServer(page, {
    sessionId: roomId,
    sessionType: "recruitment",
    questions: questions.slice(0, 1),
  });
  const api = await mockRecording(page, { failPuts: 3 });
  await page.goto(room);
  await expect(page.getByText("Recording", { exact: true })).toBeVisible();
  await answer(page, "Because of the product.");
  await expect(
    page.getByText("The upload was interrupted. Trying again…"),
  ).toBeVisible({ timeout: 15_000 });
  await expect(
    page.getByText("We couldn't upload your recording."),
  ).toBeVisible({ timeout: 20_000 });
  expect(api.puts).toBe(3);
  expect(api.completes).toBe(3);
  expect(api.status).toBe("failed");
  await expect(page).toHaveURL(new RegExp(`${room}$`));
  expect(await bufferedSlices(page)).toBeGreaterThan(0);

  await page.getByRole("button", { name: "Try the upload again" }).click();
  await expect(page.getByText("Your recording was uploaded.")).toBeVisible({
    timeout: 15_000,
  });
  expect(api.status).toBe("ready");
});

test("giving up a failed upload leaves the recording failed and deletes it from the browser", async ({
  page,
}) => {
  test.setTimeout(60_000);
  await mockRoomPreview(page, { recruitment: true, live: false });
  await mockLiveServer(page, {
    sessionId: roomId,
    sessionType: "recruitment",
    questions: questions.slice(0, 1),
  });
  const api = await mockRecording(page, { failPuts: 99 });
  await page.goto(room);
  await expect(page.getByText("Recording", { exact: true })).toBeVisible();
  await answer(page, "Because of the product.");
  await expect(
    page.getByText("We couldn't upload your recording."),
  ).toBeVisible({ timeout: 20_000 });
  await page.getByRole("button", { name: "Stop trying" }).click();
  await expect(page.getByText("Stop uploading the recording?")).toBeVisible();
  await page.getByRole("button", { name: "Delete recording" }).click();
  await expect(
    page.getByText(
      "The recording was not uploaded and was deleted from this browser. The recruiter will see that it failed.",
    ),
  ).toBeVisible();
  expect(api.status).toBe("failed");
  expect(await bufferedSlices(page)).toBe(0);
  await expect(page.getByRole("link", { name: "View report" })).toBeVisible();
});

test("a practice room never opens the camera for a recording or calls the recording API", async ({
  page,
}) => {
  await mockRoomPreview(page, { live: false });
  await mockLiveServer(page, {
    sessionId: roomId,
    questions: questions.slice(0, 1),
  });
  const api = await mockRecording(page);
  await page.addInitScript(() => {
    const original = navigator.mediaDevices.getUserMedia.bind(
      navigator.mediaDevices,
    );
    (window as unknown as { videoRequests: number }).videoRequests = 0;
    navigator.mediaDevices.getUserMedia = (constraints) => {
      if (constraints?.video)
        (window as unknown as { videoRequests: number }).videoRequests++;
      return original(constraints);
    };
  });
  await page.goto(room);
  await expect(page.getByText("Practice room")).toBeVisible();
  await answer(page, "A practice answer.");
  await expect(page).toHaveURL(new RegExp(`/reports/${roomId}$`));
  expect(
    await page.evaluate(
      () => (window as unknown as { videoRequests: number }).videoRequests,
    ),
  ).toBe(0);
  expect(api.grants).toEqual([]);
  expect(api.completes).toBe(0);
  await expect(page.getByText("This interview is recorded")).toHaveCount(0);
});
