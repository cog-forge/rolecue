// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from "vitest";
import { betterAuth } from "better-auth";
import { memoryAdapter } from "better-auth/adapters/memory";
import { authOptions } from "../../src/options";
import {
  createAuditLog,
  createEmailHasher,
  type AuditEvent,
} from "../../src/audit-log";

const base = "http://localhost:3000/api/auth";
const secret = "test-secret-at-least-thirty-two-characters";
const password = "RoleCue-Test-Password";
const email = "person@example.com";
const wrongPassword = "Definitely-Wrong-Password";
const resetTokens: string[] = [];

let events: AuditEvent[];

function createTestAuth() {
  const audit = createAuditLog({ secret, sink: (e) => events.push(e) });
  return betterAuth({
    ...authOptions,
    plugins: [...authOptions.plugins, audit.plugin],
    databaseHooks: audit.databaseHooks,
    baseURL: "http://localhost:3000",
    secret,
    database: memoryAdapter({
      users: [],
      sessions: [],
      accounts: [],
      verifications: [],
      two_factors: [],
    }),
    emailAndPassword: {
      enabled: true,
      sendResetPassword: async ({ token }) => {
        resetTokens.push(token);
      },
    },
    rateLimit: { enabled: false },
    logger: { disabled: true },
  });
}
let auth: ReturnType<typeof createTestAuth>;

function post(
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
        "x-forwarded-for": "203.0.113.7",
        ...headers,
      },
      body: JSON.stringify(body),
    }),
  );
}

beforeEach(async () => {
  events = [];
  resetTokens.length = 0;
  auth = createTestAuth();
  const response = await post("/sign-up/email", {
    email,
    name: "Person",
    password,
  });
  expect(response.status).toBe(200);
  events.length = 0;
});

describe("auth audit log", () => {
  it("records a failed sign-in with a reason, a hashed email and the IP", async () => {
    const response = await post("/sign-in/email", {
      email,
      password: wrongPassword,
    });
    expect(response.status).toBe(401);
    expect(events).toEqual([
      expect.objectContaining({
        event: "auth.sign_in",
        outcome: "failure",
        reason: "INVALID_EMAIL_OR_PASSWORD",
        emailHash: createEmailHasher(secret)(email),
        ip: "203.0.113.7",
      }),
    ]);
  });

  it("records a successful sign-in with the user id", async () => {
    const response = await post("/sign-in/email", { email, password });
    expect(response.status).toBe(200);
    const { user } = (await response.json()) as { user: { id: string } };
    expect(events).toEqual([
      expect.objectContaining({
        event: "auth.sign_in",
        outcome: "success",
        userId: user.id,
      }),
    ]);
  });

  it("records a lockout when a locked account tries to sign in", async () => {
    const ctx = await auth.$context;
    const found = await ctx.internalAdapter.findUserByEmail(email);
    await ctx.internalAdapter.updateUser(found!.user.id, { banned: true });
    events.length = 0;
    const response = await post("/sign-in/email", { email, password });
    expect(response.status).toBe(403);
    expect(events.map((e) => [e.event, e.outcome, e.reason])).toEqual([
      ["auth.sign_in", "failure", "BANNED_USER"],
      ["auth.lockout", "blocked", "BANNED_USER"],
    ]);
  });

  it("records password reset requests without revealing whether the account exists", async () => {
    await post("/request-password-reset", { email });
    await post("/request-password-reset", { email: "nobody@example.com" });
    expect(events.map((e) => [e.event, e.outcome])).toEqual([
      ["auth.password_reset_request", "requested"],
      ["auth.password_reset_request", "requested"],
    ]);
    expect(events[0].emailHash).not.toBe(events[1].emailHash);
  });

  it("records a password change", async () => {
    const login = await post("/sign-in/email", { email, password });
    const cookie = login.headers
      .getSetCookie()
      .map((c) => c.split(";")[0])
      .join("; ");
    events.length = 0;
    const response = await post(
      "/change-password",
      { currentPassword: password, newPassword: "Another-Password-1" },
      { cookie },
    );
    expect(response.status).toBe(200);
    expect(events).toEqual([
      expect.objectContaining({
        event: "auth.password_change",
        outcome: "success",
        userId: expect.any(String),
      }),
    ]);
  });

  it("never contains passwords, tokens, cookies or the raw email", async () => {
    await post("/sign-in/email", { email, password: wrongPassword });
    const login = await post("/sign-in/email", { email, password });
    const cookie = login.headers
      .getSetCookie()
      .map((c) => c.split(";")[0])
      .join("; ");
    await post("/request-password-reset", { email });
    await post("/reset-password", {
      token: "not-a-real-token",
      newPassword: "Reset-Secret-Pass-1",
    });
    await post(
      "/change-password",
      { currentPassword: password, newPassword: "Changed-Secret-Pass-1" },
      { cookie },
    );
    expect(events.length).toBeGreaterThanOrEqual(5);
    const logged = JSON.stringify(events);
    const session = (await login.json()) as { token: string };
    for (const secretValue of [
      password,
      wrongPassword,
      "Reset-Secret-Pass-1",
      "Changed-Secret-Pass-1",
      "not-a-real-token",
      session.token,
      cookie,
      resetTokens[0],
      email,
      "example.com",
      "Person",
    ])
      expect(logged).not.toContain(secretValue);
  });

  it("does not break authentication when the sink throws", async () => {
    const sink = vi.fn(() => {
      throw new Error("disk full");
    });
    const audit = createAuditLog({ secret, sink });
    const failing = betterAuth({
      ...authOptions,
      plugins: [...authOptions.plugins, audit.plugin],
      databaseHooks: audit.databaseHooks,
      baseURL: "http://localhost:3000",
      secret,
      database: memoryAdapter({
        users: [],
        sessions: [],
        accounts: [],
        verifications: [],
        two_factors: [],
      }),
      emailAndPassword: { enabled: true },
      rateLimit: { enabled: false },
      logger: { disabled: true },
    });
    const response = await failing.handler(
      new Request(base + "/sign-up/email", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          origin: "http://localhost:3000",
        },
        body: JSON.stringify({ email, name: "Person", password }),
      }),
    );
    expect(response.status).toBe(200);
    expect(sink).toHaveBeenCalled();
  });
});
