// Security headers for the Next.js app. Pure functions so next.config.ts, the CSP proxy and the
// unit tests share one definition. Keep the header snippet in .caddy/Caddyfile.deploy in sync.

export const staticSecurityHeaders = [
  { key: "X-Frame-Options", value: "DENY" },
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  {
    key: "Permissions-Policy",
    // The interview room records audio; the Avaturn creator iframe sets its own allow="camera".
    value:
      "camera=(self), microphone=(self), geolocation=(), payment=(), usb=(), bluetooth=(), serial=(), midi=(), accelerometer=(), gyroscope=(), magnetometer=(), display-capture=(), interest-cohort=()",
  },
] as const;

// `async` is required by the shape of Next's headers().
export async function buildNextHeaders() {
  return [
    {
      source: "/:path*",
      headers: staticSecurityHeaders.map((header) => ({ ...header })),
    },
  ];
}

export type CspOptions = {
  nonce: string;
  isDev: boolean;
  apiBaseUrl?: string;
  avaturnProjectUrl?: string | null;
};

function origin(value: string | undefined | null) {
  if (!value) return null;
  try {
    return new URL(value).origin;
  } catch {
    return null;
  }
}

function unique(values: Array<string | null>) {
  return [
    ...new Set(values.filter((value): value is string => Boolean(value))),
  ];
}

export function buildCsp({
  nonce,
  isDev,
  apiBaseUrl,
  avaturnProjectUrl,
}: CspOptions) {
  const api = origin(apiBaseUrl);
  const avaturn = origin(avaturnProjectUrl);
  const directives: Array<[string, string[]]> = [
    ["default-src", ["'self'"]],
    [
      "script-src",
      unique([
        "'self'",
        `'nonce-${nonce}'`,
        "'strict-dynamic'",
        isDev ? "'unsafe-eval'" : null,
      ]),
    ],
    // Inline style attributes (framer-motion, three, Radix) cannot carry a nonce.
    ["style-src", ["'self'", "'unsafe-inline'"]],
    ["img-src", unique(["'self'", "blob:", "data:", api])],
    ["media-src", unique(["'self'", "blob:", "data:", api])],
    ["font-src", ["'self'", "data:"]],
    [
      "connect-src",
      unique([
        "'self'",
        api,
        // The interview room uses a WebSocket on the API origin.
        api ? api.replace(/^http/, "ws") : null,
        avaturn,
        // Avaturn exports are fetched over https (validated in avaturn-export.ts).
        "https://*.avaturn.dev",
        "https://*.avaturn.live",
        "blob:",
        "data:",
        isDev ? "ws:" : null,
      ]),
    ],
    ["worker-src", ["'self'", "blob:"]],
    ["frame-src", unique(["'self'", avaturn, "https://*.avaturn.dev"])],
    ["frame-ancestors", ["'none'"]],
    ["object-src", ["'none'"]],
    ["base-uri", ["'self'"]],
    // PayOS checkout is a top-level redirect, which form-action does not restrict.
    ["form-action", ["'self'"]],
  ];
  return directives
    .map(([name, values]) => `${name} ${values.join(" ")}`)
    .join("; ");
}

// Only CSP_ENFORCE=true enforces the policy; anything else keeps it report-only.
export function cspHeaderName(enforce: string | undefined) {
  return enforce === "true"
    ? "Content-Security-Policy"
    : "Content-Security-Policy-Report-Only";
}
