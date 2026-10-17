import { createAuthMiddleware } from "better-auth/api";
import { z } from "zod";
import { sendAccountStatusEmail } from "@/lib/email/server";

// Go owns transition/no-op decisions. This hook only sends mail after the plugin succeeds.
export const accountNotifications = createAuthMiddleware(async (ctx) => {
  if (ctx.path !== "/admin/ban-user" && ctx.path !== "/admin/unban-user")
    return;
  const locking = ctx.path === "/admin/ban-user";
  const response = z
    .object({
      user: z
        .object({
          id: z.uuid(),
          name: z.string(),
          email: z.string(),
          banned: z.boolean(),
        })
        .passthrough(),
    })
    .safeParse(ctx.context.returned);
  const request = z
    .object({ userId: z.uuid(), banReason: z.string().optional() })
    .safeParse(ctx.body);
  if (
    !response.success ||
    !request.success ||
    response.data.user.id !== request.data.userId ||
    response.data.user.banned !== locking
  )
    return;

  let status: "accepted" | "failed" | "not-needed" = "not-needed";
  if (ctx.headers?.get("X-RoleCue-Account-Notification") === "send") {
    try {
      await sendAccountStatusEmail(
        locking ? "locked" : "unlocked",
        response.data.user,
        request.data.banReason ?? "",
      );
      status = "accepted";
    } catch {
      status = "failed";
      ctx.context.logger.warn(
        "Account notification could not be submitted; access change remains effective.",
      );
    }
  }
  return ctx.json({ ...response.data, notification: { status } });
});
