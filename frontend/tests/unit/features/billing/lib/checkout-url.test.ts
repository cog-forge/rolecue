import { afterEach, describe, expect, it, vi } from "vitest";
import {
  isTrustedCheckoutUrl,
  redirectToCheckout,
  trustedCheckoutHosts,
} from "@/features/billing/lib/checkout-url";
import {
  depositCheckoutRequestSchema,
  depositCheckoutResponseSchema,
} from "@/features/billing/schemas/wallet-schema";

const response = {
  order_code: 123456,
  amount: 50000,
  coin_amount: 100,
  transaction_id: "66666666-6666-4666-8666-666666666666",
};

describe("isTrustedCheckoutUrl", () => {
  it("accepts an https URL on the PayOS checkout host", () => {
    expect(isTrustedCheckoutUrl("https://pay.payos.vn/web/abc123")).toBe(true);
    expect(isTrustedCheckoutUrl("https://PAY.payos.vn/web/abc123?x=1#y")).toBe(
      true,
    );
  });

  it.each([
    ["another host", "https://evil.example/web/abc"],
    ["a look-alike suffix", "https://pay.payos.vn.evil.example/web/abc"],
    ["a look-alike prefix", "https://evilpay.payos.vn/web/abc"],
    ["a sibling subdomain", "https://x.pay.payos.vn/web/abc"],
    ["credentials that hide the host", "https://pay.payos.vn@evil.example/"],
    ["credentials on the host", "https://user:pw@pay.payos.vn/web/abc"],
    ["plain http", "http://pay.payos.vn/web/abc"],
    ["a javascript URL", "javascript:alert(1)"],
    ["a data URL", "data:text/html,hi"],
    ["a relative URL", "/web/abc"],
    ["a protocol-relative URL", "//evil.example/web/abc"],
    ["a non-default port", "https://pay.payos.vn:8443/web/abc"],
    ["an empty string", ""],
    ["text that is not a URL", "not a url"],
  ])("rejects %s", (_name, value) => {
    expect(isTrustedCheckoutUrl(value)).toBe(false);
  });

  it("takes extra hosts from NEXT_PUBLIC_PAYOS_CHECKOUT_HOSTS", () => {
    expect(trustedCheckoutHosts(undefined)).toEqual(["pay.payos.vn"]);
    expect(trustedCheckoutHosts("")).toEqual(["pay.payos.vn"]);
    expect(trustedCheckoutHosts("pay.example.vn, Sandbox.Example.vn")).toEqual([
      "pay.example.vn",
      "sandbox.example.vn",
    ]);
    expect(trustedCheckoutHosts("not a host!, //x")).toEqual(["pay.payos.vn"]);
    expect(
      isTrustedCheckoutUrl(
        "https://pay.example.vn/web/abc",
        trustedCheckoutHosts("pay.example.vn"),
      ),
    ).toBe(true);
  });
});

describe("redirectToCheckout", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("navigates only to a trusted checkout", () => {
    const location = { href: "" };
    vi.stubGlobal("location", location);
    expect(redirectToCheckout("https://evil.example/web/abc")).toBe(false);
    expect(location.href).toBe("");
    expect(redirectToCheckout("https://pay.payos.vn/web/abc")).toBe(true);
    expect(location.href).toBe("https://pay.payos.vn/web/abc");
  });
});

describe("checkout schemas", () => {
  it("refuses a checkout URL that is not https on the PayOS host", () => {
    for (const checkout_url of [
      "https://evil.example/web/abc",
      "http://pay.payos.vn/web/abc",
      "javascript:alert(1)",
      "https://pay.payos.vn@evil.example/",
      "",
    ]) {
      expect(
        depositCheckoutResponseSchema.safeParse({ ...response, checkout_url })
          .success,
      ).toBe(false);
    }
    expect(
      depositCheckoutResponseSchema.safeParse({
        ...response,
        checkout_url: "https://pay.payos.vn/web/abc",
      }).success,
    ).toBe(true);
  });

  it("refuses return and cancel URLs that are not http(s) URLs", () => {
    const packageId = "33333333-3333-4333-8333-333333333333";
    for (const url of [
      "javascript:alert(1)",
      "data:text/html,hi",
      "/billing",
      "not a url",
      `https://app.example/${"a".repeat(2100)}`,
    ]) {
      expect(
        depositCheckoutRequestSchema.safeParse({
          package_id: packageId,
          return_url: url,
        }).success,
      ).toBe(false);
      expect(
        depositCheckoutRequestSchema.safeParse({
          package_id: packageId,
          cancel_url: url,
        }).success,
      ).toBe(false);
    }
    expect(
      depositCheckoutRequestSchema.safeParse({
        package_id: packageId,
        return_url: "http://localhost:3000/billing?status=success",
        cancel_url: "https://app.example/billing?cancel=true",
      }).success,
    ).toBe(true);
  });
});
