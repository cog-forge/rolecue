import { describe, expect, it } from "vitest";
import {
  enabledSocialProviders,
  providerName,
} from "@/features/auth/utils/social-providers";

describe("social providers", () => {
  it("keeps only known providers from the configured list, in display order", () => {
    expect(enabledSocialProviders(" facebook,google ,twitter")).toEqual([
      "google",
      "facebook",
    ]);
    expect(enabledSocialProviders("")).toEqual([]);
    expect(enabledSocialProviders()).toEqual([]);
  });

  it("names providers and falls back to the raw ID", () => {
    expect(providerName("credential")).toBe("Email and password");
    expect(providerName("github")).toBe("GitHub");
    expect(providerName("gitlab")).toBe("gitlab");
  });
});
