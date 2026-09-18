import { defineConfig } from "@playwright/test";

const readinessMode = process.env.BROWSER_E2E_SCORING_READINESS === "1";
const ready03LoopMode = process.env.BROWSER_E2E_READY03_LOOP === "1";
const criterionLoopMode = process.env.BROWSER_E2E_CRITERION_LOOP === "1";

export default defineConfig({
  testDir: ".",
  testMatch: criterionLoopMode
    ? ["**/scoring-acceptance-criterion-loop.spec.ts"]
    : ready03LoopMode
      ? ["**/scoring-readiness-ready03-loop.spec.ts"]
      : readinessMode
        ? ["**/scoring-readiness.spec.ts"]
        : ["**/scoring-acceptance.spec.ts"],
  timeout: 240_000,
  expect: { timeout: 60_000 },
  workers: 1,
  retries: 0,
  outputDir: "test-results",
  use: {
    baseURL: process.env.BROWSER_E2E_ADMIN_URL || "http://127.0.0.1:3022",
    locale: "ru-RU",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  reporter: [["list"], ["json", { outputFile: "test-results/results.json" }]],
});
