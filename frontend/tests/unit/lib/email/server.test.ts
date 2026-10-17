// @vitest-environment node
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  assertAuthEmailConfigured,
  sendAccountStatusEmail,
  sendPasswordResetEmail,
  sendVerificationEmail,
} from "@/lib/email/server";
vi.mock("server-only", () => ({}));
const fetchEmail = vi.fn();
beforeEach(() => {
  vi.stubEnv("RESEND_API_KEY", "test-key");
  vi.stubEnv("RESEND_FROM_EMAIL", "RoleCue <no-reply@auth.dorriss.com>");
  vi.stubEnv("RESEND_VERIFY_EMAIL_TEMPLATE_ID", "verification-template");
  vi.stubEnv("RESEND_RESET_PASSWORD_TEMPLATE_ID", "reset-template");
  vi.stubEnv(
    "RESEND_ACCOUNT_LOCKED_TEMPLATE_ID",
    "3667a438-9b17-4696-8b07-d36e382fffe2",
  );
  vi.stubEnv(
    "RESEND_ACCOUNT_UNLOCKED_TEMPLATE_ID",
    "b7757145-fc97-4153-b926-c904af1edfdd",
  );
  vi.stubEnv("SUPPORT_EMAIL", "support@example.com");
  vi.stubEnv("BETTER_AUTH_URL", "https://app.example.com");
  fetchEmail
    .mockReset()
    .mockImplementation(async () => Response.json({ id: "email-1" }));
  vi.stubGlobal("fetch", fetchEmail);
});
afterEach(() => {
  vi.unstubAllEnvs();
  vi.unstubAllGlobals();
});
describe("auth template submission", () => {
  it("reuses published account templates and the same transport with escaped public text", async () => {
    const user = { email: "user@example.com", name: "<Alex> & Team" };
    await sendAccountStatusEmail("locked", user, "Spam <messages> & abuse");
    await sendAccountStatusEmail("unlocked", user, "");
    const locked = JSON.parse(fetchEmail.mock.calls[0][1].body);
    const unlocked = JSON.parse(fetchEmail.mock.calls[1][1].body);
    expect(locked.template.id).toBe("3667a438-9b17-4696-8b07-d36e382fffe2");
    expect(locked.template.variables.LOCK_REASON).toBe(
      "Spam &lt;messages&gt; &amp; abuse",
    );
    expect(locked.template.variables.USER_NAME).toBe("&lt;Alex&gt; &amp; Team");
    expect(locked.reply_to).toBe("support@example.com");
    expect(fetchEmail.mock.calls[0][1].headers["Idempotency-Key"]).toMatch(
      /^account-locked-/,
    );
    expect(unlocked.template.id).toBe("b7757145-fc97-4153-b926-c904af1edfdd");
    expect(unlocked.template.variables.LOGIN_URL).toBe(
      "https://app.example.com/login",
    );
  });
  it("uses the same sender and transport for verification and reset", async () => {
    const user = { email: "user@example.com", name: "<Alex> & Team" };
    await sendVerificationEmail(user, "https://app.example.com/verify");
    await sendPasswordResetEmail(user, "https://app.example.com/reset");
    const payloads = fetchEmail.mock.calls.map((call) =>
      JSON.parse(call[1].body),
    );
    expect(payloads[0].template).toEqual({
      id: "verification-template",
      variables: {
        USER_NAME: "&lt;Alex&gt; &amp; Team",
        VERIFICATION_URL: "https://app.example.com/verify",
      },
    });
    expect(payloads[1].template.id).toBe("reset-template");
    expect(payloads[1].template.variables.RESET_URL).toBe(
      "https://app.example.com/reset",
    );
    expect(payloads[0].from).toBe(payloads[1].from);
  });
  it("requires configuration and reports provider failure without leaking its response", async () => {
    vi.stubEnv("RESEND_VERIFY_EMAIL_TEMPLATE_ID", "");
    expect(() => assertAuthEmailConfigured("verification")).toThrow();
    fetchEmail.mockResolvedValue(
      Response.json({ message: "private provider data" }, { status: 500 }),
    );
    await expect(
      sendPasswordResetEmail(
        { email: "user@example.com", name: "Alex" },
        "https://app.example.com/reset",
      ),
    ).rejects.toThrow("Unable to send password-reset email");
  });
});
