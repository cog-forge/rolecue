import type { NextConfig } from "next";
import createNextIntlPlugin from "next-intl/plugin";
import { buildNextHeaders } from "./src/lib/security/headers";

const withNextIntl = createNextIntlPlugin("./src/lib/i18n/request.ts");

const config: NextConfig = {
  output: "standalone",
  poweredByHeader: false,
  turbopack: { root: process.cwd() },
  transpilePackages: ["@doan-labs/peek"],
  headers: buildNextHeaders,
};
export default withNextIntl(config);
