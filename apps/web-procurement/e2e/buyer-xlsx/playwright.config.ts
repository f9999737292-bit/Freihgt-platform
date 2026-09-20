import { defineConfig } from '@playwright/test'

const webURL = process.env.BROWSER_E2E_WEB_URL || 'http://localhost:3005'

export default defineConfig({
  testDir: '.',
  testMatch: '**/*.spec.ts',
  timeout: 120_000,
  expect: { timeout: 20_000 },
  fullyParallel: false,
  workers: 1,
  outputDir: 'test-results',
  use: {
    baseURL: webURL,
    locale: 'en-US',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  reporter: [['list']],
})
