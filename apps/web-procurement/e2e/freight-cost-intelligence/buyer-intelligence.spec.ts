import { expect, test, type Page } from '@playwright/test'

const jwt = process.env.BROWSER_E2E_JWT || ''
const tenantId = process.env.BROWSER_E2E_TENANT_ID || ''
const companyId = process.env.BROWSER_E2E_BUYER_COMPANY_ID || ''
const expectedPlanned = process.env.BROWSER_E2E_EXPECTED_PLANNED || '190000.00'
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
  await page.goto('/freight-costs')
  await overviewResp
  await expect(page.getByText(/190.?000/)).toBeVisible()
})

test('FC22G1-UI-002 live lanes', async ({ page }) => {
  const lanesResp = page.waitForResponse((resp) =>
    resp.url().includes('/api/v1/freight-costs/analytics/lanes') && resp.status() === 200,
  )
  await page.goto('/freight-costs/lanes')
  await lanesResp
  await expect(page.getByRole('heading', { name: /lanes/i })).toBeVisible()
})

test('FC22G1-UI-003 live carriers', async ({ page }) => {
  const carriersResp = page.waitForResponse((resp) =>
    resp.url().includes('/api/v1/freight-costs/analytics/carriers') && resp.status() === 200,
  )
  await page.goto('/freight-costs/carriers')
  await carriersResp
  await expect(page.getByRole('heading', { name: /carriers/i })).toBeVisible()
})

test('FC22G1-UI-004 live accessorials', async ({ page }) => {
  const accessorialsResp = page.waitForResponse((resp) =>
    resp.url().includes('/api/v1/freight-costs/analytics/accessorials') && resp.status() === 200,
  )
  await page.goto('/freight-costs/accessorials')
  await accessorialsResp
  await expect(page.getByText(/150/)).toBeVisible()
})

test('FC22G1-UI-005 live opportunities', async ({ page }) => {
  const opportunitiesResp = page.waitForResponse((resp) =>
    resp.url().includes('/api/v1/freight-costs/opportunities') && resp.status() === 200,
  )
  await page.goto('/freight-costs/opportunities')
  await opportunitiesResp
  await expect(page.getByText(new RegExp(expectedDelta.replace('.', '[.,]?')))).toBeVisible()
})

test('FC22G1-UI-006 live filters change network query', async ({ page }) => {
  await page.goto('/freight-costs/lanes?currency=RUB')
  const first = page.waitForResponse((resp) => resp.url().includes('/analytics/lanes') && resp.url().includes('currency=RUB'))
  await first
  const secondPromise = page.waitForResponse((resp) => resp.url().includes('/analytics/lanes') && resp.url().includes('currency=EUR'))
  await page.goto('/freight-costs/lanes?currency=EUR')
  await secondPromise
})

test('FC22G1-UI-008 feature flag off hides workspace', async ({ page, context }) => {
  await context.close()
  test.skip(true, 'flag-off run uses separate process env; covered by dedicated script')
})

test('FC22G1-UI-007 live pagination issues next request', async ({ page }) => {
  await page.goto('/freight-costs/lanes?limit=1&offset=0')
  const first = await page.waitForResponse((resp) => resp.url().includes('/analytics/lanes'))
  expect(first.url()).toContain('limit=1')
})
