import { describe, expect, it } from "vitest";
import {
  deviceLabel,
  isMobileAgent,
  securityDate,
  securityDay,
} from "@/features/auth/utils/format";
import { createTranslator } from "next-intl";
import { formattersFor, messagesFor } from "@tests/helpers/intl";
import type { AppLocale } from "@/lib/i18n/config";

const fmt = formattersFor("en");
const securityT = (locale: AppLocale = "en") =>
  createTranslator({
    locale,
    messages: messagesFor(locale),
    namespace: "auth.security",
  });
const t = securityT();

describe("security formatting", () => {
  it("formats dates in UTC and hides invalid values", () => {
    expect(securityDate("2026-10-10T10:00:00.000Z", fmt, t)).toBe(
      "Oct 10, 2026, 10:00 AM UTC",
    );
    expect(securityDate("not a date", fmt, t)).toBe("Unavailable");
    expect(securityDate(null, fmt, t)).toBe("Unavailable");
  });

  it.each([
    ["Mozilla/5.0 (Windows NT 10.0) Chrome/123.0 Edg/123.0", "Edge on Windows"],
    ["Mozilla/5.0 (Macintosh) Version/17.0 Safari/605.1", "Safari on macOS"],
    ["Mozilla/5.0 (iPhone) CriOS/123.0 Mobile Safari/604.1", "Chrome on iOS"],
    ["Mozilla/5.0 (X11; Linux) Firefox/124.0", "Firefox on Linux"],
    ["curl/8.0", "Unknown browser on unknown device"],
    [null, "Unknown device"],
  ])("labels %s", (agent, label) => {
    expect(deviceLabel(agent, t)).toBe(label);
  });

  it("prints the day alone in UTC", () => {
    expect(securityDay("2026-09-02T23:30:00.000Z", fmt, t)).toBe("Sep 2, 2026");
    expect(securityDay(undefined, fmt, t)).toBe("Unavailable");
  });

  it.each([
    ["Mozilla/5.0 (iPhone; CPU iPhone OS 18_0) Mobile/15E148", true],
    ["Mozilla/5.0 (Linux; Android 15) Chrome/129.0 Mobile", true],
    ["Mozilla/5.0 (Macintosh) Chrome/129.0", false],
    [null, false],
  ])("treats %s as a phone: %s", (agent, mobile) => {
    expect(isMobileAgent(agent)).toBe(mobile);
  });

  it("speaks Vietnamese for the same values", () => {
    const vi = securityT("vi");
    const viFmt = formattersFor("vi");
    expect(deviceLabel("Mozilla/5.0 (Macintosh) Safari/605.1", vi)).toBe(
      "Safari trên macOS",
    );
    expect(deviceLabel(null, vi)).toBe("Thiết bị không xác định");
    expect(securityDate("2026-10-10T10:00:00.000Z", viFmt, vi)).toMatch(/UTC$/);
    expect(securityDay(undefined, viFmt, vi)).toBe("Không có");
  });
});
