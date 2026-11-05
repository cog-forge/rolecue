// @vitest-environment node
import { beforeEach, describe, expect, it } from "vitest";
import { betterAuth } from "better-auth";
import { memoryAdapter } from "better-auth/adapters/memory";
import { authOptions } from "../../src/options";
import { createBeforeHooks } from "../../src/auth";

const base = "http://localhost:3000/api/auth";
const password = "RoleCue-Test-Password";
const newPassword = "RoleCue-Next-Password";

function createTestAuth() {
  return betterAuth({
    ...authOptions,
    hooks: { before: createBeforeHooks("test-internal-secret") },
    baseURL: "http://localhost:3000",
    secret: "test-secret-at-least-thirty-two-characters",
    database: memoryAdapter({
      users: [],
      sessions: [],
      accounts: [],
      verifications: [],
      two_factors: [],
      rate_limits: [],
    }),
    emailAndPassword: { enabled: true },
    rateLimit: { enabled: false },
    logger: { level: "error" },
  });
}

let auth: ReturnType<typeof createTestAuth>;

async function post(
  path: string,
  body: unknown,
  headers: Record<string, string> = {},
) {
  return auth.handler(
    new Request(base + path, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        origin: "http://localhost:3000",
        ...headers,
      },
      body: JSON.stringify(body),
    }),
  );
}

function cookieOf(response: Response) {
  return response.headers
    .getSetCookie()
    .map((cookie) => cookie.split(";")[0])
    .join("; ");
}

async function signInFromNewDevice(ip: string) {
  const response = await post(
    "/sign-in/email",
    { email: "candidate@example.com", password },
    { "x-forwarded-for": ip },
  );
  expect(response.status).toBe(200);
  return cookieOf(response);
}

beforeEach(async () => {
  auth = createTestAuth();
  const signUp = await post("/sign-up/email", {
    email: "candidate@example.com",
    name: "candidate",
    password,
  });
  expect(signUp.status).toBe(200);
});

describe("password change session revocation", () => {
  it("revokes other sessions on change-password without the client asking", async () => {
    const sessionA = await signInFromNewDevice("203.0.113.20");
    const sessionB = await signInFromNewDevice("203.0.113.21");

    // The client omits revokeOtherSessions; the server must still revoke session B.
    const change = await post(
      "/change-password",
      { currentPassword: password, newPassword },
      { cookie: sessionA },
    );
    expect(change.status).toBe(200);

    const sessionBState = await auth.handler(
      new Request(base + "/get-session", { headers: { cookie: sessionB } }),
    );
    expect(await sessionBState.json()).toBeNull();

    // Revoking all sessions issues the caller a fresh session cookie, so A continues on that cookie.
    const rotatedA = cookieOf(change);
    expect(rotatedA).not.toBe("");
    const sessionAState = await auth.handler(
      new Request(base + "/get-session", { headers: { cookie: rotatedA } }),
    );
    expect(await sessionAState.json()).toMatchObject({
      user: { email: "candidate@example.com" },
    });
  });
});
