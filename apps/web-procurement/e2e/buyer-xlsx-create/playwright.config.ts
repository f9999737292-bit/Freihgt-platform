import { defineConfig } from '@playwright/test'

function requireEnv(name: string): string {
  const value = (process.env[name] || '').trim()
  if (!value) {
    throw new Error(`${name} is required for the buyer XLSX create browser gate`)
  }
  return value
}

if (process.env.BROWSER_E2E !== '1') {
  throw new Error('BROWSER_E2E=1 is required for the buyer XLSX create browser gate')
}
if (process.env.BROWSER_E2E_REQUIRE !== '1') {
  throw new Error('BROWSER_E2E_REQUIRE=1 is required for the buyer XLSX create browser gate')
}

const webURL = requireEnv('BROWSER_E2E_WEB_URL')
requireEnv('BROWSER_E2E_GATEWAY_URL')
requireEnv('BROWSER_E2E_JWT')
requireEnv('BROWSER_E2E_WORKBOOK_PATH')
requireEnv('BROWSER_E2E_FLAG_OFF_WEB_URL')
requireEnv('BROWSER_E2E_FLAG_OFF_GATEWAY_URL')

export default defineConfig({
  testDir: '.',
  testMatch: '**/*.spec.ts',
  timeout: 180_000,
  expect: { timeout: 30_000 },
  fullyParallel: false,
  workers: 1,
  retries: 0,
  forbidOnly: true,
  outputDir: 'test-results',
  use: {
    baseURL: webURL,
    locale: 'en-US',
    acceptDownloads: true,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  reporter: [['list'], ['json', { outputFile: 'test-results/results.json' }]],
})
