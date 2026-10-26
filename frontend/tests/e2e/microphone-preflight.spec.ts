import { expect, test } from "@playwright/test";
import {
  candidateSession,
  installMicrophoneProbe,
} from "./fixtures/microphone";

const preflight = "/interviews/new/preflight";
test.beforeEach(async ({ page }) => {
  await candidateSession(page);
  await installMicrophoneProbe(page);
});

test.use({
  launchOptions: {
    executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH,
    args: [
      "--use-fake-device-for-media-stream",
      "--use-fake-ui-for-media-stream",
    ],
  },
});

test("explicit capture, mute, stop and navigation cleanup never start an interview", async ({
  page,
}, testInfo) => {
  const errors: string[] = [];
  const mutations: string[] = [];
  const sockets: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  page.on("request", (request) => {
    if (["POST", "PUT", "PATCH", "DELETE"].includes(request.method()))
      mutations.push(request.url());
  });
  page.on("websocket", (socket) => sockets.push(socket.url()));
  await page.goto(preflight);
  await expect(
    page.getByText("Microphone not tested", { exact: true }),
  ).toBeVisible();
  expect(await page.evaluate(() => window.microphoneProbe.calls.length)).toBe(
    0,
  );
  expect(
    await page.evaluate(() => window.microphoneProbe.contexts.length),
  ).toBe(0);
  for (const error of errors.splice(0)) {
    // Verified on origin/main: unrelated root layout theme bootstrap regex.
    expect(error).toBe("Invalid regular expression: /|$)/: Unmatched ')'");
    testInfo.annotations.push({ type: "baseline issue", description: error });
  }
  await page
    .getByRole("button", { name: "Test microphone", exact: true })
    .click();
  await expect(
    page.getByText("Microphone ready", { exact: true }),
  ).toBeVisible();
  await expect
    .poll(() => page.evaluate(() => window.microphoneProbe.contexts[0]?.state))
    .toBe("running");
  expect(
    await page.evaluate(
      () => window.microphoneProbe.streams[0].getVideoTracks().length,
    ),
  ).toBe(0);
  await page.screenshot({
    path: testInfo.outputPath("preflight-ready-desktop.png"),
    fullPage: true,
  });
  await page
    .getByRole("button", { name: "Mute microphone", exact: true })
    .click();
  await expect(
    page.getByText("Microphone muted", { exact: true }),
  ).toBeVisible();
  expect(
    await page.evaluate(
      () => window.microphoneProbe.streams[0].getAudioTracks()[0].enabled,
    ),
  ).toBe(false);
  await page
    .getByRole("button", { name: "Unmute microphone", exact: true })
    .click();
  await expect(
    page.getByText("Microphone ready", { exact: true }),
  ).toBeVisible();
  expect(await page.evaluate(() => window.microphoneProbe.calls.length)).toBe(
    1,
  );
  await page.getByRole("button", { name: "Stop microphone check" }).click();
  await expect
    .poll(() => page.evaluate(() => window.microphoneProbe.contexts[0].state))
    .toBe("closed");
  expect(
    await page.evaluate(
      () => window.microphoneProbe.streams[0].getTracks()[0].readyState,
    ),
  ).toBe("ended");
  await page
    .getByRole("button", { name: "Test microphone", exact: true })
    .click();
  await expect(
    page.getByText("Microphone ready", { exact: true }),
  ).toBeVisible();
  await page
    .getByRole("navigation", { name: "Interview creation steps" })
    .locator('a[href="/interviews/new/setup"]')
    .click();
  await expect(page).toHaveURL(/\/setup$/);
  await expect
    .poll(() =>
      page.evaluate(() =>
        window.microphoneProbe.contexts.every((c) => c.state === "closed"),
      ),
    )
    .toBe(true);
  expect(
    await page.evaluate(() =>
      window.microphoneProbe.streams.every((s) =>
        s.getTracks().every((t) => t.readyState === "ended"),
      ),
    ),
  ).toBe(true);
  expect(mutations).toEqual([]);
  expect(sockets).toEqual([]);
  expect(errors).toEqual([]);
});

test("device selection, hotplug and retry with native streams and Web Audio", async ({
  page,
}) => {
  await page.goto(preflight);
  await page.evaluate(() => {
    window.microphoneProbe.synthetic = true;
  });
  await page
    .getByRole("button", { name: "Test microphone", exact: true })
    .click();
  await expect(
    page.getByText("Microphone ready", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("Listening — speak to check your input", { exact: true }),
  ).toBeVisible();
  await page.evaluate(() => window.microphoneProbe.setLevel(0.2));
  await expect(page.getByText("Sound detected", { exact: true })).toBeVisible();
  await page.evaluate(() => window.microphoneProbe.setLevel(0));
  await expect(
    page.getByText("Listening — speak to check your input", { exact: true }),
  ).toBeVisible();
  await page.getByRole("combobox", { name: "Microphone input" }).click();
  await page
    .getByRole("option", { name: "Headset microphone", exact: true })
    .click();
  await expect(
    page.getByText("Microphone ready", { exact: true }),
  ).toBeVisible();
  expect(
    await page.evaluate(
      () => window.microphoneProbe.streams[0].getTracks()[0].readyState,
    ),
  ).toBe("ended");
  await page.evaluate(() => {
    window.microphoneProbe.devices = ["mic-1"];
    navigator.mediaDevices.dispatchEvent(new Event("devicechange"));
  });
  await expect(
    page
      .getByRole("region", { name: "Check your microphone" })
      .getByRole("alert"),
  ).toContainText("disconnected");
  await page.evaluate(() => {
    window.microphoneProbe.devices = ["mic-1", "mic-2"];
    navigator.mediaDevices.dispatchEvent(new Event("devicechange"));
  });
  expect(await page.evaluate(() => window.microphoneProbe.calls.length)).toBe(
    2,
  );
  await page
    .getByRole("button", { name: "Retry microphone", exact: true })
    .click();
  await expect(
    page.getByText("Microphone ready", { exact: true }),
  ).toBeVisible();
  await page.evaluate(() => {
    const track = window.microphoneProbe.streams.at(-1)!.getAudioTracks()[0];
    track.stop();
    track.dispatchEvent(new Event("ended"));
  });
  await expect(
    page
      .getByRole("region", { name: "Check your microphone" })
      .getByRole("alert"),
  ).toContainText("disconnected");
});

test("pending permission is cancelled safely on navigation", async ({
  page,
}) => {
  await page.goto(preflight);
  await page.evaluate(() => {
    window.microphoneProbe.deferred = true;
  });
  await page
    .getByRole("button", { name: "Test microphone", exact: true })
    .click();
  await expect(
    page.getByText("Waiting for microphone permission", { exact: true }),
  ).toBeVisible();
  await page
    .getByRole("navigation", { name: "Interview creation steps" })
    .locator('a[href="/interviews/new/setup"]')
    .click();
  await expect(page).toHaveURL(/\/setup$/);
  await page.evaluate(() => window.microphoneProbe.release?.());
  await expect
    .poll(() =>
      page.evaluate(
        () => window.microphoneProbe.streams[0]?.getTracks()[0].readyState,
      ),
    )
    .toBe("ended");
  expect(
    await page.evaluate(() => window.microphoneProbe.contexts.length),
  ).toBe(0);
});
