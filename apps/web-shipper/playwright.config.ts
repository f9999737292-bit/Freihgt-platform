import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  workers: 1,
  timeout: 60_000,
  use: {
    baseURL: 'http://127.0.0.1:3001',
    headless: true,
    trace: 'retain-on-failure',
  },
  webServer: {
    command: 'node .output/server/index.mjs',
    url: 'http://127.0.0.1:3001/login',
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
    env: {
      HOST: '127.0.0.1',
      PORT: '3001',
    },
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
  ],
})
