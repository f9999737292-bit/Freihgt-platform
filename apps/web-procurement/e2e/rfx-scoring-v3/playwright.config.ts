import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: '.',
  timeout: 240_000,
  expect: { timeout: 60_000 },
  workers: 1,
  retries: 0,
  use: {
    baseURL: process.env.BROWSER_E2E_ADMIN_URL || 'http://127.0.0.1:3022',
    locale: 'ru-RU',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  reporter: [['list']],
})
