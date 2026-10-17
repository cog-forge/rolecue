import "server-only";
import { randomUUID } from "node:crypto";
import { z } from "zod";

type EmailUser = { email: string; name: string };
type AuthEmailKind = "verification" | "password-reset";
type EmailKind = AuthEmailKind | "locked" | "unlocked";

const templateEnv = {
  verification: "RESEND_VERIFY_EMAIL_TEMPLATE_ID",
  "password-reset": "RESEND_RESET_PASSWORD_TEMPLATE_ID",
  locked: "RESEND_ACCOUNT_LOCKED_TEMPLATE_ID",
  unlocked: "RESEND_ACCOUNT_UNLOCKED_TEMPLATE_ID",
} as const;
const emailConfigSchema = z.object({
  apiKey: z.string().min(1),
  from: z.string().min(1),
  templateId: z.string().min(1),
});
const emailResponseSchema = z.object({ id: z.string().min(1) });
const accountEmailConfigSchema = z.object({
  supportEmail: z.string().min(1),
  appURL: z.string().min(1),
});

function getEmailConfig(kind: EmailKind) {
  return emailConfigSchema.parse({
    apiKey: process.env.RESEND_API_KEY,
    from: process.env.RESEND_FROM_EMAIL,
    templateId: process.env[templateEnv[kind]],
  });
}
export const assertAuthEmailConfigured = (kind: AuthEmailKind) =>
  getEmailConfig(kind);

// Names and public reasons are user input inserted into HTML template variables.
function escapeHtml(value: string) {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

async function sendTemplateEmail(
  kind: EmailKind,
  user: EmailUser,
  variables: Record<string, string>,
  options?: { replyTo: string; idempotencyKey: string },
) {
  const { apiKey, from, templateId } = getEmailConfig(kind);
  const response = await fetch("https://api.resend.com/emails", {
    method: "POST",
    headers: {
      Authorization: `Bearer ${apiKey}`,
      "Content-Type": "application/json",
      ...(options ? { "Idempotency-Key": options.idempotencyKey } : {}),
    },
    body: JSON.stringify({
      from,
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

export const sendVerificationEmail = (user: EmailUser, url: string) =>
  sendTemplateEmail("verification", user, { VERIFICATION_URL: url });
export const sendPasswordResetEmail = (user: EmailUser, url: string) =>
  sendTemplateEmail("password-reset", user, { RESET_URL: url });

export function sendAccountStatusEmail(
  kind: "locked" | "unlocked",
  user: EmailUser,
  publicReason: string,
) {
  const { supportEmail, appURL } = accountEmailConfigSchema.parse({
    supportEmail: process.env.SUPPORT_EMAIL,
    appURL: process.env.BETTER_AUTH_URL,
  });
  return sendTemplateEmail(
    kind,
    user,
    {
      SUPPORT_EMAIL: escapeHtml(supportEmail),
      ...(kind === "locked"
        ? { LOCK_REASON: escapeHtml(publicReason) }
        : { LOGIN_URL: `${appURL}/login` }),
    },
    {
      replyTo: supportEmail,
      idempotencyKey: `account-${kind}-${randomUUID()}`,
    },
  );
}
