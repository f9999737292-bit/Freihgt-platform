import { expect, test, type Page } from '@playwright/test'

const jwt = process.env.BROWSER_E2E_JWT || ''
const tenantId = process.env.BROWSER_E2E_TENANT_ID || ''
const companyId = process.env.BROWSER_E2E_BUYER_COMPANY_ID || ''

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
  await page.goto('/login')
  await page.waitForLoadState('domcontentloaded')
})

test('FC22G1-UI-008 feature flag off hides workspace', async ({ page }) => {
  await page.goto('/freight-costs', { waitUntil: 'domcontentloaded' })
  await page.waitForURL('**/freight-costs/unavailable', { timeout: 30_000 })
  await expect(page.getByText(/Freight cost workspace unavailable/i)).toBeVisible()
  await expect(page.getByText(/NUXT_PUBLIC_FREIGHT_COST_WORKSPACE_ENABLED/i)).toBeVisible()
})
