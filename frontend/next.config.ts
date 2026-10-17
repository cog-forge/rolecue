import type { NextConfig } from "next";
const config: NextConfig = {
  output: "standalone",
  turbopack: { root: process.cwd() },
  transpilePackages: ["@doan-labs/peek"],
};
export default config;
