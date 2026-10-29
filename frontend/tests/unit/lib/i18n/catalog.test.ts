import { afterEach, describe, expect, it, vi } from "vitest";
import { loadMessages } from "@/lib/i18n/catalog";
import { findMissingKeys, mergeMessages } from "@/lib/i18n/fallback";
import en from "@/lib/i18n/messages/en";
import vi_ from "@/lib/i18n/messages/vi";

describe("catalog parity", () => {
  it("has every English key in Vietnamese so nothing silently falls back", () => {
    expect(findMissingKeys(en, vi_)).toEqual([]);
  });

  it("has no Vietnamese key that English lacks", () => {
    expect(findMissingKeys(vi_, en)).toEqual([]);
  });
});

describe("mergeMessages", () => {
  it("fills missing keys, including nested ones, from the base", () => {
    const base = { a: "A", group: { b: "B", c: "C" } };
    expect(mergeMessages(base, { group: { b: "b" } })).toEqual({
      a: "A",
      group: { b: "b", c: "C" },
    });
  });

  it("does not mutate the base catalog", () => {
    const base = { a: "A" };
    mergeMessages(base, { a: "x" });
    expect(base).toEqual({ a: "A" });
  });
});

describe("findMissingKeys", () => {
  it("reports dotted paths for absent keys and absent groups", () => {
    expect(
      findMissingKeys({ a: "A", g: { b: "B" }, h: { c: "C" } }, { g: {} }),
    ).toEqual(["a", "g.b", "h.c"]);
  });
});

describe("loadMessages", () => {
  afterEach(() => vi.unstubAllEnvs());

  it("returns the Vietnamese catalog for vi", async () => {
    const messages = await loadMessages("vi");
    expect(messages.common.skipToContent).toBe(vi_.common.skipToContent);
    expect(messages.common.skipToContent).not.toBe(en.common.skipToContent);
  });

  it("returns English for en", async () => {
    expect((await loadMessages("en")).common.skipToContent).toBe(
      "Skip to content",
    );
  });

  it("falls back to English for a missing key and logs it in development", async () => {
    vi.stubEnv("NODE_ENV", "development");
    vi.resetModules();
    vi.doMock("@/lib/i18n/messages/vi", () => ({
      default: { ...vi_, common: { skipToContent: "Chuyển" } },
    }));
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    const { loadMessages: load } = await import("@/lib/i18n/catalog");
    const messages = await load("vi");
    expect(messages.common.skipToContent).toBe("Chuyển");
    expect(messages.common.language.change).toBe(en.common.language.change);
    expect(warn).toHaveBeenCalledWith(
      expect.stringContaining("common.language.change"),
    );
    vi.doUnmock("@/lib/i18n/messages/vi");
  });
});
