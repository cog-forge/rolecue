import { readFile } from "node:fs/promises";
import { expect, test } from "@playwright/test";
import {
  leaveRoom,
  mockRoomPreview,
  roomId,
} from "./fixtures/room-preview-api";
const room = `/interviews/${roomId}/room`;
// A private local likeness must never be embedded in Playwright traces or videos.
test.use({
  trace: "off",
  screenshot: "off",
  video: "off",
  launchOptions: {
    args: [
      "--use-gl=angle",
      "--use-angle=swiftshader",
      "--enable-unsafe-swiftshader",
    ],
  },
});
test("real team VRM survives performance changes and remount through mock model grants", async ({
  page,
}) => {
  const modelPath = process.env.KAN58_VRM_PATH;
  test.skip(
    !modelPath,
    "Optional private local fixture; never commit a real VRM",
  );
  if (!modelPath) return;
  const bytes = await readFile(modelPath);
  let downloads = 0;
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await mockRoomPreview(page, { avatar: true });
  await page.route("https://s3.test/**", (route) => {
    downloads++;
    return route.fulfill({
      body: bytes,
      headers: {
        "Access-Control-Allow-Origin": "*",
        "Content-Type": "model/gltf-binary",
      },
    });
  });
  await page.goto(room);
  await expect(page.getByText("Practice room")).toBeVisible();
  await expect(page.getByText("Preparing your interviewer…")).toHaveCount(0, {
    timeout: 20000,
  });
  await expect(page.locator("canvas")).toHaveCount(1);
  await page.getByRole("button", { name: "Audio settings" }).click();
  await expect(
    page.getByRole("button", { name: "Reset view", exact: true }),
  ).toBeEnabled();
  await page.keyboard.press("Escape");
  const stage = page.getByRole("region", {
    name: "Interview room",
    exact: true,
  });
  // The mocked live socket asks one text-only question, so the room listens right away.
  await expect(stage).toHaveAttribute("data-performance-state", "listening", {
    timeout: 10000,
  });
  expect(downloads).toBe(1);
  await page.reload();
  await expect(page.getByText("Practice room")).toBeVisible();
  await expect(page.getByText("Preparing your interviewer…")).toHaveCount(0, {
    timeout: 20000,
  });
  await page.getByRole("button", { name: "Audio settings" }).click();
  await expect(
    page.getByRole("button", { name: "Reset view", exact: true }),
  ).toBeEnabled();
  await page.keyboard.press("Escape");
  await expect(page.locator("canvas")).toHaveCount(1);
  expect(downloads).toBe(2);
  await leaveRoom(page);
  await expect(page.locator("canvas")).toHaveCount(0);
  expect(errors).toEqual([]);
});
