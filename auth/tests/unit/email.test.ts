// @vitest-environment node
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createMailer } from "../../src/email";
const config = {
  apiKey: "test-key",
  from: "RoleCue <no-reply@auth.example.com>",
  supportEmail: "support@example.com",
  appURL: "https://app.example.com",
  templates: {
    verification: "verification-template",
    "password-reset": "reset-template",
    locked: "3667a438-9b17-4696-8b07-d36e382fffe2",
    unlocked: "b7757145-fc97-4153-b926-c904af1edfdd",
  },
};
const {
  sendAccountStatusEmail,
  sendPasswordResetEmail,
  sendVerificationEmail,
} = createMailer(config);
const fetchEmail = vi.fn();
beforeEach(() => {
  fetchEmail
    .mockReset()
    .mockImplementation(async () => Response.json({ id: "email-1" }));
  vi.stubGlobal("fetch", fetchEmail);
});
afterEach(() => vi.unstubAllGlobals());
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
  it("tells the user when a timed lock ends, in UTC and inside the escaped reason", async () => {
    const user = { email: "user@example.com", name: "Alex" };
    await sendAccountStatusEmail(
      "locked",
      user,
      "Spam <messages>.",
      new Date("2026-11-05T10:00:45.000Z"),
    );
    await sendAccountStatusEmail("locked", user, "Spam.", null);
    await sendAccountStatusEmail(
      "unlocked",
      user,
      "",
      new Date("2026-11-05T10:00:00Z"),
    );
    const timed = JSON.parse(fetchEmail.mock.calls[0][1].body);
    const indefinite = JSON.parse(fetchEmail.mock.calls[1][1].body);
    const unlocked = JSON.parse(fetchEmail.mock.calls[2][1].body);
    expect(timed.template.variables.LOCK_REASON).toBe(
      "Spam &lt;messages&gt;. Your access returns on 2026-11-05 10:00 UTC.",
    );
    // No end date for an indefinite lock, and an unlock notice never carries one.
    expect(indefinite.template.variables.LOCK_REASON).toBe("Spam.");
    // An empty reason still reads as one sentence, with no leading space.
    await sendAccountStatusEmail(
      "locked",
      user,
      "",
      new Date("2026-11-05T10:00:00Z"),
    );
    const noReason = JSON.parse(fetchEmail.mock.calls[3][1].body);
    expect(noReason.template.variables.LOCK_REASON).toBe(
      "Your access returns on 2026-11-05 10:00 UTC.",
    );
    expect(unlocked.template.variables).not.toHaveProperty("LOCK_REASON");
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
    await expect(
      createMailer({
        ...config,
        supportEmail: undefined,
      }).sendAccountStatusEmail(
        "unlocked",
        { email: "a@example.com", name: "A" },
        "",
      ),
    ).rejects.toThrow("Missing support email");
    expect(fetchEmail).not.toHaveBeenCalled();
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
