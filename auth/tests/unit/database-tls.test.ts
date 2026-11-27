import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import type { ConnectionOptions } from "node:tls";
import { Client } from "pg";
import { expect, it } from "vitest";

// The auth service passes BETTER_AUTH_DATABASE_URL straight to pg. In production that URL is
// `?sslmode=verify-full&sslrootcert=/etc/ssl/certs/rds-global-bundle.pem`, so pg must read the
// root file itself and keep certificate and hostname verification on. These checks pin that
// behaviour of the exact pg version we ship, so an upgrade cannot silently drop it.
const bundlePath = resolve(__dirname, "../../certs/rds-global-bundle.pem");

// connectionParameters is the parsed form pg connects with; it is missing from @types/pg.
function sslOf(url: string): boolean | ConnectionOptions | undefined {
  const client = new Client({ connectionString: url }) as unknown as {
    connectionParameters: { ssl?: boolean | ConnectionOptions };
  };
  return client.connectionParameters.ssl;
}

it("ships the RDS CA bundle with several certificates", () => {
  const pem = readFileSync(bundlePath, "utf8");
  expect(pem.match(/-----BEGIN CERTIFICATE-----/g)?.length).toBeGreaterThan(50);
  expect(pem).not.toContain("PRIVATE KEY");
});

it("loads sslrootcert from disk as the CA and keeps verification on", () => {
  const ssl = sslOf(
    `postgres://u:p@db.example.rds.amazonaws.com:5432/rolecue?sslmode=verify-full&sslrootcert=${bundlePath}`,
  );
  expect(ssl).toBeTypeOf("object");
  if (typeof ssl !== "object")
    throw new Error("expected an ssl options object");
  expect(ssl.ca).toBe(readFileSync(bundlePath, "utf8"));
  // Undefined leaves Node's default (reject) in place; only an explicit false disables it.
  expect(ssl.rejectUnauthorized).not.toBe(false);
  expect(ssl.checkServerIdentity).toBeUndefined();
});

it("leaves non-TLS URLs untouched", () => {
  expect(sslOf("postgres://u:p@localhost:5432/rolecue?sslmode=disable")).toBe(
    false,
  );
  expect(sslOf("postgres://u:p@localhost:5432/rolecue")).toBeFalsy();
});
