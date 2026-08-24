import { defineConfig } from '@playwright/test'

const webURL = process.env.BROWSER_E2E_WEB_URL || 'http://127.0.0.1:3010'

export default defineConfig({
  testDir: '.',
  timeout: 180_000,
  expect: { timeout: 30_000 },
  use: {
    baseURL: webURL,
    locale: 'en-US',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  reporter: [['list']],
})
