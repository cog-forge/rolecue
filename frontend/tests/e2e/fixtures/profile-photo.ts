import { readFileSync } from "node:fs";
import type { Page } from "@playwright/test";

const api = "http://localhost:8080";
const uploadId = "22222222-2222-4222-8222-222222222222";
const photoKey = "33333333-3333-4333-8333-333333333333";
export const profilePhotoFile = {
  name: "photo.png",
  mimeType: "image/png",
  buffer: readFileSync(
    new URL("../../../public/rolecue-cue.png", import.meta.url),
  ),
};

export async function mockProfilePhoto(page: Page, owner: string) {
  let uploaded = false;
  let missing = false;
  const photoPath = `/profile/photos/${owner}/${photoKey}`;
  const uploadUrl = `${api}/photo-storage/${uploadId}`;
  const headers = {
    "access-control-allow-origin": "http://127.0.0.1:3100",
    "access-control-allow-credentials": "true",
    "access-control-allow-methods": "GET,POST,PUT,OPTIONS",
    "access-control-allow-headers": "Content-Type",
  };

  await page.route(`${api}/profile/photo-uploads`, async (route) => {
    if (route.request().method() === "OPTIONS") {
      await route.fulfill({ status: 204, headers });
      return;
    }
    await route.fulfill({
      status: 201,
      headers,
      json: {
        success: true,
        data: {
          upload_id: uploadId,
          request: {
            URL: uploadUrl,
            Method: "PUT",
            Headers: { "Content-Type": "image/png" },
            ExpiresAt: "2027-01-01T00:00:00Z",
          },
        },
      },
    });
  });
  await page.route(uploadUrl, async (route) => {
    if (route.request().method() === "OPTIONS") {
      await route.fulfill({ status: 204, headers });
      return;
    }
    uploaded =
      route.request().method() === "PUT" &&
      Boolean(
        route.request().postDataBuffer()?.equals(profilePhotoFile.buffer),
      );
    await route.fulfill({ status: uploaded ? 200 : 400, headers });
  });
  await page.route(`${api}${photoPath}`, (route) =>
    missing
      ? route.fulfill({ status: 404, headers })
      : route.fulfill({
          headers,
          contentType: "image/png",
          body: profilePhotoFile.buffer,
        }),
  );

  return {
    photoPath,
    attachedPhoto: (value: unknown) =>
      uploaded && value === uploadId ? photoPath : null,
    makePhotoUnavailable: () => {
      missing = true;
    },
  };
}
