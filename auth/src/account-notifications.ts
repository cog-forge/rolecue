import { createAuthMiddleware } from "better-auth/api";
import { z } from "zod";
import type { createMailer } from "./email";

type SendAccountStatusEmail = ReturnType<
  typeof createMailer
>["sendAccountStatusEmail"];

function toDate(value: Date | string | null | undefined) {
  if (value === null || value === undefined) return null;
  const date = value instanceof Date ? value : new Date(value);
  return Number.isNaN(date.getTime()) ? null : date;
}

// Go owns transition/no-op decisions. This hook only sends mail after the plugin succeeds.
export const createAccountNotifications = (
  sendAccountStatusEmail: SendAccountStatusEmail,
) =>
  createAuthMiddleware(async (ctx) => {
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
            // A timed lock carries its end; the plugin returns a Date in-process and a string over HTTP.
            banExpires: z.union([z.date(), z.string(), z.null()]).optional(),
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
          toDate(response.data.user.banExpires),
        );
        status = "accepted";
      } catch {
        status = "failed";
        // Better Auth logging is disabled to keep tokens out of logs; use a fixed message.
        console.warn(
          "Account notification could not be submitted; access change remains effective.",
        );
      }
    }
    return ctx.json({ ...response.data, notification: { status } });
  });
