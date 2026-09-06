import { expect, type Page } from '@playwright/test'

export const jwt = process.env.BROWSER_E2E_JWT || ''
export const tenantId = process.env.BROWSER_E2E_TENANT_ID || ''
export const carrierCompanyId = process.env.BROWSER_E2E_CARRIER_COMPANY_ID || ''
export const eventId = process.env.BROWSER_E2E_EVENT_ID || ''
export const rfxNumber = process.env.BROWSER_E2E_RFX_NUMBER || ''
export const userId = process.env.BROWSER_E2E_USER_ID || ''
export const gatewayURL = process.env.BROWSER_E2E_GATEWAY_URL || ''

export function questionnairePath() {
  return `/carrier/tenders/${eventId}/questionnaire`
}

export async function seedCarrierSession(page: Page) {
  await page.addInitScript(({ token, tenant, company, user }) => {
    localStorage.setItem(
      'freight_procurement_session',
      JSON.stringify({
        token,
        user: {
          id: user,
          tenant_id: tenant,
          email: 'carrier-response-e2e@freight.test',
          full_name: 'Carrier Response E2E',
          preferred_locale: 'ru-RU',
          status: 'ACTIVE',
          roles: ['CARRIER_DISPATCHER'],
        },
      }),
    )
    localStorage.setItem('freight_procurement_tenant_id', tenant)
    localStorage.setItem('freight_procurement_company_id', company)
    document.cookie = 'freight_procurement_locale=ru-RU; path=/'
  }, { token: jwt, tenant: tenantId, company: carrierCompanyId, user: userId })
}

export async function waitForSaved(page: Page) {
  const status = page.getByTestId('autosave-status')
  await expect(status).toHaveText(/Сохранено/i, { timeout: 60_000 })
}

export async function waitForInvalid(page: Page) {
  const status = page.getByTestId('autosave-status')
  await expect(status).toHaveText(/ошибки/i, { timeout: 60_000 })
}

export function assertGatewayHost(url: string) {
  if (!gatewayURL) throw new Error('BROWSER_E2E_GATEWAY_URL is required')
  const expected = new URL(gatewayURL)
  const actual = new URL(url)
  if (actual.host !== expected.host) {
    throw new Error(`expected gateway host ${expected.host}, got ${actual.host}`)
  }
}
