import axios from "axios";
import { z } from "zod";

export const signedStorageRequestSchema = z
  .object({
    URL: z
      .url()
      .refine((value) => ["http:", "https:"].includes(new URL(value).protocol)),
    Method: z.enum(["PUT", "GET"]),
    Headers: z.record(z.string(), z.string()),
    ExpiresAt: z.string(),
  })
  .strict();

export async function putStorageFile(
  request: z.infer<typeof signedStorageRequestSchema>,
  file: File,
  progress: (percent: number) => void,
  signal?: AbortSignal,
) {
  if (request.Method !== "PUT") throw new Error("Invalid upload method");

  // Signed storage URLs use a separate transport from the authenticated API.
  await axios.request({
    url: request.URL,
    method: "PUT",
    headers: request.Headers,
    data: file,
    withCredentials: false,
    signal,
    timeout: 120_000,
    onUploadProgress: (event) =>
      progress(Math.min(100, Math.round((event.loaded / file.size) * 100))),
  });
}
