import type { NextConfig } from "next";
const config: NextConfig = {
  turbopack: { root: process.cwd() },
  transpilePackages: ["@doan-labs/peek"],
};
export default config;
