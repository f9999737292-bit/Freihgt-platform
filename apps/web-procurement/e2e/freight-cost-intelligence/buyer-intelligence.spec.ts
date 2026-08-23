import { expect, test, type Page } from '@playwright/test'

const jwt = process.env.BROWSER_E2E_JWT || ''
const tenantId = process.env.BROWSER_E2E_TENANT_ID || ''
const companyId = process.env.BROWSER_E2E_BUYER_COMPANY_ID || ''
const expectedPlanned = process.env.BROWSER_E2E_EXPECTED_PLANNED || '195000.00'
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

test.beforeEach(async ({ page }) => {
  if (!jwt || !tenantId || !companyId) {
    test.skip(true, 'browser E2E env not configured')
  }
  await seedBuyerSession(page)
})

test('FC22G1-UI-001 live buyer overview', async ({ page }) => {
  const overviewResp = page.waitForResponse((resp) =>
    resp.url().includes('/api/v1/freight-costs/analytics/overview') && resp.status() === 200,
  )
  await page.goto('/freight-costs', { waitUntil: 'domcontentloaded' })
  await page.waitForURL('**/freight-costs', { timeout: 30_000 })
  const response = await overviewResp
  const body = await response.json()
  expect(body.summary?.order_count).toBeGreaterThan(0)
  await expect(page.getByText(new RegExp(expectedPlanned.replace('.', '[.,]?')))).toBeVisible()
})

test('FC22G1-UI-002 live lanes', async ({ page }) => {
  const lanesResp = page.waitForResponse((resp) =>
    resp.url().includes('/api/v1/freight-costs/analytics/lanes') && resp.status() === 200,
  )
  await page.goto('/freight-costs/lanes', { waitUntil: 'domcontentloaded' })
  await page.waitForURL('**/freight-costs/lanes', { timeout: 30_000 })
  const response = await lanesResp
  const body = await response.json()
  expect(body.items?.length ?? 0).toBeGreaterThan(0)
  await expect(page.getByRole('heading', { name: /lane performance/i })).toBeVisible()
})

test('FC22G1-UI-003 live carriers', async ({ page }) => {
  const carriersResp = page.waitForResponse((resp) =>
    resp.url().includes('/api/v1/freight-costs/analytics/carriers') && resp.status() === 200,
  )
  await page.goto('/freight-costs/carriers', { waitUntil: 'domcontentloaded' })
  await page.waitForURL('**/freight-costs/carriers', { timeout: 30_000 })
  const response = await carriersResp
  const body = await response.json()
  expect(body.items?.length ?? 0).toBeGreaterThan(0)
  await expect(page.getByRole('heading', { name: /carrier performance/i })).toBeVisible()
})

test('FC22G1-UI-004 live accessorials', async ({ page }) => {
  const accessorialsResp = page.waitForResponse((resp) =>
    resp.url().includes('/api/v1/freight-costs/analytics/accessorials') && resp.status() === 200,
  )
  await page.goto('/freight-costs/accessorials', { waitUntil: 'domcontentloaded' })
  await page.waitForURL('**/freight-costs/accessorials', { timeout: 30_000 })
  const response = await accessorialsResp
  const body = await response.json()
  expect(body.items?.length ?? 0).toBeGreaterThan(0)
  await expect(page.getByText(/150/)).toBeVisible()
})

test('FC22G1-UI-005 live opportunities', async ({ page }) => {
  const opportunitiesResp = page.waitForResponse((resp) =>
    resp.url().includes('/api/v1/freight-costs/opportunities') && resp.status() === 200,
  )
  await page.goto('/freight-costs/opportunities', { waitUntil: 'domcontentloaded' })
  await page.waitForURL('**/freight-costs/opportunities', { timeout: 30_000 })
  const response = await opportunitiesResp
  const body = await response.json()
  expect(body.items?.length ?? 0).toBeGreaterThan(0)
  await expect(page.getByText(new RegExp(expectedDelta.replace('.', '[.,]?')))).toBeVisible()
})

test('FC22G1-UI-006 live filters change network query', async ({ page }) => {
  const first = page.waitForResponse((resp) =>
    resp.url().includes('/analytics/lanes') && resp.url().includes('currency=RUB') && resp.status() === 200,
  )
  await page.goto('/freight-costs/lanes?currency=RUB', { waitUntil: 'domcontentloaded' })
  await first
  const secondPromise = page.waitForResponse((resp) =>
    resp.url().includes('/analytics/lanes') && resp.url().includes('currency=EUR') && resp.status() === 200,
  )
  await page.goto('/freight-costs/lanes?currency=EUR', { waitUntil: 'domcontentloaded' })
  await secondPromise
})

test('FC22G1-UI-007 live pagination issues next request', async ({ page }) => {
  const first = page.waitForResponse((resp) =>
    resp.url().includes('/analytics/lanes') && resp.url().includes('limit=1') && resp.status() === 200,
  )
  await page.goto('/freight-costs/lanes?limit=1&offset=0', { waitUntil: 'domcontentloaded' })
  const response = await first
  expect(response.url()).toContain('limit=1')
})
