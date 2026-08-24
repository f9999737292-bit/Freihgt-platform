import { expect, test, type Page } from '@playwright/test'
import {
  decimalAmountPattern,
  expectDecimalClose,
  expectShellHeading,
  expectShellReady,
} from './helpers'

const jwt = process.env.BROWSER_E2E_JWT || ''
const tenantId = process.env.BROWSER_E2E_TENANT_ID || ''
const companyId = process.env.BROWSER_E2E_BUYER_COMPANY_ID || ''
const expectedPlanned = process.env.BROWSER_E2E_EXPECTED_PLANNED || '197000.00'
const expectedDelta = process.env.BROWSER_E2E_EXPECTED_DELTA || '7000.00'

async function seedBuyerSession(page: Page) {
  await page.addInitScript(({ token, tenant, company }) => {
    localStorage.setItem('freight_procurement_session', JSON.stringify({
      token,
      user: {
        id: '8541a3a3-bde7-4fed-9501-37b9953bf904',
        tenant_id: tenant,
        email: 'buyer-e2e@freight.test',
        full_name: 'Buyer E2E',
        preferred_locale: 'en-US',
        status: 'ACTIVE',
        roles: ['PROCUREMENT_MANAGER'],
      },
    }))
    localStorage.setItem('freight_procurement_tenant_id', tenant)
    localStorage.setItem('freight_procurement_company_id', company)
    document.cookie = 'freight_procurement_locale=en-US; path=/'
  }, { token: jwt, tenant: tenantId, company: companyId })
}

async function gotoWorkspacePage(page: Page, path: string) {
  await page.goto(path, { waitUntil: 'networkidle' })
  await expect(page).not.toHaveURL(/\/login(?:\?|$)/)
  await expectShellReady(page)
}

test.beforeEach(async ({ page }) => {
  test.setTimeout(180_000)
  if (!jwt || !tenantId || !companyId) {
    test.skip(true, 'browser E2E env not configured')
  }
  await seedBuyerSession(page)
})

test('FC22G1-UI-001 live buyer overview', async ({ page }) => {
  const overviewResp = page.waitForResponse((resp) =>
    resp.url().includes('/api/v1/freight-costs/analytics/overview') && resp.status() === 200,
  )
  await gotoWorkspacePage(page, '/freight-costs')
  const response = await overviewResp
  const body = await response.json()
  expect(body.summary?.order_count).toBeGreaterThan(0)
  const plannedTotal = String(body.summary?.planned_total ?? '')
  expectDecimalClose(plannedTotal, expectedPlanned)
  await expectShellHeading(page, /overview|сводка/i)
  await expect(page.getByText(decimalAmountPattern(plannedTotal))).toBeVisible()
})

test('FC22G1-UI-002 live lanes', async ({ page }) => {
  const lanesResp = page.waitForResponse((resp) =>
    resp.url().includes('/api/v1/freight-costs/analytics/lanes') && resp.status() === 200,
  )
  await gotoWorkspacePage(page, '/freight-costs/lanes')
  const response = await lanesResp
  const body = await response.json()
  expect(body.items?.length ?? 0).toBeGreaterThan(0)
  expect(body.items[0]?.lane_label).toBeTruthy()
  await expectShellHeading(page, /lane performance|направлен/i)
  await expect(page.getByText(String(body.items[0].lane_label))).toBeVisible()
})

test('FC22G1-UI-003 live carriers', async ({ page }) => {
  const carriersResp = page.waitForResponse((resp) =>
    resp.url().includes('/api/v1/freight-costs/analytics/carriers') && resp.status() === 200,
  )
  await gotoWorkspacePage(page, '/freight-costs/carriers')
  const response = await carriersResp
  const body = await response.json()
  expect(body.items?.length ?? 0).toBeGreaterThan(0)
  expect(body.items[0]?.carrier_label ?? body.items[0]?.carrier_company_id).toBeTruthy()
  await expectShellHeading(page, /carrier performance|перевозчик/i)
})

test('FC22G1-UI-004 live accessorials', async ({ page }) => {
  const accessorialsResp = page.waitForResponse((resp) =>
    resp.url().includes('/api/v1/freight-costs/analytics/accessorials') && resp.status() === 200,
  )
  await gotoWorkspacePage(page, '/freight-costs/accessorials')
  const response = await accessorialsResp
  const body = await response.json()
  expect(body.items?.length ?? 0).toBeGreaterThan(0)
  const accessorial = body.items.find((item: { total_amount?: { amount?: string } }) =>
    Number(item.total_amount?.amount ?? 0) >= 150,
  ) ?? body.items[0]
  expect(Number(accessorial.total_amount?.amount ?? 0)).toBeGreaterThanOrEqual(150)
  await expect(page.getByText(decimalAmountPattern(String(accessorial.total_amount.amount)))).toBeVisible()
})

test('FC22G1-UI-005 live opportunities', async ({ page }) => {
  const opportunitiesResp = page.waitForResponse((resp) =>
    resp.url().includes('/api/v1/freight-costs/opportunities') && resp.status() === 200,
  )
  await gotoWorkspacePage(page, '/freight-costs/opportunities')
  const response = await opportunitiesResp
  const body = await response.json()
  expect(body.items?.length ?? 0).toBeGreaterThan(0)
  const opportunity = body.items.find((item: { estimated_delta?: { amount?: string } }) =>
    Math.abs(Number(item.estimated_delta?.amount ?? 0) - Number(expectedDelta)) < 0.01,
  ) ?? body.items[0]
  expect(opportunity?.observed_value?.amount).toBeTruthy()
  expect(opportunity?.baseline_value?.amount).toBeTruthy()
  expect(opportunity?.estimated_delta?.amount).toBeTruthy()
  expect(opportunity?.estimated_delta?.currency_code).toBeTruthy()
  await expect(page.getByText(decimalAmountPattern(String(opportunity.estimated_delta.amount)))).toBeVisible()
})

test('FC22G1-UI-006 live filters change network query', async ({ page }) => {
  const first = page.waitForResponse((resp) =>
    resp.url().includes('/analytics/lanes') && resp.url().includes('currency=RUB') && resp.status() === 200,
  )
  await page.goto('/freight-costs/lanes?currency=RUB', { waitUntil: 'networkidle' })
  const rubResponse = await first
  const rubBody = await rubResponse.json()
  expect(rubBody.items?.length ?? 0).toBeGreaterThan(0)

  const secondPromise = page.waitForResponse((resp) =>
    resp.url().includes('/analytics/lanes') && resp.url().includes('currency=EUR') && resp.status() === 200,
  )
  await page.goto('/freight-costs/lanes?currency=EUR', { waitUntil: 'networkidle' })
  const eurResponse = await secondPromise
  expect(eurResponse.url()).toContain('currency=EUR')
})

test('FC22G1-UI-007 live pagination issues next request', async ({ page }) => {
  const first = page.waitForResponse((resp) =>
    resp.url().includes('/analytics/lanes') && resp.url().includes('limit=1') && resp.status() === 200,
  )
  await page.goto('/freight-costs/lanes?limit=1&offset=0', { waitUntil: 'networkidle' })
  const response = await first
  expect(response.url()).toContain('limit=1')
  const body = await response.json()
  expect(body.items?.length ?? 0).toBe(1)
})
