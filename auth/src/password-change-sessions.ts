// A password change must end every other sign-in. Better Auth only does that when the caller
// sends revokeOtherSessions: true, so the server sets it instead of trusting the client.
// Plain function so auth.ts can run it inside its single hooks.before middleware.
export const forceRevokeOtherSessions = (ctx: {
  path: string;
  body?: unknown;
}) => {
  if (ctx.path !== "/change-password") return;
  const body =
    typeof ctx.body === "object" && ctx.body !== null
      ? (ctx.body as Record<string, unknown>)
      : {};
  return { context: { body: { ...body, revokeOtherSessions: true } } };
};
