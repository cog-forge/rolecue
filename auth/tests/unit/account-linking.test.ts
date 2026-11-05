// @vitest-environment node
import { beforeEach, describe, expect, it } from "vitest";
import { betterAuth } from "better-auth";
import { memoryAdapter } from "better-auth/adapters/memory";
import { authOptions } from "../../src/options";

// KAN-291: with trustedProviders empty, a provider never links to an existing account on its own say-so.
// Linking needs the provider to assert a verified email AND the local account to have a verified email.
const base = "http://localhost:3000/api/auth";
const email = "owner@example.com";
const password = "RoleCue-Test-Password";
let profile: { emailVerified: boolean; email: string };

// Google decodes the claims from the ID token; verification is stubbed because the signature is not under test.
function idToken() {
  const part = (value: object) =>
    Buffer.from(JSON.stringify(value)).toString("base64url");
  return [
    part({ alg: "none", typ: "JWT" }),
    part({
      sub: "google-subject-1",
      name: "Provider Name",
      email: profile.email,
      email_verified: profile.emailVerified,
    }),
    "signature",
  ].join(".");
}

function createTestAuth() {
  return betterAuth({
    ...authOptions,
    baseURL: "http://localhost:3000",
    secret: "test-secret-at-least-thirty-two-characters",
    database: memoryAdapter({
      users: [],
      sessions: [],
      accounts: [],
      verifications: [],
      two_factors: [],
    }),
    emailAndPassword: { enabled: true },
    socialProviders: {
      google: {
        clientId: "test-client",
        clientSecret: "test-secret",
        verifyIdToken: async () => true,
      },
    },
    rateLimit: { enabled: false },
    logger: { disabled: true },
  });
}
let auth: ReturnType<typeof createTestAuth>;

function post(path: string, body: unknown) {
  return auth.handler(
    new Request(base + path, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        origin: "http://localhost:3000",
      },
      body: JSON.stringify(body),
    }),
  );
}

const googleSignIn = () =>
  post("/sign-in/social", {
    provider: "google",
    idToken: { token: idToken() },
  });

async function googleAccounts() {
  const ctx = await auth.$context;
  const found = await ctx.internalAdapter.findUserByEmail(email, {
    includeAccounts: true,
  });
  return found?.accounts.filter((a) => a.providerId === "google") ?? [];
}

async function signUpLocal(emailVerified: boolean) {
  await post("/sign-up/email", { email, name: "Owner", password });
  const ctx = await auth.$context;
  const found = await ctx.internalAdapter.findUserByEmail(email);
  await ctx.internalAdapter.updateUser(found!.user.id, { emailVerified });
}

beforeEach(() => {
  profile = { email, emailVerified: false };
  auth = createTestAuth();
});

describe("account linking", () => {
  it("keeps the trusted provider list empty and pins the safe options", () => {
    expect(authOptions.account.accountLinking).toMatchObject({
      enabled: true,
      trustedProviders: [],
      allowDifferentEmails: false,
      allowUnlinkingAll: false,
    });
  });

  it("does not link a provider whose email is unverified to a verified account", async () => {
    await signUpLocal(true);
    const response = await googleSignIn();
    expect(response.status).toBe(401);
    expect(await response.json()).toMatchObject({ code: "OAUTH_LINK_ERROR" });
    expect(await googleAccounts()).toHaveLength(0);
  });

  it("does not link even a verified provider email to an unverified local account", async () => {
    await signUpLocal(false);
    profile.emailVerified = true;
    const response = await googleSignIn();
    expect(response.status).toBe(401);
    expect(await googleAccounts()).toHaveLength(0);
  });

  it("links only when both the provider and the local email are verified", async () => {
    await signUpLocal(true);
    profile.emailVerified = true;
    const response = await googleSignIn();
    expect(response.status).toBe(200);
    expect(await googleAccounts()).toHaveLength(1);
  });
});
