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
      "--use-fake-ui-for-media-stream=deny",
    ],
  },
});

test("native browser denial, simulated permission recovery and mobile keyboard controls", async ({
  page,
}, testInfo) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto(preflight);
  await page
    .getByRole("button", { name: "Test microphone", exact: true })
    .focus();
  await page.keyboard.press("Enter");
  await expect(
    page
      .getByRole("region", { name: "Check your microphone" })
      .getByRole("alert"),
  ).toContainText("not allowed");
  await page.screenshot({
    path: testInfo.outputPath("preflight-denied-mobile.png"),
    fullPage: true,
  });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  expect(await page.evaluate(() => window.microphoneProbe.calls.length)).toBe(
    1,
  );
  // This headless Chromium requires fake-UI=deny to produce native NotAllowedError.
  // Its denial flag is immutable; simulate the later permission grant at the API boundary.
  // The returned stream and Web Audio graph remain native browser implementations.
  await page.evaluate(() => {
    window.microphoneProbe.synthetic = true;
  });
  await page
    .getByRole("button", { name: "Retry microphone", exact: true })
    .click();
  await expect(
    page.getByText("Microphone ready", { exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Stop microphone check" }).click();
  expect(
    await page.evaluate(() =>
      window.microphoneProbe.streams.every((s) =>
        s.getTracks().every((t) => t.readyState === "ended"),
      ),
    ),
  ).toBe(true);
});
