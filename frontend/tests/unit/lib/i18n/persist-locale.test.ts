import { afterEach, describe, expect, it } from "vitest";
import { persistLocale } from "@/lib/i18n/persist-locale";
import { resolveLocale } from "@/lib/i18n/resolve-locale";
import { readBrowserLocale } from "@/lib/i18n/browser-locale";

afterEach(() => {
  document.cookie = "NEXT_LOCALE=; Path=/; Max-Age=0";
});

describe("persistLocale", () => {
  it("stores a supported locale so the next request resolves it", () => {
    persistLocale("vi");
    const cookie = document.cookie.match(/NEXT_LOCALE=([^;]+)/)?.[1];
    expect(cookie).toBe("vi");
    expect(resolveLocale({ cookie, acceptLanguage: "en" })).toBe("vi");
  });

  it("ignores an unsupported locale", () => {
    persistLocale("fr");
    expect(document.cookie).not.toContain("NEXT_LOCALE");
  });

  it("is read back by the browser-side resolver used outside the provider tree", () => {
    expect(readBrowserLocale()).toBe("en");
    persistLocale("vi");
    expect(readBrowserLocale()).toBe("vi");
  });
});
