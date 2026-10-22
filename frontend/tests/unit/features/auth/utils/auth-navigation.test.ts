import { describe, expect, it } from "vitest";
import {
  challengeDestination,
  reauthenticationHref,
  securityReturnTo,
  signedInDestination,
} from "@/features/auth/utils/auth-navigation";

describe("auth navigation", () => {
  it.each([
    ["/settings", "/settings"],
    ["/dashboard", undefined],
    ["https://example.com/settings", undefined],
    ["//example.com", undefined],
    [["/settings"], undefined],
    [undefined, undefined],
  ])(
    "allows only the security screen as a return target (%j)",
    (value, expected) => {
      expect(securityReturnTo(value)).toBe(expected);
    },
  );

  it("sends each role home unless returning to security", () => {
    expect(signedInDestination("admin")).toBe(
      "/admin/dashboard?auth=signed-in",
    );
    expect(signedInDestination("recruiter")).toBe(
      "/recruiter/dashboard?auth=signed-in",
    );
    expect(signedInDestination("candidate")).toBe("/dashboard?auth=signed-in");
    expect(signedInDestination("candidate", "/settings")).toBe(
      "/settings?auth=signed-in",
    );
    expect(signedInDestination("candidate", "https://evil.example")).toBe(
      "/dashboard?auth=signed-in",
    );
  });

  it("keeps the security return target through the challenge and reauthentication", () => {
    expect(challengeDestination("/settings")).toBe(
      "/two-factor?returnTo=%2Fsettings",
    );
    expect(challengeDestination("/elsewhere")).toBe("/two-factor");
    expect(reauthenticationHref("/settings")).toBe(
      "/login?returnTo=%2Fsettings",
    );
    expect(reauthenticationHref()).toBe("/login");
  });
});
