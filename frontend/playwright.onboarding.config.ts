import { defineConfig, devices } from "@playwright/test";

// Run against the local app/API and existing Docker DB; no mocked network routes.
export default defineConfig({
  testDir: "./tests/integration",
  testMatch: "onboarding.spec.ts",
  workers: 1,
  outputDir: "./playwright-report/onboarding-live",
  reporter: "list",
  use: {
    baseURL: "http://localhost:3000",
    trace: "retain-on-failure",
    launchOptions: {
      executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH,
    },
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
});
