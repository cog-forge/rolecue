import { createServer, type Server } from "node:http";
import type { AddressInfo } from "node:net";
import { expect, test } from "@playwright/test";
import { buildPermissionsPolicy } from "../../src/lib/security/headers";

test.use({
  launchOptions: {
    executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH,
    args: [
      "--use-fake-device-for-media-stream",
      "--use-fake-ui-for-media-stream",
    ],
  },
});

function listen(
  handler: (url: string) => { headers?: Record<string, string>; body: string },
) {
  return new Promise<{ server: Server; origin: string }>((resolve) => {
    const server = createServer((request, response) => {
      const { headers = {}, body } = handler(request.url ?? "/");
      response.writeHead(200, { "content-type": "text/html", ...headers });
      response.end(body);
    });
    server.listen(0, "127.0.0.1", () =>
      resolve({
        server,
        origin: `http://127.0.0.1:${(server.address() as AddressInfo).port}`,
      }),
    );
  });
}

// The camera step of Avaturn runs in a cross-origin iframe; the parent's Permissions-Policy must name it.
async function probeCreatorIframe(creatorUrl: string | null) {
  const creator = await listen(() => ({
    body: "<!doctype html><title>creator</title>",
  }));
  const parent = await listen(() => ({
    headers: {
      "Permissions-Policy": buildPermissionsPolicy(
        creatorUrl ?? creator.origin,
      ),
    },
    body: `<!doctype html><iframe id="creator" allow="camera *" src="${creator.origin}/"></iframe>`,
  }));
  return { creator, parent };
}

test("the configured Avaturn origin can open the camera inside the creator iframe", async ({
  page,
}) => {
  const { creator, parent } = await probeCreatorIframe(null);
  try {
    await page.goto(parent.origin);
    const frame = page.frames().find((f) => f.url().startsWith(creator.origin));
    expect(frame).toBeTruthy();
    const result = await frame!.evaluate(async () => {
      try {
        const stream = await navigator.mediaDevices.getUserMedia({
          video: true,
        });
        stream.getTracks().forEach((track) => track.stop());
        return "opened";
      } catch (error) {
        return (error as Error).name;
      }
    });
    expect(result).toBe("opened");
  } finally {
    creator.server.close();
    parent.server.close();
  }
});

test("an origin that is not the configured Avaturn origin is denied the camera", async ({
  page,
}) => {
  const { creator, parent } = await probeCreatorIframe(
    "https://rolecue.avaturn.dev",
  );
  try {
    await page.goto(parent.origin);
    const frame = page.frames().find((f) => f.url().startsWith(creator.origin));
    expect(frame).toBeTruthy();
    const result = await frame!.evaluate(async () => {
      try {
        await navigator.mediaDevices.getUserMedia({ video: true });
        return "opened";
      } catch (error) {
        return (error as Error).name;
      }
    });
    expect(result).toBe("NotAllowedError");
  } finally {
    creator.server.close();
    parent.server.close();
  }
});
