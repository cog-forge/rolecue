import { afterEach, describe, expect, it, vi } from "vitest";
import { downloadAvaturnExport } from "@/features/avatar/api/avaturn-export";

const exported = {
  avatarId: "avatar",
  sessionId: "session",
  bodyId: "body",
  gender: "male",
  avatarSupportsFaceAnimations: true,
  urlType: "httpURL",
  url: "https://assets.avaturn.me/avatar.glb",
};

function glb(size = 20) {
  const header = new DataView(new ArrayBuffer(size));
  header.setUint32(0, 0x46546c67, true); // "glTF"
  header.setUint32(4, 2, true);
  header.setUint32(8, size, true);
  return header.buffer;
}
const base64 = (buffer: ArrayBuffer) =>
  btoa(String.fromCharCode(...new Uint8Array(buffer)));

afterEach(() => vi.unstubAllGlobals());

async function rejection(payload: unknown) {
  const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
  const fetch = vi.fn();
  vi.stubGlobal("fetch", fetch);
  const error = await downloadAvaturnExport(
    payload,
    new AbortController().signal,
  ).catch((cause: unknown) => cause);
  expect(fetch).not.toHaveBeenCalled();
  return { message: error instanceof Error ? error.message : "", warn };
}

describe("export boundary", () => {
  it("names a T1 avatar instead of a generic failure", async () => {
    const { message, warn } = await rejection({
      ...exported,
      avatarSupportsFaceAnimations: false,
    });
    expect(message).toContain("created as T1");
    expect(warn).toHaveBeenCalledWith(
      "[avatar] Avaturn export rejected",
      expect.objectContaining({
        reason: "no facial animation",
        avatarSupportsFaceAnimations: false,
      }),
    );
  });

  it("separates an unreadable payload from a T1 avatar", async () => {
    const { message } = await rejection({ url: exported.url });
    expect(message).toContain("couldn't read");
    expect(message).not.toContain("T1");
  });

  it.each([
    "http://assets.avaturn.me/avatar.glb",
    "https://secret@assets.avaturn.me/avatar.glb",
    "javascript:alert(1)",
  ])("refuses the download link %s", async (url) => {
    const { message } = await rejection({ ...exported, url });
    expect(message).toContain("download link");
  });

  it("logs the media type but never the address or model data", async () => {
    const url = `data:application/x-unknown;base64,${base64(glb())}not base64!`;
    const { warn } = await rejection({ ...exported, urlType: "dataURL", url });
    const logged = JSON.stringify(warn.mock.calls);
    expect(logged).toContain("data:application/x-unknown;base64");
    expect(logged).not.toContain(base64(glb()));
  });

  it("does not require fields RoleCue doesn't use", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(glb())));
    const file = await downloadAvaturnExport(
      {
        avatarSupportsFaceAnimations: true,
        urlType: "httpURL",
        url: exported.url,
      },
      new AbortController().signal,
    );
    expect(file.size).toBe(20);
  });

  it("accepts a data URL whatever media type Avaturn declares", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response(glb())),
    );
    for (const mime of ["application/octet-stream", "model/gltf-binary", ""]) {
      const file = await downloadAvaturnExport(
        {
          ...exported,
          urlType: "dataURL",
          url: `data:${mime};base64,${base64(glb())}`,
        },
        new AbortController().signal,
      );
      expect(file.type).toBe("model/gltf-binary");
    }
  });

  it("rejects a downloaded non-GLB without sending RoleCue credentials", async () => {
    const fetch = vi.fn().mockResolvedValue(new Response("this is not a GLB"));
    vi.stubGlobal("fetch", fetch);
    await expect(
      downloadAvaturnExport(exported, new AbortController().signal),
    ).rejects.toThrow("invalid GLB");
    expect(fetch).toHaveBeenCalledWith(
      exported.url,
      expect.objectContaining({
        credentials: "omit",
        referrerPolicy: "no-referrer",
      }),
    );
  });
});
