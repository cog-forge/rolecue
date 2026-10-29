import { describe, expect, it } from "vitest";
import {
  matchLocale,
  parseAcceptLanguage,
  resolveLocale,
} from "@/lib/i18n/resolve-locale";

describe("parseAcceptLanguage", () => {
  it("orders ranges by quality and keeps header order on ties", () => {
    expect(
      parseAcceptLanguage("fr;q=0.4, vi-VN, en;q=0.8, de").map((r) => r.tag),
    ).toEqual(["vi-vn", "de", "en", "fr"]);
  });

  it("drops empty entries, q=0 and malformed weights", () => {
    expect(parseAcceptLanguage(", en;q=0, vi;q=abc, de;q=0.5")).toEqual([
      { tag: "de", quality: 0.5 },
    ]);
  });

  it("returns nothing for a missing header", () => {
    expect(parseAcceptLanguage(null)).toEqual([]);
    expect(parseAcceptLanguage(undefined)).toEqual([]);
    expect(parseAcceptLanguage("")).toEqual([]);
  });
});

describe("matchLocale", () => {
  it("matches the primary language subtag case-insensitively", () => {
    expect(matchLocale("vi-VN")).toBe("vi");
    expect(matchLocale("EN-gb")).toBe("en");
  });

  it("does not match unsupported languages or wildcards", () => {
    expect(matchLocale("fr")).toBeUndefined();
    expect(matchLocale("*")).toBeUndefined();
  });
});

describe("resolveLocale", () => {
  it("defaults a new visitor with no signals to English", () => {
    expect(resolveLocale({})).toBe("en");
    expect(resolveLocale({ cookie: null, acceptLanguage: null })).toBe("en");
  });

  it("uses the Accept-Language header when there is no saved choice", () => {
    expect(resolveLocale({ acceptLanguage: "vi-VN,vi;q=0.9,en;q=0.8" })).toBe(
      "vi",
    );
    expect(resolveLocale({ acceptLanguage: "fr-FR,vi;q=0.5" })).toBe("vi");
  });

  it("falls back to English when no requested language is supported", () => {
    expect(resolveLocale({ acceptLanguage: "fr-FR,de;q=0.8,*;q=0.1" })).toBe(
      "en",
    );
  });

  it("prefers the saved cookie over the browser language", () => {
    expect(resolveLocale({ cookie: "en", acceptLanguage: "vi" })).toBe("en");
    expect(resolveLocale({ cookie: "vi", acceptLanguage: "en-US" })).toBe("vi");
  });

  it("ignores an unsupported or tampered cookie", () => {
    expect(resolveLocale({ cookie: "fr", acceptLanguage: "vi" })).toBe("vi");
    expect(resolveLocale({ cookie: "VI", acceptLanguage: undefined })).toBe(
      "en",
    );
  });
});
