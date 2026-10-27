import { avaturnExportSchema } from "../schemas/avatar-schema";

// Browser memory safeguard; matches KAN-122's default input limit. The backend
// still validates its configured limit and the complete T2 asset.
const maxDownloadBytes = 50 * 1024 * 1024;

const tooLarge = "This avatar exceeds the 50 MB browser upload limit.";

// Why an export was refused, without the URL or any model data: enough to tell the cases apart.
function reportRejected(reason: string, payload: unknown) {
  const value: Record<string, unknown> =
    typeof payload === "object" && payload !== null
      ? Object.fromEntries(Object.entries(payload))
      : {};
  const url = typeof value.url === "string" ? value.url : "";
  const comma = url.indexOf(",");
  console.warn("[avatar] Avaturn export rejected", {
    reason,
    fields: Object.keys(value),
    urlType: value.urlType,
    // The scheme and data URL media type only; never the address or the model bytes.
    urlPrefix: url.startsWith("data:")
      ? url.slice(0, comma > 0 ? Math.min(comma, 80) : 40)
      : url.slice(0, Math.max(url.indexOf(":") + 1, 0)),
    avatarSupportsFaceAnimations: value.avatarSupportsFaceAnimations,
  });
}

function validUrl(urlType: "dataURL" | "httpURL", url: string) {
  if (urlType === "dataURL")
    return /^data:[\w.+/-]*(;[\w.+-]+=[\w.+-]+)*;base64,[A-Za-z0-9+/=]+$/.test(
      url,
    );
  try {
    const parsed = new URL(url);
    return parsed.protocol === "https:" && !parsed.username && !parsed.password;
  } catch {
    return false;
  }
}

export async function downloadAvaturnExport(
  payload: unknown,
  signal: AbortSignal,
) {
  const result = avaturnExportSchema.safeParse(payload);
  if (!result.success) {
    reportRejected("unrecognized payload", payload);
    throw new Error(
      "Avaturn sent an export RoleCue couldn't read. Choose Next in Avaturn again.",
    );
  }
  const exported = result.data;
  if (!exported.avatarSupportsFaceAnimations) {
    reportRejected("no facial animation", payload);
    throw new Error(
      "This avatar has no facial animation (it was created as T1). Create a new avatar in Avaturn, then choose Next again.",
    );
  }
  if (!validUrl(exported.urlType, exported.url)) {
    reportRejected("unsupported URL", payload);
    throw new Error(
      "Avaturn sent a download link RoleCue can't use. Choose Next in Avaturn again.",
    );
  }
  if (
    exported.urlType === "dataURL" &&
    exported.url.length > (maxDownloadBytes * 4) / 3 + 128
  ) {
    throw new Error(tooLarge);
  }
  // Avaturn assets must never receive RoleCue cookies or authorization headers.
  const response = await fetch(exported.url, {
    signal,
    credentials: "omit",
    referrerPolicy: "no-referrer",
  });
  if (!response.ok || !response.body)
    throw new Error(
      "We couldn't download the exported avatar. Export it again in Avaturn.",
    );
  const reader = response.body.getReader();
  const chunks: ArrayBuffer[] = [];
  let size = 0;
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      size += value.byteLength;
      if (size > maxDownloadBytes) throw new Error(tooLarge);
      chunks.push(value.slice().buffer);
    }
  } finally {
    await reader.cancel();
    reader.releaseLock();
  }
  const file = new Blob(chunks, { type: "model/gltf-binary" });
  const header = new DataView(await file.slice(0, 12).arrayBuffer());
  if (
    file.size < 12 ||
    header.getUint32(0, true) !== 0x46546c67 ||
    header.getUint32(4, true) !== 2 ||
    header.getUint32(8, true) !== file.size
  ) {
    throw new Error(
      "Avaturn returned an invalid GLB file. Please export again.",
    );
  }
  return file;
}
