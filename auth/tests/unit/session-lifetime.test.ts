import { expect, it } from "vitest";
import { sessionOptions } from "../../src/auth";
import { readEnv } from "../../src/env";

const base = {
  APP_URL: "http://localhost:3000",
  AUTH_BASE_URL: "http://localhost:3000",
  BETTER_AUTH_SECRET: "fixture-signing-secret-at-least-32-characters",
  ADMIN_AUTH_INTERNAL_SECRET: "fixture-admin-secret-at-least-32-characters",
  BETTER_AUTH_DATABASE_URL: "postgres://test:test@localhost/test",
  RESEND_API_KEY: "test-key",
  RESEND_FROM_EMAIL: "RoleCue <no-reply@example.com>",
  RESEND_VERIFY_EMAIL_TEMPLATE_ID: "verification-template",
  RESEND_RESET_PASSWORD_TEMPLATE_ID: "reset-template",
};

it("keeps the 14 day session lifetime unless AUTH_SESSION_MAX_AGE_DAYS says otherwise", () => {
  const defaults = readEnv(base);
  expect(defaults.sessionMaxAgeDays).toBe(14);
  expect(sessionOptions(defaults.sessionMaxAgeDays).expiresIn).toBe(
    14 * 24 * 60 * 60,
  );
  const short = readEnv({ ...base, AUTH_SESSION_MAX_AGE_DAYS: "3" });
  expect(sessionOptions(short.sessionMaxAgeDays)).toMatchObject({
    expiresIn: 3 * 24 * 60 * 60,
    updateAge: 24 * 60 * 60,
    modelName: "sessions",
  });
});

it("refreshes a one day session while it is still valid", () => {
  expect(sessionOptions(1)).toMatchObject({
    expiresIn: 86_400,
    updateAge: 43_200,
  });
});

it("rejects a session lifetime outside 1 to 90 whole days", () => {
  for (const value of ["0", "-1", "1.5", "abc", "91"])
    expect(() =>
      readEnv({ ...base, AUTH_SESSION_MAX_AGE_DAYS: value }),
    ).toThrow(/AUTH_SESSION_MAX_AGE_DAYS/);
});
