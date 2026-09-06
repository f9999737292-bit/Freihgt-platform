import { defineConfig } from '@playwright/test'

const webURL = process.env.BROWSER_E2E_WEB_URL || 'http://127.0.0.1:3005'

export default defineConfig({
  testDir: '.',
  testMatch: '**/*.spec.ts',
  timeout: 300_000,
  expect: { timeout: 60_000 },
  outputDir: 'test-results',
  use: {
    baseURL: webURL,
    locale: 'ru-RU',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  reporter: [['list']],
})
