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
  const carrierQuery = `?carrier_company_id=${carrierCompany}`
  const start = await request.post(`${gatewayURL}/api/v1/rfx-events/${eventId}/carrier-response/start${carrierQuery}`, { headers, data: {} })
  expect(start.ok()).toBeTruthy()
  const ws = await start.json()
  const questions = ws.questionnaire?.sections?.flatMap((s: { questions?: Array<{ id: string; question_code: string }> }) => s.questions ?? []) ?? []
  const adrQ = questions.find((q: { question_code: string }) => q.question_code === 'ADR_AVAILABLE')
  const fleetQ = questions.find((q: { question_code: string }) => q.question_code === 'FLEET_COUNT')
  expect(adrQ?.id).toBeTruthy()
  expect(fleetQ?.id).toBeTruthy()
  const patch = await request.patch(`${gatewayURL}/api/v1/rfx-events/${eventId}/carrier-response/answers${carrierQuery}`, {
    headers,
    data: {
      save_version: ws.save_version,
      answers: [
        { question_id: adrQ!.id, value: adr },
        { question_id: fleetQ!.id, value: fleet },
      ],
    },
  })
  expect(patch.ok()).toBeTruthy()
  const saved = await patch.json()
  const submit = await request.post(`${gatewayURL}/api/v1/rfx-events/${eventId}/carrier-response/submit${carrierQuery}`, {
    headers,
    data: { save_version: saved.save_version },
  })
  expect(submit.ok()).toBeTruthy()
}

export async function submitCarrierA(request: APIRequestContext) {
  await carrierSubmit(request, carrierAJWT, carrierACompany, true, 50)
}

export async function submitCarrierB(request: APIRequestContext) {
  await carrierSubmit(request, carrierBJWT, carrierBCompany, false, 100)
}

/** Company-service is not part of the scoring browser chain; stub directory lookups for evaluation UI labels. */
export async function stubBuyerCompanies(page: Page) {
  await page.route('**/api/v1/companies**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        items: [
          {
            id: companyId,
            legal_name: 'Buyer A',
            company_type: 'SHIPPER',
            status: 'ACTIVE',
          },
          {
            id: carrierACompany,
            legal_name: 'Carrier A',
            company_type: 'CARRIER',
            status: 'ACTIVE',
          },
          {
            id: carrierBCompany,
            legal_name: 'Carrier B',
            company_type: 'CARRIER',
            status: 'ACTIVE',
          },
        ],
      }),
    })
  })
}

export function evaluationPath(forEventId = eventId) {
  return `/tenders/${forEventId}/evaluation`
}

/** Seed procurement-origin localStorage and warm Pinia session before cross-app studio steps. */
export async function bootstrapProcurementSession(page: Page) {
  await page.goto(`${procurementURL}/tenders`, { waitUntil: 'domcontentloaded' })
  await expect(page).not.toHaveURL(/\/login(?:\?|$)/, { timeout: 30_000 })
  await page.waitForFunction(() => {
    const raw = localStorage.getItem('freight_procurement_session')
    if (!raw) return false
    try {
      const data = JSON.parse(raw) as { token?: string }
      return Boolean(data.token)
    } catch {
      return false
    }
  }, undefined, { timeout: 30_000 })
}

export async function assertBrowserScoresApi(page: Page, responseIds: string[], forEventId = eventId) {
  for (const responseId of responseIds) {
    const probe = await page.evaluate(
      async ({ gw, ev, responseId, company, tenantKey, sessionKey }) => {
        const raw = localStorage.getItem(sessionKey)
        if (!raw) return { ok: false, status: 0, reason: 'missing-session', responseId }
        const session = JSON.parse(raw) as { token: string; user: { id: string; tenant_id: string } }
        const tenant = localStorage.getItem(tenantKey) || session.user.tenant_id
        const resp = await fetch(`${gw}/api/v1/rfx-events/${ev}/responses/${responseId}/score`, {
          headers: {
            Authorization: `Bearer ${session.token}`,
            'X-Company-ID': company,
            'X-Tenant-ID': tenant,
            'X-User-ID': session.user.id,
            Accept: 'application/json',
          },
        })
        const body = resp.ok ? await resp.json() : await resp.text()
        return {
          ok: resp.ok,
          status: resp.status,
          reason: resp.ok ? '' : String(body).slice(0, 240),
          responseId,
          calculationStatus: resp.ok ? body?.qualification?.calculation_status : undefined,
          totalScore: resp.ok ? body?.qualification?.total_score : undefined,
        }
      },
      {
        gw: gatewayURL,
        ev: forEventId,
        responseId,
        company: companyId,
        tenantKey: 'freight_procurement_tenant_id',
        sessionKey: 'freight_procurement_session',
      },
    )
    expect(probe.ok, `browser score probe failed for ${responseId}: status=${probe.status} ${probe.reason}`).toBeTruthy()
    expect(probe.calculationStatus).toBe('CALCULATED')
  }
}

export async function assertBrowserResponsesApi(page: Page, forEventId = eventId) {
  const probe = await page.evaluate(
    async ({ gw, ev, company, tenantKey, sessionKey }) => {
      const raw = localStorage.getItem(sessionKey)
      if (!raw) return { ok: false, status: 0, reason: 'missing-session' }
      const session = JSON.parse(raw) as { token: string; user: { id: string; tenant_id: string } }
      const tenant = localStorage.getItem(tenantKey) || session.user.tenant_id
      const resp = await fetch(`${gw}/api/v1/rfx-events/${ev}/responses`, {
        headers: {
          Authorization: `Bearer ${session.token}`,
          'X-Company-ID': company,
          'X-Tenant-ID': tenant,
          'X-User-ID': session.user.id,
          Accept: 'application/json',
        },
      })
      const body = resp.ok ? '' : await resp.text()
      return { ok: resp.ok, status: resp.status, reason: body.slice(0, 240) }
    },
    {
      gw: gatewayURL,
      ev: forEventId,
      company: companyId,
      tenantKey: 'freight_procurement_tenant_id',
      sessionKey: 'freight_procurement_session',
    },
  )
  expect(probe.ok, `browser responses probe failed: status=${probe.status} ${probe.reason}`).toBeTruthy()
}
