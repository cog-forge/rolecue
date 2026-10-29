import { expect, test } from "@playwright/test";
import { mockRoomPreview, roomId } from "./fixtures/room-preview-api";
const room = `/interviews/${roomId}/room`;

declare global {
  interface Window {
    kan53MicrophoneTracks?: MediaStreamTrack[];
    kan53CameraTracks?: MediaStreamTrack[];
    kan53MediaRequests?: MediaStreamConstraints[];
  }
}

test.use({
  launchOptions: {
    args: [
      "--use-fake-device-for-media-stream",
      "--use-fake-ui-for-media-stream",
    ],
  },
});

test("voice room preview reads the selected avatar context without the text interview flow", async ({
  page,
}) => {
  const calls = await mockRoomPreview(page, { completed: true });
  await page.goto(room);
  await expect(page.getByText("Practice room")).toBeVisible();
  await expect(page.getByText("Preview", { exact: true })).toBeVisible();
  await expect(page.getByRole("textbox")).toHaveCount(0);
  await expect(page.getByText("Old REST question")).toHaveCount(0);
  await expect(page.getByText("Old answer")).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Submit answer" })).toHaveCount(
    0,
  );
  await expect(page).toHaveURL(room);
  await page.reload();
  await expect(
    page.getByRole("button", { name: "Enable microphone" }),
  ).toBeVisible();
  await expect(
    page.getByText("Live captions are unavailable in this preview."),
  ).toBeVisible();
  await page.getByRole("button", { name: "Hide transcript" }).click();
  await page.getByRole("link", { name: "Leave", exact: true }).click();
  await expect(page).toHaveURL("/history");
  expect(calls.writes).toEqual([]);
});

test("mic is opt-in, supports mute, input settings and stop, and releases its track on Leave", async ({
  page,
}) => {
  const calls = await mockRoomPreview(page);
  await page.addInitScript(() => {
    const original = navigator.mediaDevices.getUserMedia.bind(
      navigator.mediaDevices,
    );
    window.kan53MediaRequests = [];
    window.kan53MicrophoneTracks = [];
    navigator.mediaDevices.getUserMedia = async (constraints) => {
      window.kan53MediaRequests?.push(constraints ?? {});
      const stream = await original(constraints);
      window.kan53MicrophoneTracks?.push(...stream.getAudioTracks());
      return stream;
    };
  });
  await page.goto(room);
  await expect(page.getByText("Practice room")).toBeVisible();
  expect(await page.evaluate(() => window.kan53MediaRequests)).toEqual([]);
  await page.getByRole("button", { name: "Enable microphone" }).click();
  await expect(
    page.getByRole("button", { name: "Mute microphone" }),
  ).toBeVisible();
  expect(await page.evaluate(() => window.kan53MediaRequests)).toEqual([
    { audio: true, video: false },
  ]);
  await page.getByRole("button", { name: "Mute microphone" }).click();
  expect(
    await page.evaluate(() => window.kan53MicrophoneTracks?.[0].enabled),
  ).toBe(false);
  await page.getByRole("button", { name: "Unmute microphone" }).click();
  expect(
    await page.evaluate(() => window.kan53MicrophoneTracks?.[0].enabled),
  ).toBe(true);
  await page.getByRole("button", { name: "Audio settings" }).click();
  await expect(
    page.getByRole("combobox", { name: "Microphone input" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Stop microphone" }).click();
  expect(
    await page.evaluate(() => window.kan53MicrophoneTracks?.[0].readyState),
  ).toBe("ended");
  await page.keyboard.press("Escape");
  await page.getByRole("button", { name: "Enable microphone" }).click();
  await expect(
    page.getByRole("button", { name: "Mute microphone" }),
  ).toBeVisible();
  await page.getByRole("link", { name: "Leave", exact: true }).click();
  await expect(page).toHaveURL("/history");
  expect(
    await page.evaluate(() =>
      window.kan53MicrophoneTracks?.every(
        (track) => track.readyState === "ended",
      ),
    ),
  ).toBe(true);
  expect(calls.writes).toEqual([]);
});

test("microphone refusal is visible and retryable", async ({ page }) => {
  await mockRoomPreview(page);
  await page.addInitScript(() => {
    navigator.mediaDevices.getUserMedia = async () => {
      throw new DOMException("Denied", "NotAllowedError");
    };
  });
  await page.goto(room);
  await page.getByRole("button", { name: "Enable microphone" }).click();
  await expect(
    page
      .getByRole("alert")
      .filter({ hasText: "Microphone access is not allowed" }),
  ).toContainText("Microphone access is not allowed");
  await expect(
    page.getByRole("button", { name: "Retry microphone" }),
  ).toBeVisible();
});

test("recruitment never reads recruiter inventory and a metadata failure does not start or end a session", async ({
  page,
}) => {
  const calls = await mockRoomPreview(page, {
    recruitment: true,
    avatar: true,
  });
  await page.goto(room);
  await expect(page.getByText("Recruitment room")).toBeVisible();
  await expect(
    page.getByText(/company interviewer is shown in 2D/),
  ).toBeVisible();
  expect(calls.details).toBe(0);
  expect(calls.writes).toEqual([]);
  await page.unroute("http://localhost:8080/**");
  const failed = await mockRoomPreview(page, { error: true });
  await page.reload();
  await expect(
    page.getByText(/Unable to load interview details/),
  ).toBeVisible();
  await expect(page.getByText("Internal server information")).toHaveCount(0);
  await page.getByRole("button", { name: "Retry interviewer" }).click();
  await expect.poll(() => failed.reads).toBe(2);
  expect(failed.writes).toEqual([]);
});

test("expired VRM access is renewed once and remains independent of microphone controls", async ({
  page,
}) => {
  const calls = await mockRoomPreview(page, { avatar: true });
  await page.route("https://s3.test/**", (route) =>
    route.fulfill({
      status: 403,
      headers: { "access-control-allow-origin": "*" },
      body: "Request has expired",
    }),
  );
  await page.goto(room);
  await expect(page.getByText(/preview link expired/)).toBeVisible({
    timeout: 20000,
  });
  expect(calls.details).toBe(2);
  await expect(
    page.getByRole("button", { name: "Enable microphone" }),
  ).toBeEnabled();
  expect(calls.writes).toEqual([]);
});

for (const viewport of [
  { width: 1440, height: 900 },
  { width: 1920, height: 1080 },
  { width: 1280, height: 720 },
  { width: 390, height: 844 },
  { width: 390, height: 667 },
]) {
  test(`voice room controls fit ${viewport.width} × ${viewport.height}`, async ({
    page,
  }) => {
    await page.setViewportSize(viewport);
    await page.emulateMedia({ reducedMotion: "reduce" });
    await mockRoomPreview(page);
    await page.goto(room);
    await expect(page.getByText("Practice room")).toBeVisible();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    const dock = await page
      .getByRole("toolbar", { name: "Interview controls" })
      .boundingBox();
    expect(dock!.x).toBeGreaterThanOrEqual(0);
    expect(dock!.x + dock!.width).toBeLessThanOrEqual(viewport.width);
    await page.getByRole("button", { name: "Audio settings" }).click();
    const settings = page.getByRole("dialog", { name: "Audio settings" });
    await expect(settings).toBeVisible();
    const bounds = await settings.boundingBox();
    expect(bounds!.x).toBeGreaterThanOrEqual(0);
    expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(viewport.width);
    await page.screenshot({
      path: `test-results/voice-room-${viewport.width}-${viewport.height}.png`,
      animations: "disabled",
      fullPage: true,
    });
    await page.keyboard.press("Escape");
    if (viewport.width < 768) {
      await page.getByRole("button", { name: "Expand transcript" }).click();
      await page.getByRole("button", { name: "Reduce transcript" }).click();
      await expect(
        page.getByRole("button", { name: "Expand transcript" }),
      ).toBeVisible();
    }
    await expect(
      page.getByRole("region", { name: "Live transcript" }),
    ).toBeVisible();
  });
}

test("camera is opt-in, shows a real local video preview, and stops on off or Leave", async ({
  page,
}) => {
  const calls = await mockRoomPreview(page);
  await page.addInitScript(() => {
    const original = navigator.mediaDevices.getUserMedia.bind(
      navigator.mediaDevices,
    );
    window.kan53MediaRequests = [];
    window.kan53CameraTracks = [];
    navigator.mediaDevices.getUserMedia = async (constraints) => {
      window.kan53MediaRequests?.push(constraints ?? {});
      const stream = await original(constraints);
      window.kan53CameraTracks?.push(...stream.getVideoTracks());
      return stream;
    };
  });
  await page.goto(room);
  await expect(page.getByText("Practice room")).toBeVisible();
  expect(await page.evaluate(() => window.kan53MediaRequests)).toEqual([]);
  await expect(
    page.getByRole("complementary", { name: "Your camera" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Turn camera on" }).click();
  await expect(
    page.getByRole("button", { name: "Turn camera off" }),
  ).toBeVisible();
  await expect
    .poll(() =>
      page.evaluate(() => document.querySelector("video")?.videoWidth ?? 0),
    )
    .toBeGreaterThan(0);
  expect(await page.evaluate(() => window.kan53MediaRequests)).toEqual([
    { audio: false, video: { facingMode: "user" } },
  ]);
  await page.screenshot({
    path: "test-results/interview-camera-preview.png",
    fullPage: true,
    animations: "disabled",
  });
  await page.getByRole("button", { name: "Turn camera off" }).click();
  expect(
    await page.evaluate(() => window.kan53CameraTracks?.[0].readyState),
  ).toBe("ended");
  await expect(page.getByText("Camera off", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Turn camera on" }).click();
  await expect(
    page.getByRole("button", { name: "Turn camera off" }),
  ).toBeVisible();
  await page.getByRole("link", { name: "Leave", exact: true }).click();
  await expect(page).toHaveURL("/history");
  expect(
    await page.evaluate(() =>
      window.kan53CameraTracks?.every((track) => track.readyState === "ended"),
    ),
  ).toBe(true);
  expect(calls.writes).toEqual([]);
});

test("camera permission errors are visible and retryable", async ({ page }) => {
  await mockRoomPreview(page);
  await page.addInitScript(() => {
    navigator.mediaDevices.getUserMedia = async () => {
      throw new DOMException("Denied", "NotAllowedError");
    };
  });
  await page.goto(room);
  await page.getByRole("button", { name: "Turn camera on" }).click();
  await expect(
    page.getByRole("alert").filter({ hasText: "Camera permission is blocked" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Retry camera" }),
  ).toBeVisible();
});

test("generated backgrounds are selectable from the dock without a tool rail or writes", async ({
  page,
}) => {
  const calls = await mockRoomPreview(page);
  await page.goto(room);
  await expect(page.getByText("Practice room")).toBeVisible();
  await expect(
    page.getByRole("complementary", { name: "Room tools" }),
  ).toHaveCount(0);
  for (const [label, file] of [
    ["Balcony", "balcony"],
    ["Café", "cafe"],
    ["Office", "office"],
  ]) {
    await page.getByRole("button", { name: "Choose background" }).click();
    await page.getByRole("radio", { name: label, exact: true }).click();
    await expect(
      page.locator('[data-slot="studio-backdrop"] img'),
    ).toHaveAttribute("src", new RegExp(`${file}\\.webp`));
    await page.keyboard.press("Escape");
  }
  expect(calls.writes).toEqual([]);
  await expect(page.getByRole("textbox")).toHaveCount(0);
});
