// @vitest-environment node
import { describe, expect, it } from "vitest";
import { betterAuth } from "better-auth";
import { memoryAdapter } from "better-auth/adapters/memory";
import { authOptions } from "../../src/options";
import { rateLimitIpAddress, rateLimitOptions } from "../../src/rate-limit";

const base = "http://localhost:3000/api/auth";
const password = "RoleCue-Test-Password";

// Uses the exported production config unchanged; only the database is in memory.
function createTestAuth() {
  return betterAuth({
    ...authOptions,
    advanced: { ...authOptions.advanced, ipAddress: rateLimitIpAddress },
    rateLimit: rateLimitOptions,
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
    logger: { level: "error" },
  });
}

async function signIn(
  auth: ReturnType<typeof createTestAuth>,
  ip: string,
  body: Record<string, string>,
) {
  return auth.handler(
    new Request(base + "/sign-in/email", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        origin: "http://localhost:3000",
        "x-forwarded-for": ip,
      },
      body: JSON.stringify(body),
    }),
  );
}

describe("auth rate limiting", () => {
  it("returns 429 after the configured sign-in maximum per client IP", async () => {
    const auth = createTestAuth();
    const { max } = rateLimitOptions.customRules["/sign-in/email"] as {
      max: number;
    };
    const attempt = { email: "nobody@example.com", password };

    for (let i = 0; i < max; i++) {
      const response = await signIn(auth, "203.0.113.10", attempt);
      expect(response.status).toBe(401);
    }

    const limited = await signIn(auth, "203.0.113.10", attempt);
    expect(limited.status).toBe(429);
    expect(Number(limited.headers.get("X-Retry-After"))).toBeGreaterThan(0);

    // Another client IP has its own bucket and is still allowed through.
    const otherIp = await signIn(auth, "203.0.113.11", attempt);
    expect(otherIp.status).toBe(401);
  });
});
