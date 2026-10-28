import { z } from "zod";
const environmentSchema = z.object({ apiBaseUrl: z.url().optional() });
export function getEnvironment() {
  return environmentSchema.parse({
    apiBaseUrl: process.env.NEXT_PUBLIC_API_BASE_URL || undefined,
  });
}

// The public Avaturn Free project origin. Anything else disables the creator; there is no demo fallback.
export function getAvaturnProjectUrl() {
  const result = z.url().safeParse(process.env.NEXT_PUBLIC_AVATURN_PROJECT_URL);
  if (!result.success) return null;
  const url = new URL(result.data);
  return url.protocol === "https:" &&
    /^[a-z0-9-]+\.avaturn\.dev$/.test(url.hostname) &&
    !url.username &&
    !url.password &&
    !url.port &&
    url.pathname === "/" &&
    !url.search &&
    !url.hash
    ? url.origin
    : null;
}
