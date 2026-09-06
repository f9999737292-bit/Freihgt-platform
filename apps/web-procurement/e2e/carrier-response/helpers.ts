import { expect, type Page } from '@playwright/test'

export const jwt = process.env.BROWSER_E2E_JWT || ''
export const tenantId = process.env.BROWSER_E2E_TENANT_ID || ''
export const carrierCompanyId = process.env.BROWSER_E2E_CARRIER_COMPANY_ID || ''
export const buyerCompanyId = process.env.BROWSER_E2E_BUYER_COMPANY_ID || ''
export const eventId = process.env.BROWSER_E2E_EVENT_ID || ''
export const conflictEventId = process.env.BROWSER_E2E_CONFLICT_EVENT_ID || eventId
export const rfxNumber = process.env.BROWSER_E2E_RFX_NUMBER || ''
export const conflictRfxNumber = process.env.BROWSER_E2E_CONFLICT_RFX_NUMBER || rfxNumber
export const eventTitle = process.env.BROWSER_E2E_EVENT_TITLE || ''
export const conflictEventTitle = process.env.BROWSER_E2E_CONFLICT_EVENT_TITLE || eventTitle
export const userId = process.env.BROWSER_E2E_USER_ID || ''
export const gatewayURL = process.env.BROWSER_E2E_GATEWAY_URL || ''

export function assertGatewayHost(url: string) {
  if (!gatewayURL) {
    throw new Error('BROWSER_E2E_GATEWAY_URL is required')
  }
  const expected = new URL(gatewayURL)
  const actual = new URL(url)
  if (actual.host !== expected.host) {
    throw new Error(`expected gateway host ${expected.host}, got ${actual.host} for ${url}`)
  }
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
          email: 'carrier-browser-e2e@freight.test',
          full_name: 'Carrier Browser E2E',
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

/** GET /api/v1/rfx-events/{id} is buyer-only at gateway; stub minimal event metadata for page shell. */
export async function stubCarrierEventMetadata(
  page: Page,
  targetEventId = eventId,
  title = eventTitle,
  number = rfxNumber,
) {
  await page.route(`**/api/v1/rfx-events/${targetEventId}`, async (route) => {
    if (route.request().method() !== 'GET') {
      await route.continue()
      return
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        id: targetEventId,
        tenant_id: tenantId,
        owner_company_id: buyerCompanyId,
        rfx_number: number,
        title,
        status: 'PUBLISHED',
        rfx_type: 'SPOT_RFQ',
        category: 'FREIGHT',
        response_deadline: new Date(Date.now() + 48 * 3600 * 1000).toISOString(),
      }),
    })
  })
}

export async function stubAllCarrierEventMetadata(page: Page) {
  await stubCarrierEventMetadata(page)
  if (conflictEventId && conflictEventId !== eventId) {
    await stubCarrierEventMetadata(page, conflictEventId, conflictEventTitle, conflictRfxNumber)
  }
}

export async function stubCarrierCompanyContext(page: Page) {
  await page.route(`**/api/v1/users/${userId}/companies**`, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        items: [
          {
            membership_id: `${carrierCompanyId}-membership`,
            company_id: carrierCompanyId,
            legal_name: 'Carrier A',
            company_type: 'CARRIER',
            membership_status: 'ACTIVE',
            roles: [{ code: 'CARRIER_DISPATCHER', name: 'Carrier Dispatcher' }],
          },
        ],
      }),
    })
  })
  await page.route('**/api/v1/companies**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        items: [
          {
            id: buyerCompanyId || carrierCompanyId,
            legal_name: 'Buyer A',
            company_type: 'SHIPPER',
            status: 'ACTIVE',
          },
          {
            id: carrierCompanyId,
            legal_name: 'Carrier A',
            company_type: 'CARRIER',
            status: 'ACTIVE',
          },
        ],
      }),
    })
  })
}

export function questionnairePath(forEventId = eventId) {
  return `/carrier/tenders/${forEventId}/questionnaire`
}

export async function waitForCarrierWorkspace(page: Page) {
  await expect(page.getByTestId('carrier-response-workspace')).toBeVisible({ timeout: 120_000 })
}

export async function waitForCarrierWorkspaceLoad(page: Page, forEventId = eventId) {
  return page.waitForResponse(
    (resp) => {
      const url = resp.url()
      if (!url.includes(`/api/v1/rfx-events/${forEventId}/carrier-response`)) {
        return false
      }
      if (resp.status() >= 500) {
        return false
      }
      const method = resp.request().method()
      if (method === 'POST' && url.includes('/start')) {
        return true
      }
      return method === 'GET'
        && !url.includes('/answers')
        && !url.includes('/validate')
        && !url.includes('/submit')
    },
    { timeout: 120_000 },
  )
}

export async function expectAutosaveSaved(page: Page) {
  const status = page.getByTestId('autosave-status')
  await expect(status).not.toHaveText(/Есть несохранённые|Сохранение/i, { timeout: 30_000 })
  await expect(status).toHaveText(/Сохранено/i, { timeout: 30_000 })
}

export async function expectAutosaveInvalid(page: Page) {
  const status = page.getByTestId('autosave-status')
  await expect(status).toHaveText(/ошибки — изменения не сохранены/i, { timeout: 30_000 })
}

export function questionRoot(page: Page, code: string) {
  return page.getByTestId(`question-${code}`)
}

export async function setYesNo(page: Page, code: string, value: boolean) {
  const root = questionRoot(page, code)
  const label = value ? 'Да' : 'Нет'
  await root.getByText(label, { exact: true }).click()
}

export async function fillTextQuestion(page: Page, code: string, value: string) {
  const input = questionRoot(page, code).locator('input[type="text"], textarea').first()
  await input.fill(value)
  await input.blur()
}

export async function fillDateQuestion(page: Page, code: string, value: string) {
  const input = questionRoot(page, code).locator('input[type="date"]').first()
  await input.fill(value)
  await input.blur()
}

export async function fillNumberQuestion(page: Page, code: string, value: string) {
  const input = questionRoot(page, code).locator('input[type="number"]').first()
  await input.fill(value)
  await input.blur()
}

export async function waitForAnswersPatch(page: Page, forEventId = eventId) {
  return page.waitForResponse(
    (resp) =>
      resp.url().includes(`/api/v1/rfx-events/${forEventId}/carrier-response/answers`)
      && resp.request().method() === 'PATCH',
    { timeout: 60_000 },
  )
}

export async function flushAutosave(page: Page) {
  await page.waitForTimeout(900)
  await expectAutosaveSaved(page)
}
