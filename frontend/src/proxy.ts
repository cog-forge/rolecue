import { NextResponse, type NextRequest } from "next/server";
import { getAvaturnProjectUrl } from "@/config/env";
import { buildCsp, cspHeaderName } from "@/lib/security/headers";

// Sets a per-request nonce CSP. Pages are already dynamic (the root layout reads the locale
// from request headers), so Next applies the nonce to its own scripts automatically.
export function proxy(request: NextRequest) {
  const nonce = btoa(crypto.randomUUID());
  const csp = buildCsp({
    nonce,
    isDev: process.env.NODE_ENV === "development",
    apiBaseUrl: process.env.NEXT_PUBLIC_API_BASE_URL,
    avaturnProjectUrl: getAvaturnProjectUrl(),
  });
  const name = cspHeaderName(process.env.CSP_ENFORCE);
  const requestHeaders = new Headers(request.headers);
  requestHeaders.set("x-nonce", nonce);
  // Next extracts the nonce from either header name on the request.
  requestHeaders.set(name, csp);
  const response = NextResponse.next({ request: { headers: requestHeaders } });
  response.headers.set(name, csp);
  return response;
}

export const config = {
  matcher: [
    {
      source: "/((?!api|_next/static|_next/image|favicon.ico|fonts|.*\\..*).*)",
      missing: [
        { type: "header", key: "next-router-prefetch" },
        { type: "header", key: "purpose", value: "prefetch" },
      ],
    },
  ],
};
