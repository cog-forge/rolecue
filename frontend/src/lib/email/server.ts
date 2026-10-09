import "server-only";

type EmailUser = { email: string; name: string };
type AuthEmailKind = "verification" | "password-reset";

export function assertAuthEmailConfigured(kind: AuthEmailKind) {
  const apiKey = process.env.RESEND_API_KEY;
  const from = process.env.RESEND_FROM_EMAIL;
  const templateId =
    kind === "verification"
      ? process.env.RESEND_VERIFY_EMAIL_TEMPLATE_ID
      : process.env.RESEND_RESET_PASSWORD_TEMPLATE_ID;
  if (!apiKey || !from || !templateId)
    throw new Error(`Resend ${kind} email configuration is incomplete`);
  return { apiKey, from, templateId };
}

async function sendAuthEmail(
  kind: AuthEmailKind,
  user: EmailUser,
  url: string,
) {
  const { apiKey, from, templateId } = assertAuthEmailConfigured(kind);
  const response = await fetch("https://api.resend.com/emails", {
    method: "POST",
    headers: {
      Authorization: `Bearer ${apiKey}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      from,
      to: [user.email],
      template: {
        id: templateId,
        variables: {
          USER_NAME: user.name.trim() || "there",
          [kind === "verification" ? "VERIFICATION_URL" : "RESET_URL"]: url,
        },
      },
    }),
    signal: AbortSignal.timeout(10_000),
  });
  const result: unknown = await response.json();
  if (
    !response.ok ||
    !result ||
    typeof result !== "object" ||
    !("id" in result) ||
    typeof result.id !== "string"
  ) {
    console.error("Resend auth email rejected", {
      kind,
      status: response.status,
    });
    // Never include recipient, link, credential or the provider response in logs/errors.
    throw new Error(`Unable to send ${kind} email`);
  }
  console.info("Resend auth email accepted", { kind, emailId: result.id });
}

export const sendVerificationEmail = (user: EmailUser, url: string) =>
  sendAuthEmail("verification", user, url);
export const sendPasswordResetEmail = (user: EmailUser, url: string) =>
  sendAuthEmail("password-reset", user, url);
