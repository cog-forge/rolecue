type Environment = Record<string, string | undefined>;
const socialProviderNames = ["google", "github", "facebook"] as const;
type SocialProvider = (typeof socialProviderNames)[number];

function exactOrigin(value: string | undefined, name: string): URL {
  try {
    const url = new URL(value ?? "");
    if (
      !value ||
      url.origin !== value ||
      !["http:", "https:"].includes(url.protocol)
    )
      throw new Error();
    return url;
  } catch {
    throw new Error(`${name} must be an exact HTTP(S) origin`);
  }
}

export type AuthEnv = ReturnType<typeof readEnv>;

export function readEnv(env: Environment = process.env) {
  const app = exactOrigin(env.APP_URL, "APP_URL");
  const base = exactOrigin(env.AUTH_BASE_URL, "AUTH_BASE_URL");
  for (const url of [app, base]) {
    if (
      !["localhost", "127.0.0.1", "[::1]"].includes(url.hostname) &&
      !url.hostname.includes(".")
    )
      throw new Error("Public origins must not be internal service names");
  }
  const secret = env.BETTER_AUTH_SECRET;
  const adminSecret = env.ADMIN_AUTH_INTERNAL_SECRET;
  if (!secret || secret.length < 32 || !adminSecret || adminSecret.length < 32)
    throw new Error(
      "Auth signing and internal Admin secrets must contain at least 32 characters",
    );
  const connectionString = env.BETTER_AUTH_DATABASE_URL;
  if (!connectionString)
    throw new Error("BETTER_AUTH_DATABASE_URL is required");
  try {
    if (
      !["postgres:", "postgresql:"].includes(new URL(connectionString).protocol)
    )
      throw new Error();
  } catch {
    throw new Error("BETTER_AUTH_DATABASE_URL must be a PostgreSQL URL");
  }
  const domain = env.BETTER_AUTH_COOKIE_DOMAIN || undefined;
  const secure = base.protocol === "https:";
  if (
    domain &&
    (!secure ||
      app.protocol !== "https:" ||
      ![app, base].every(
        (url) => url.hostname === domain || url.hostname.endsWith(`.${domain}`),
      ))
  )
    throw new Error(
      "Shared cookies require HTTPS public origins under the cookie domain",
    );
  if (env.NODE_ENV === "production" && (!secure || !domain))
    throw new Error("Production requires HTTPS and a shared cookie domain");
  const cookiePrefix = env.BETTER_AUTH_COOKIE_PREFIX || "rolecue";
  if (!/^[a-zA-Z0-9_-]+$/.test(cookiePrefix))
    throw new Error("Invalid cookie prefix");
  const port = Number(env.PORT || 3001);
  if (!Number.isInteger(port) || port < 1 || port > 65535)
    throw new Error("Invalid auth PORT");
  const socialProviders: Partial<
    Record<SocialProvider, { clientId: string; clientSecret: string }>
  > = {};
  for (const provider of socialProviderNames) {
    const clientId = env[`${provider.toUpperCase()}_CLIENT_ID`];
    const clientSecret = env[`${provider.toUpperCase()}_CLIENT_SECRET`];
    if (Boolean(clientId) !== Boolean(clientSecret))
      throw new Error(`${provider} requires both client ID and secret`);
    if (clientId && clientSecret)
      socialProviders[provider] = { clientId, clientSecret };
  }
  const email = {
    apiKey: env.RESEND_API_KEY,
    from: env.RESEND_FROM_EMAIL,
    supportEmail: env.SUPPORT_EMAIL,
    templates: {
      verification: env.RESEND_VERIFY_EMAIL_TEMPLATE_ID,
      "password-reset": env.RESEND_RESET_PASSWORD_TEMPLATE_ID,
      locked: env.RESEND_ACCOUNT_LOCKED_TEMPLATE_ID,
      unlocked: env.RESEND_ACCOUNT_UNLOCKED_TEMPLATE_ID,
    },
  };
  // Verification/reset are part of sign-up and recovery; account notices may fail per send.
  if (
    !email.apiKey ||
    !email.from ||
    !email.templates.verification ||
    !email.templates["password-reset"]
  )
    throw new Error("Resend API key, sender and auth templates are required");
  return {
    appURL: app.origin,
    baseURL: base.origin,
    secret,
    adminSecret,
    connectionString,
    domain,
    secure,
    cookiePrefix,
    port,
    hostname: env.HOST || "0.0.0.0",
    socialProviders,
    email: {
      ...email,
      apiKey: email.apiKey,
      from: email.from,
      appURL: app.origin,
    },
  };
}
