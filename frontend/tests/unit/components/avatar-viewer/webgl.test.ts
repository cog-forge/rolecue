import { afterEach, describe, expect, it, vi } from "vitest";

const UNMASKED_RENDERER = 0x9246;
const RENDERER = 0x1f01;
const MAX_TEXTURE_SIZE = 0x0d33;

function fakeContext(renderer: string, maxTexture = 16384) {
  return {
    RENDERER,
    MAX_TEXTURE_SIZE,
    getExtension: (name: string) =>
      name === "WEBGL_debug_renderer_info"
        ? { UNMASKED_RENDERER_WEBGL: UNMASKED_RENDERER }
        : name === "WEBGL_lose_context"
          ? { loseContext: () => {} }
          : null,
    getParameter: (parameter: number) =>
      parameter === MAX_TEXTURE_SIZE ? maxTexture : renderer,
  };
}

async function probe(contexts: { webgl2?: unknown; webgl?: unknown }) {
  vi.resetModules();
  vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockImplementation(
    ((kind: string) =>
      (kind === "webgl2" ? contexts.webgl2 : contexts.webgl) ??
      null) as HTMLCanvasElement["getContext"],
  );
  return import("@/components/avatar-viewer/webgl");
}

afterEach(() => vi.restoreAllMocks());

describe("WebGL capability", () => {
  it("allows 3D rooms on a hardware WebGL 2 GPU", async () => {
    const webgl = await probe({
      webgl2: fakeContext("ANGLE (Intel, Intel(R) Iris(R) Xe Graphics)"),
    });
    expect(webgl.isWebGLAvailable()).toBe(true);
    expect(webgl.canRenderEnvironments()).toBe(true);
  });

  it.each([
    ["SwiftShader", "ANGLE (Google, Vulkan 1.3.0 (SwiftShader Device))"],
    ["llvmpipe", "llvmpipe (LLVM 17.0.6, 256 bits)"],
    ["Microsoft Basic Render", "Microsoft Basic Render Driver"],
  ])(
    "keeps the avatar but not the 3D room on a software renderer (%s)",
    async (_, renderer) => {
      const webgl = await probe({ webgl2: fakeContext(renderer) });
      expect(webgl.isWebGLAvailable()).toBe(true);
      expect(webgl.canRenderEnvironments()).toBe(false);
    },
  );

  it("keeps the 2D backdrop without WebGL 2 or 4K textures", async () => {
    const webgl1 = await probe({ webgl: fakeContext("Adreno (TM) 506") });
    expect(webgl1.isWebGLAvailable()).toBe(true);
    expect(webgl1.canRenderEnvironments()).toBe(false);
    const small = await probe({ webgl2: fakeContext("Mali-G52", 2048) });
    expect(small.canRenderEnvironments()).toBe(false);
  });

  it("reports nothing available without a context, and probes only once", async () => {
    const webgl = await probe({});
    expect(webgl.isWebGLAvailable()).toBe(false);
    expect(webgl.canRenderEnvironments()).toBe(false);
    webgl.isWebGLAvailable();
    expect(HTMLCanvasElement.prototype.getContext).toHaveBeenCalledTimes(2);
  });
});
