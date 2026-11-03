import { readFile } from "node:fs/promises";
import { expect, test, type Page } from "@playwright/test";
import { mockRoomPreview, roomId } from "./fixtures/room-preview-api";
const room = `/interviews/${roomId}/room`;

// The optional real-interviewer test below loads a private local likeness, which must never be
// embedded in Playwright traces or videos.
test.use({
  trace: "off",
  screenshot: "off",
  video: "off",
  launchOptions: {
    args: [
      "--use-fake-device-for-media-stream",
      "--use-fake-ui-for-media-stream",
      "--use-gl=angle",
      "--use-angle=swiftshader",
      "--enable-unsafe-swiftshader",
    ],
  },
});

// Headless Chromium renders with SwiftShader, which the room rightly treats as too weak for a 3D
// room. These tests name a hardware GPU instead so the real loader and renderer run.
async function reportHardwareGpu(page: Page) {
  await page.addInitScript(() => {
    for (const context of [WebGLRenderingContext, WebGL2RenderingContext]) {
      const getParameter = context.prototype.getParameter;
      context.prototype.getParameter = function (parameter: number) {
        return parameter === 0x9246
          ? "ANGLE (Test, Hardware GPU)"
          : getParameter.call(this, parameter);
      };
    }
  });
}

async function disableWebGL(page: Page) {
  await page.addInitScript(() => {
    const getContext = HTMLCanvasElement.prototype.getContext;
    HTMLCanvasElement.prototype.getContext = function (
      this: HTMLCanvasElement,
      kind: string,
      ...rest: unknown[]
    ) {
      if (kind.startsWith("webgl") || kind === "experimental-webgl")
        return null;
      return getContext.call(this, kind, ...rest);
    } as typeof getContext;
  });
}

// The interviewer's model download is held open: the canvas (and its 3D room) mount while it loads.
async function holdInterviewerModel(page: Page) {
  await page.route("https://s3.test/**", () => {});
}

function trackEnvironmentFiles(page: Page) {
  const files: string[] = [];
  page.on("request", (request) => {
    const path = new URL(request.url()).pathname;
    if (path.startsWith("/environments/")) files.push(path);
  });
  return files;
}

const stage = (page: Page) =>
  page.getByRole("region", { name: "Interview room", exact: true });
const backdrop = (page: Page) =>
  page.locator('[data-slot="studio-backdrop"] img');

async function choose(page: Page, name: string) {
  await page.getByRole("button", { name: "Choose background" }).click();
  await page.getByRole("radio", { name, exact: true }).click();
  await page.keyboard.press("Escape");
}

test("the room loads only the chosen 3D room, over its still, and serves it again from cache", async ({
  page,
}) => {
  // Uploading 4K textures through SwiftShader is slow when the suite runs in parallel.
  test.slow();
  await reportHardwareGpu(page);
  const calls = await mockRoomPreview(page, { avatar: true });
  await holdInterviewerModel(page);
  const files = trackEnvironmentFiles(page);
  await page.goto(room);
  await expect(page.getByText("Practice room")).toBeVisible({
    timeout: 15_000,
  });
  await expect(stage(page)).toHaveAttribute("data-background", "cafe-3d");
  await expect(backdrop(page)).toHaveAttribute("src", /cafe%2Fstill\.webp/);
  await expect(stage(page)).toHaveAttribute("data-scene-status", "ready", {
    timeout: 30_000,
  });
  expect(files.sort()).toEqual([
    "/environments/cafe/backplate.webp",
    "/environments/cafe/lighting.hdr",
    "/environments/cafe/props.glb",
  ]);

  await choose(page, "Office (3D room)");
  await expect(backdrop(page)).toHaveAttribute("src", /office%2Fstill\.webp/);
  await expect(stage(page)).toHaveAttribute("data-scene-status", "ready", {
    timeout: 30_000,
  });
  expect(files.filter((file) => file.includes("/office/"))).toHaveLength(3);
  expect(files.some((file) => file.includes("/meeting/"))).toBe(false);

  await choose(page, "Café (3D room)");
  await expect(stage(page)).toHaveAttribute("data-scene-status", "ready", {
    timeout: 30_000,
  });
  expect(files.filter((file) => file.includes("/cafe/"))).toHaveLength(3);

  await choose(page, "Balcony");
  await expect(stage(page)).toHaveAttribute("data-scene-status", "off");
  await expect(backdrop(page)).toHaveAttribute("src", /balcony\.webp/);
  expect(calls.writes).toEqual([]);
});

test("without WebGL the room keeps the scene's still and explains it", async ({
  page,
}) => {
  await disableWebGL(page);
  await mockRoomPreview(page, { avatar: true });
  const files = trackEnvironmentFiles(page);
  await page.goto(room);
  await expect(
    page.getByText("3D preview isn't available on this device or browser."),
  ).toBeVisible({ timeout: 15_000 });
  await expect(stage(page)).toHaveAttribute("data-scene-status", "fallback");
  await expect(backdrop(page)).toHaveAttribute("src", /cafe%2Fstill\.webp/);
  await page.getByRole("button", { name: "Choose background" }).click();
  await expect(page.getByRole("status").last()).toHaveText(
    "This device shows a photo of the room instead of 3D.",
  );
  expect(files).toEqual([]);
});

test("a software renderer draws the interviewer but keeps the room as a still", async ({
  page,
}) => {
  await mockRoomPreview(page, { avatar: true });
  await holdInterviewerModel(page);
  const files = trackEnvironmentFiles(page);
  await page.goto(room);
  // The 3D chunk and a SwiftShader context take a few seconds on CI-class machines.
  await expect(page.locator("canvas")).toHaveCount(1, { timeout: 20_000 });
  await expect(stage(page)).toHaveAttribute("data-scene-status", "fallback");
  await expect(backdrop(page)).toHaveAttribute("src", /cafe%2Fstill\.webp/);
  expect(files).toEqual([]);
});

test("a 3D room that fails to load falls back to its still without breaking the room", async ({
  page,
}) => {
  // Uploading 4K textures through SwiftShader is slow when the suite runs in parallel.
  test.slow();
  await reportHardwareGpu(page);
  await mockRoomPreview(page, { avatar: true });
  await holdInterviewerModel(page);
  await page.route("**/environments/cafe/props.glb", (route) =>
    route.fulfill({ status: 503, body: "unavailable" }),
  );
  await page.goto(room);
  await expect(stage(page)).toHaveAttribute("data-scene-status", "fallback", {
    timeout: 30_000,
  });
  await expect(backdrop(page)).toHaveAttribute("src", /cafe%2Fstill\.webp/);
  await expect(page.locator("canvas")).toHaveCount(1);
  await expect(
    page.getByRole("button", { name: "Enable microphone" }),
  ).toBeEnabled();
  await page.getByRole("button", { name: "Choose background" }).click();
  await expect(page.getByRole("status").last()).toHaveText(
    "The 3D room couldn't load, so a photo is shown instead.",
  );
  await page.getByRole("radio", { name: "Meeting room (3D room)" }).click();
  await expect(stage(page)).toHaveAttribute("data-scene-status", "ready", {
    timeout: 30_000,
  });
});

test.describe("with a real interviewer", () => {
  test("camera views and the 3D room keep working together", async ({
    page,
  }) => {
    // Uploading 4K textures through SwiftShader is slow when the suite runs in parallel.
    test.slow();
    const modelPath = process.env.KAN58_VRM_PATH;
    test.skip(
      !modelPath,
      "Optional private local fixture; never commit a real VRM",
    );
    if (!modelPath) return;
    const bytes = await readFile(modelPath);
    await reportHardwareGpu(page);
    await mockRoomPreview(page, { avatar: true });
    await page.route("https://s3.test/**", (route) =>
      route.fulfill({
        body: bytes,
        headers: {
          "Access-Control-Allow-Origin": "*",
          "Content-Type": "model/gltf-binary",
        },
      }),
    );
    await page.goto(room);
    await expect(page.getByText("Preparing your interviewer…")).toHaveCount(0, {
      timeout: 30_000,
    });
    await expect(stage(page)).toHaveAttribute("data-scene-status", "ready", {
      timeout: 30_000,
    });
    for (const view of ["Closer", "Conversation"]) {
      await page.getByRole("button", { name: "Audio settings" }).click();
      await page.getByRole("radio", { name: view }).click();
      await page.keyboard.press("Escape");
      await expect(stage(page)).toHaveAttribute("data-scene-status", "ready");
      await expect(page.locator("canvas")).toHaveCount(1);
    }
    await choose(page, "Meeting room (3D room)");
    await expect(stage(page)).toHaveAttribute("data-scene-status", "ready", {
      timeout: 30_000,
    });
    await expect(
      page.getByText(/3D rendering stopped|couldn't start on this device/),
    ).toHaveCount(0);
    await expect(page.locator("canvas")).toHaveCount(1);
  });
});
