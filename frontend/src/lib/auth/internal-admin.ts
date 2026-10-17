import { timingSafeEqual } from "node:crypto";
import { APIError, createAuthMiddleware } from "better-auth/api";

// Account policy lives in Go. Only Go may invoke plugin writes and request notification mail.
export const internalAdmin = createAuthMiddleware(async (ctx) => {
  if (ctx.path !== "/admin/ban-user" && ctx.path !== "/admin/unban-user")
    return;
  const expected = process.env.ADMIN_AUTH_INTERNAL_SECRET;
  const provided = ctx.headers?.get("X-RoleCue-Admin-Secret");
  if (
    !expected ||
    !provided ||
    Buffer.byteLength(expected) !== Buffer.byteLength(provided) ||
    !timingSafeEqual(Buffer.from(expected), Buffer.from(provided))
  )
    throw new APIError("FORBIDDEN", {
      message: "Manage accounts through the application API.",
    });
});
