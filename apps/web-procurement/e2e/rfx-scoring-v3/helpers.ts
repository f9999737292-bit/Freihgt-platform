import { expect, type APIRequestContext, type Page } from '@playwright/test'

export const adminURL = process.env.BROWSER_E2E_ADMIN_URL || ''
export const procurementURL = process.env.BROWSER_E2E_PROCUREMENT_URL || ''
export const gatewayURL = process.env.BROWSER_E2E_GATEWAY_URL || ''
export const jwt = process.env.BROWSER_E2E_JWT || ''
export const tenantId = process.env.BROWSER_E2E_TENANT_ID || ''
export const companyId = process.env.BROWSER_E2E_BUYER_COMPANY_ID || ''
export const eventId = process.env.BROWSER_E2E_EVENT_ID || ''
export const userId = process.env.BROWSER_E2E_USER_ID || ''
export const carrierAJWT = process.env.BROWSER_E2E_CARRIER_A_JWT || ''
export const carrierACompany = process.env.BROWSER_E2E_CARRIER_A_COMPANY_ID || ''
export const carrierBJWT = process.env.BROWSER_E2E_CARRIER_B_JWT || ''
export const carrierBCompany = process.env.BROWSER_E2E_CARRIER_B_COMPANY_ID || ''
export const legacyEventId = process.env.BROWSER_E2E_LEGACY_EVENT_ID || ''

export function assertGatewayHost(url: string) {
  const expected = new URL(gatewayURL)
  const actual = new URL(url)
  expect(actual.host).toBe(expected.host)
}

export async function seedBuyerAdminSession(page: Page) {
  await page.addInitScript(({ token, tenant, company, user }) => {
    localStorage.setItem(
      'freight_admin_session',
      JSON.stringify({
        token,
        user: { id: user, tenant_id: tenant, email: 'buyer@freight.test', full_name: 'Buyer', preferred_locale: 'ru-RU', status: 'ACTIVE', roles: ['PROCUREMENT_MANAGER'] },
      }),
    )
    localStorage.setItem('freight_admin_tenant_id', tenant)
    localStorage.setItem('freight_admin_company_id', company)
    document.cookie = 'freight_admin_locale=ru-RU; path=/'
  }, { token: jwt, tenant: tenantId, company: companyId, user: userId })
}

export async function seedBuyerProcurementSession(page: Page) {
  await page.addInitScript(({ token, tenant, company, user }) => {
    localStorage.setItem(
      'freight_procurement_session',
      JSON.stringify({
        token,
        user: { id: user, tenant_id: tenant, email: 'buyer@freight.test', full_name: 'Buyer', preferred_locale: 'ru-RU', status: 'ACTIVE', roles: ['PROCUREMENT_MANAGER'] },
      }),
    )
    localStorage.setItem('freight_procurement_tenant_id', tenant)
    localStorage.setItem('freight_procurement_company_id', company)
    document.cookie = 'freight_procurement_locale=ru-RU; path=/'
  }, { token: jwt, tenant: tenantId, company: companyId, user: userId })
}

async function carrierSubmit(
  request: APIRequestContext,
  token: string,
  carrierCompany: string,
  adr: boolean,
  fleet: number,
) {
  const headers = {
    Authorization: `Bearer ${token}`,
    'X-Company-ID': carrierCompany,
    'Content-Type': 'application/json',
  }
  const start = await request.post(`${gatewayURL}/api/v1/rfx-events/${eventId}/carrier-response/start`, { headers, data: {} })
  expect(start.ok()).toBeTruthy()
  const ws = await start.json()
  const questions = ws.questionnaire?.sections?.flatMap((s: { questions?: Array<{ id: string; question_code: string }> }) => s.questions ?? []) ?? []
  const adrQ = questions.find((q) => q.question_code === 'ADR_AVAILABLE')
  const fleetQ = questions.find((q) => q.question_code === 'FLEET_COUNT')
  expect(adrQ?.id).toBeTruthy()
  expect(fleetQ?.id).toBeTruthy()
  const patch = await request.patch(`${gatewayURL}/api/v1/rfx-events/${eventId}/carrier-response/answers`, {
    headers,
    data: {
      expected_save_version: ws.save_version,
      answers: [
        { question_id: adrQ!.id, value: adr },
        { question_id: fleetQ!.id, value: fleet },
      ],
    },
  })
  expect(patch.ok()).toBeTruthy()
  const saved = await patch.json()
  const submit = await request.post(`${gatewayURL}/api/v1/rfx-events/${eventId}/carrier-response/submit`, {
    headers,
    data: { expected_save_version: saved.save_version },
  })
  expect(submit.ok()).toBeTruthy()
}

export async function submitCarrierA(request: APIRequestContext) {
  await carrierSubmit(request, carrierAJWT, carrierACompany, true, 50)
}

export async function submitCarrierB(request: APIRequestContext) {
  await carrierSubmit(request, carrierBJWT, carrierBCompany, false, 100)
}
