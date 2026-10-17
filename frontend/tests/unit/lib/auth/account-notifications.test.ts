// @vitest-environment node
import { beforeEach, afterEach, describe, expect, it, vi } from "vitest";
import { betterAuth } from "better-auth";
import { memoryAdapter } from "better-auth/adapters/memory";
import { authOptions } from "@/lib/auth/options";
import { z } from "zod";

const mail = vi.hoisted(() => ({ send: vi.fn() }));
vi.mock("@/lib/email/server", () => ({ sendAccountStatusEmail: mail.send }));
const base = "http://localhost:3000/api/auth";
const password = "RoleCue-Test-Password";
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
    rateLimit: { enabled: false },
    logger: { level: "error" },
  });
}
let auth: ReturnType<typeof createTestAuth>;
let adminCookie: string;
let candidateId: string;

async function request(
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

beforeEach(async () => {
  mail.send.mockReset().mockResolvedValue(undefined);
  vi.stubEnv("ADMIN_AUTH_INTERNAL_SECRET", "test-internal-secret");
  auth = createTestAuth();
  for (const role of ["admin", "candidate"]) {
    const response = await request("/sign-up/email", {
      email: `${role}@example.com`,
      name: role,
      password,
    });
    const result = z
      .object({ user: z.object({ id: z.uuid() }) })
      .parse(await response.json());
    const ctx = await auth.$context;
    await ctx.internalAdapter.updateUser(result.user.id, {
      role,
      emailVerified: true,
    });
    if (role === "candidate") candidateId = result.user.id;
  }
  const login = await request("/sign-in/email", {
    email: "admin@example.com",
    password,
  });
  adminCookie = login.headers
    .getSetCookie()
    .map((cookie) => cookie.split(";")[0])
    .join("; ");
});
afterEach(() => vi.unstubAllEnvs());

const internalHeaders = () => ({
  cookie: adminCookie,
  "X-RoleCue-Admin-Secret": "test-internal-secret",
  "X-RoleCue-Account-Notification": "send",
});

describe("internal account notification integration", () => {
  it("denies browser bypass and sends only after a successful plugin write", async () => {
    const body = { userId: candidateId, banReason: "Public reason" };
    const denied = await request("/admin/ban-user", body, {
      cookie: adminCookie,
    });
    expect(denied.status).toBe(403);
    expect(mail.send).not.toHaveBeenCalled();
    const response = await request("/admin/ban-user", body, internalHeaders());
    expect(response.status).toBe(200);
    expect(await response.json()).toMatchObject({
      user: { id: candidateId, banned: true },
      notification: { status: "accepted" },
    });
    const ctx = await auth.$context;
    expect(await ctx.internalAdapter.findUserById(candidateId)).toMatchObject({
      banned: true,
    });
    expect(mail.send).toHaveBeenCalledWith(
      "locked",
      expect.objectContaining({ id: candidateId }),
      "Public reason",
    );
    const skipped = await request(
      "/admin/unban-user",
      { userId: candidateId },
      { ...internalHeaders(), "X-RoleCue-Account-Notification": "skip" },
    );
    expect(await skipped.json()).toMatchObject({
      notification: { status: "not-needed" },
    });
    expect(mail.send).toHaveBeenCalledTimes(1);
  });
  it("keeps the plugin transition successful when sending mail fails", async () => {
    mail.send.mockRejectedValue(new Error("Provider unavailable"));
    const response = await request(
      "/admin/ban-user",
      { userId: candidateId, banReason: "Public reason" },
      internalHeaders(),
    );
    expect(response.status).toBe(200);
    expect(await response.json()).toMatchObject({
      user: { banned: true },
      notification: { status: "failed" },
    });
    const ctx = await auth.$context;
    expect(await ctx.internalAdapter.findUserById(candidateId)).toMatchObject({
      banned: true,
    });
  });
});
