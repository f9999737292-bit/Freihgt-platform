import { defineConfig } from '@playwright/test'

const required = process.env.BROWSER_E2E === '1'
  || process.env.BROWSER_E2E_REQUIRE === '1'
  || process.env.CI === 'true'

function requireEnv(name: string): string {
  const value = (process.env[name] || '').trim()
  if (!value) {
    throw new Error(`${name} is required for the late submission browser gate`)
  }
  return value
}

const webURL = required
  ? requireEnv('BROWSER_E2E_WEB_URL')
  : (process.env.BROWSER_E2E_WEB_URL || '')

if (required) {
  requireEnv('BROWSER_E2E_LATE_FLAG')
  if (process.env.BROWSER_E2E_LATE_FLAG !== '1') {
    throw new Error('BROWSER_E2E_LATE_FLAG=1 is required for the late submission browser gate')
  }
}

if (!webURL) {
  throw new Error('BROWSER_E2E_WEB_URL is required for late submission Playwright; silent skip is not allowed')
}

export default defineConfig({
  testDir: '.',
  testMatch: '**/*.spec.ts',
  timeout: 180_000,
  expect: { timeout: 30_000 },
  fullyParallel: false,
  workers: 1,
  retries: 0,
  outputDir: 'test-results',
  use: {
    baseURL: webURL,
    locale: 'en-US',
    acceptDownloads: true,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  reporter: [['list'], ['json', { outputFile: 'test-results/results.json' }]],
  projects: [
    { name: 'ui-stub', testMatch: 'late-submission-acceptance.spec.ts' },
    { name: 'live-stack', testMatch: 'late-submission-live.spec.ts' },
  ],
})
