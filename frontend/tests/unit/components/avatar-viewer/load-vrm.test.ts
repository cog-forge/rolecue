import { afterEach, describe, expect, it, vi } from "vitest";
import { loadVrm, VrmLoadError } from "@/components/avatar-viewer/load-vrm";

const access = (expiresInMs: number) => ({
  url: "https://bucket.s3.example.com/avatars/x.vrm?X-Amz-Signature=abc",
  method: "GET" as const,
  headers: {},
  expiresAt: new Date(Date.now() + expiresInMs).toISOString(),
});

afterEach(() => vi.unstubAllGlobals());

async function kind(promise: Promise<unknown>) {
  try {
    await promise;
  } catch (error) {
    return error instanceof VrmLoadError ? error.kind : "other";
  }
  return "resolved";
}

describe("loadVrm", () => {
  it("asks for a fresh grant instead of downloading with one about to expire", async () => {
    const fetch = vi.fn();
    vi.stubGlobal("fetch", fetch);
    expect(
      await kind(loadVrm(access(5_000), new AbortController().signal)),
    ).toBe("expired");
    expect(fetch).not.toHaveBeenCalled();
  });

  it("downloads without RoleCue credentials or referrer and maps an S3 403 to expired", async () => {
    const fetch = vi.fn().mockResolvedValue(new Response("", { status: 403 }));
    vi.stubGlobal("fetch", fetch);
    const grant = access(120_000);
    expect(await kind(loadVrm(grant, new AbortController().signal))).toBe(
      "expired",
    );
    expect(fetch).toHaveBeenCalledWith(grant.url, {
      method: "GET",
      headers: {},
      credentials: "omit",
      referrerPolicy: "no-referrer",
      signal: expect.any(AbortSignal),
    });
  });

  it("separates network failures from invalid files", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockRejectedValue(new TypeError("Failed to fetch")),
    );
    expect(
      await kind(loadVrm(access(120_000), new AbortController().signal)),
    ).toBe("network");
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(new Response("not a model")),
    );
    expect(
      await kind(loadVrm(access(120_000), new AbortController().signal)),
    ).toBe("invalid");
  });
});
