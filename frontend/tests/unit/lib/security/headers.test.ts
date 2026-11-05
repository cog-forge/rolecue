import { describe, expect, it } from "vitest";
import {
  buildCsp,
  buildNextHeaders,
  cspHeaderName,
} from "@/lib/security/headers";

describe("security headers", () => {
  it("applies the static headers to every path", async () => {
    const [rule] = await buildNextHeaders();
    expect(rule.source).toBe("/:path*");
    const map = Object.fromEntries(rule.headers.map((h) => [h.key, h.value]));
    expect(map["X-Frame-Options"]).toBe("DENY");
    expect(map["X-Content-Type-Options"]).toBe("nosniff");
    expect(map["Referrer-Policy"]).toBe("strict-origin-when-cross-origin");
    expect(map["Permissions-Policy"]).toContain("camera=(self)");
    expect(map["Permissions-Policy"]).toContain("microphone=(self)");
    expect(map["Permissions-Policy"]).toContain("geolocation=()");
  });

  it("builds a nonce-based CSP with the API and Avaturn origins", () => {
    const csp = buildCsp({
      nonce: "abc",
      isDev: false,
      apiBaseUrl: "https://api.example.com/v1",
      avaturnProjectUrl: "https://rolecue.avaturn.dev",
    });
    expect(csp).toContain("script-src 'self' 'nonce-abc' 'strict-dynamic'");
    expect(csp).not.toContain("unsafe-eval");
    expect(csp).toContain("frame-ancestors 'none'");
    expect(csp).toContain("object-src 'none'");
    expect(csp).toMatch(/connect-src [^;]*https:\/\/api\.example\.com/);
    expect(csp).toMatch(/connect-src [^;]*wss:\/\/api\.example\.com/);
    expect(csp).toMatch(/frame-src [^;]*https:\/\/rolecue\.avaturn\.dev/);
    expect(csp).toMatch(/media-src [^;]*blob:/);
    expect(csp).toMatch(/worker-src [^;]*blob:/);
  });

  it("allows eval only in development", () => {
    expect(buildCsp({ nonce: "n", isDev: true })).toContain("'unsafe-eval'");
  });

  it("stays report-only unless CSP_ENFORCE is true", () => {
    expect(cspHeaderName(undefined)).toBe(
      "Content-Security-Policy-Report-Only",
    );
    expect(cspHeaderName("1")).toBe("Content-Security-Policy-Report-Only");
    expect(cspHeaderName("true")).toBe("Content-Security-Policy");
  });
});
