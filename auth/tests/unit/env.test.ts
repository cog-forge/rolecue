import { expect, it } from "vitest";
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
it("separates public origins from private listener and validates production cookie scope", () => {
  expect(readEnv(base)).toMatchObject({
    port: 3001,
    secure: false,
    cookiePrefix: "rolecue",
  });
  expect(
    readEnv({
      ...base,
      NODE_ENV: "production",
      APP_URL: "https://app.example.com",
      AUTH_BASE_URL: "https://app.example.com",
      BETTER_AUTH_COOKIE_DOMAIN: "example.com",
    }).secure,
  ).toBe(true);
  for (const value of [
    "http://auth:3001",
    "https://user:password@example.com",
    "https://app.example.com/",
    "https://app.example.com?secret=test",
  ])
    expect(() => readEnv({ ...base, AUTH_BASE_URL: value })).toThrow();
  expect(() => readEnv({ ...base, GOOGLE_CLIENT_ID: "partial" })).toThrow();
  expect(() => readEnv({ ...base, NODE_ENV: "production" })).toThrow();
  expect(() =>
    readEnv({
      ...base,
      APP_URL: "https://app.example.com",
      AUTH_BASE_URL: "https://app.example.com",
      BETTER_AUTH_COOKIE_DOMAIN: "other.com",
    }),
  ).toThrow();
  expect(() => readEnv({ ...base, RESEND_API_KEY: "" })).toThrow();
});
