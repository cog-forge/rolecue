import { timingSafeEqual } from "node:crypto";
import { APIError, createAuthMiddleware } from "better-auth/api";

// Account policy lives in Go. Only Go may invoke plugin writes and request notification mail.
// Plain check so auth.ts can run it inside its single hooks.before middleware.
export const enforceInternalAdmin = (
  ctx: { path: string; headers?: Headers | null },
  expected: string,
) => {
  if (ctx.path !== "/admin/ban-user" && ctx.path !== "/admin/unban-user")
    return;
  const provided = ctx.headers?.get("X-RoleCue-Admin-Secret");
  if (
    !provided ||
    Buffer.byteLength(expected) !== Buffer.byteLength(provided) ||
    !timingSafeEqual(Buffer.from(expected), Buffer.from(provided))
  )
    throw new APIError("FORBIDDEN", {
      message: "Manage accounts through the application API.",
    });
};

export const createInternalAdmin = (expected: string) =>
  createAuthMiddleware(async (ctx) => {
    enforceInternalAdmin(ctx, expected);
  });
