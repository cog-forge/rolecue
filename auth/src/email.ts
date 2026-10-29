import { randomUUID } from "node:crypto";
import { z } from "zod";
import type { AuthEnv } from "./env";

type EmailUser = { email: string; name: string };
type EmailKind = keyof AuthEnv["email"]["templates"];

const emailResponseSchema = z.object({ id: z.string().min(1) });

// Names and public reasons are user input inserted into HTML template variables.
function escapeHtml(value: string) {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

// "2026-11-05 10:00 UTC": unambiguous for a user in any time zone.
function formatLockEnd(date: Date) {
  return `${date.toISOString().slice(0, 16).replace("T", " ")} UTC`;
}

export function createMailer(config: AuthEnv["email"]) {
  async function sendTemplateEmail(
    kind: EmailKind,
    user: EmailUser,
    variables: Record<string, string>,
    options?: { replyTo: string; idempotencyKey: string },
  ) {
    const templateId = config.templates[kind];
    if (!templateId) throw new Error(`Missing ${kind} email template`);
    const response = await fetch("https://api.resend.com/emails", {
      method: "POST",
      headers: {
        Authorization: `Bearer ${config.apiKey}`,
        "Content-Type": "application/json",
        ...(options ? { "Idempotency-Key": options.idempotencyKey } : {}),
      },
      body: JSON.stringify({
        from: config.from,
        to: [user.email],
        ...(options ? { reply_to: options.replyTo } : {}),
        template: {
          id: templateId,
          variables: {
            USER_NAME: escapeHtml(user.name.trim() || "there"),
            ...variables,
          },
        },
      }),
      signal: AbortSignal.timeout(10_000),
    });
    const result: unknown = await response.json();
    if (!response.ok || !emailResponseSchema.safeParse(result).success)
      throw new Error(`Unable to send ${kind} email`);
  }

  return {
    sendVerificationEmail: (user: EmailUser, url: string) =>
      sendTemplateEmail("verification", user, { VERIFICATION_URL: url }),
    sendPasswordResetEmail: (user: EmailUser, url: string) =>
      sendTemplateEmail("password-reset", user, { RESET_URL: url }),
    async sendAccountStatusEmail(
      kind: "locked" | "unlocked",
      user: EmailUser,
      publicReason: string,
      lockEndsAt?: Date | null,
    ) {
      const { supportEmail } = config;
      if (!supportEmail) throw new Error("Missing support email");
      return sendTemplateEmail(
        kind,
        user,
        {
          SUPPORT_EMAIL: escapeHtml(supportEmail),
          ...(kind === "locked"
            ? {
                // The published template prints only the reason, so the end of a timed lock rides with it.
                LOCK_REASON: escapeHtml(
                  lockEndsAt
                    ? `${publicReason} Your access returns on ${formatLockEnd(lockEndsAt)}.`
                    : publicReason,
                ),
              }
            : { LOGIN_URL: `${config.appURL}/login` }),
        },
        {
          replyTo: supportEmail,
          idempotencyKey: `account-${kind}-${randomUUID()}`,
        },
      );
    },
  };
}
