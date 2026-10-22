export const socialProviders = ["google", "github", "facebook"] as const;
export type SocialProvider = (typeof socialProviders)[number];

const providerNames: Record<string, string> = {
  credential: "Email and password",
  google: "Google",
  github: "GitHub",
  facebook: "Facebook",
};

export function providerName(provider: string) {
  return providerNames[provider] ?? provider;
}

// Non-secret display allowlist; provider credentials live only in the auth service.
export function enabledSocialProviders(value = ""): SocialProvider[] {
  const configured = new Set(value.split(",").map((item) => item.trim()));
  return socialProviders.filter((provider) => configured.has(provider));
}
