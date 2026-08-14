import { defineConfig } from '@playwright/test'

const baseURL = process.env.E2E_FRONTEND_URL || 'http://127.0.0.1:3000'

export default defineConfig({
  testDir: '.',
  testMatch: 'control-tower.spec.mjs',
  timeout: 120_000,
  expect: { timeout: 20_000 },
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: [['list'], ['html', { open: 'never', outputFolder: 'artifacts/html-report' }]],
  use: {
    baseURL,
    channel: 'msedge',
    headless: true,
    screenshot: 'off',
    trace: 'retain-on-failure',
    video: 'off',
    launchOptions: {
      // Local-only: Nuxt dev binds 127.0.0.1 while API Gateway CORS allows localhost:3000.
      args: ['--disable-web-security', '--disable-features=IsolateOrigins,site-per-process'],
    },
  },
  outputDir: 'artifacts/test-results',
})
