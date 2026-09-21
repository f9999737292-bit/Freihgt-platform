import { expect, type Page, type Response } from '@playwright/test'

export function requireE7Env(name: string): string {
  const value = (process.env[name] || '').trim()
  if (!value) {
    throw new Error(`${name} is required for the E7 browser acceptance gate`)
  }
  return value
}

export const procurementURL = requireE7Env('BROWSER_E2E_PROCUREMENT_URL')
export const adminURL = requireE7Env('BROWSER_E2E_ADMIN_URL')
export const flagOffWebURL = requireE7Env('BROWSER_E2E_FLAG_OFF_WEB_URL')
export const gatewayURL = requireE7Env('BROWSER_E2E_GATEWAY_URL')
export const flagOffGatewayURL = requireE7Env('BROWSER_E2E_FLAG_OFF_GATEWAY_URL')
export const buyerJwt = requireE7Env('BROWSER_E2E_JWT')
export const tenantId = requireE7Env('BROWSER_E2E_TENANT_ID')
export const buyerCompanyId = requireE7Env('BROWSER_E2E_BUYER_COMPANY_ID')
export const buyerUserId = requireE7Env('BROWSER_E2E_USER_ID')
export const carrierJwt = requireE7Env('BROWSER_E2E_CARRIER_JWT')
export const carrierUserId = requireE7Env('BROWSER_E2E_CARRIER_USER_ID')
export const carrierCompanyId = requireE7Env('BROWSER_E2E_CARRIER_COMPANY_ID')
export const otherBuyerJwt = requireE7Env('BROWSER_E2E_OTHER_BUYER_JWT')
export const otherBuyerUserId = requireE7Env('BROWSER_E2E_OTHER_BUYER_USER_ID')
export const otherBuyerCompanyId = requireE7Env('BROWSER_E2E_OTHER_BUYER_COMPANY_ID')
export const logistJwt = requireE7Env('BROWSER_E2E_LOGIST_JWT')
export const logistUserId = requireE7Env('BROWSER_E2E_LOGIST_USER_ID')
export const isolationEventId = requireE7Env('BROWSER_E2E_ISOLATION_EVENT_ID')
export const isolationRfxNumber = requireE7Env('BROWSER_E2E_ISOLATION_RFX_NUMBER')
export const buyerXlsxEventId = requireE7Env('BROWSER_E2E_BUYER_XLSX_EVENT_ID')
export const buyerXlsxRfxNumber = requireE7Env('BROWSER_E2E_BUYER_XLSX_RFX_NUMBER')
export const carrierXlsxEventId = requireE7Env('BROWSER_E2E_CARRIER_XLSX_EVENT_ID')
export const carrierXlsxResponseId = requireE7Env('BROWSER_E2E_CARRIER_XLSX_RESPONSE_ID')
export const carrierXlsxRfxNumber = requireE7Env('BROWSER_E2E_CARRIER_XLSX_RFX_NUMBER')
export const competitorOffer = requireE7Env('BROWSER_E2E_COMPETITOR_OFFER')
export const competitorName = requireE7Env('BROWSER_E2E_COMPETITOR_NAME')
export const competitorAnswer = requireE7Env('BROWSER_E2E_COMPETITOR_ANSWER')
export const templateId = requireE7Env('BROWSER_E2E_TEMPLATE_ID')
export const templateCode = requireE7Env('BROWSER_E2E_TEMPLATE_CODE')
export const lateCreateEventId = requireE7Env('BROWSER_E2E_LATE_CREATE_EVENT_ID')
export const lateCreateRfxNumber = requireE7Env('BROWSER_E2E_LATE_CREATE_RFX_NUMBER')
export const lateSubmitEventId = requireE7Env('BROWSER_E2E_LATE_SUBMIT_EVENT_ID')
export const lateSubmitRfxNumber = requireE7Env('BROWSER_E2E_LATE_SUBMIT_RFX_NUMBER')
export const lateRejectEventId = requireE7Env('BROWSER_E2E_LATE_REJECT_EVENT_ID')
export const lateRejectRfxNumber = requireE7Env('BROWSER_E2E_LATE_REJECT_RFX_NUMBER')
export const lateNotStartedEventId = requireE7Env('BROWSER_E2E_LATE_NOT_STARTED_EVENT_ID')
export const lateExpiredEventId = requireE7Env('BROWSER_E2E_LATE_EXPIRED_EVENT_ID')
export const flagOffXlsxEventId = requireE7Env('BROWSER_E2E_FLAG_OFF_XLSX_EVENT_ID')
export const flagOffLateEventId = requireE7Env('BROWSER_E2E_FLAG_OFF_LATE_EVENT_ID')

export type E7StageResult = { stage: string; method: string; path: string; status: number }

export function logStage(result: E7StageResult) {
  console.log(`E7-STAGE ${result.stage} ${result.method} ${result.path} -> ${result.status}`)
}

export function assertNoPageRoute(page: Page) {
  page.on('request', (req) => {
    if (req.url().includes('__playwright_route__')) {
      throw new Error(`page.route is forbidden in the E7 live gate: ${req.url()}`)
    }
  })
}

export async function seedProcurementSession(
  page: Page,
  input: { token: string; user: string; company: string; roles: string[]; locale?: string },
) {
  const locale = input.locale || 'en-US'
  await page.addInitScript((payload) => {
    localStorage.setItem(
      'freight_procurement_session',
      JSON.stringify({
        token: payload.token,
        user: {
          id: payload.user,
          tenant_id: payload.tenant,
          email: 'e7-browser@freight.test',
          full_name: 'E7 Browser',
          preferred_locale: payload.locale,
          status: 'ACTIVE',
          roles: payload.roles,
        },
      }),
    )
    localStorage.setItem('freight_procurement_tenant_id', payload.tenant)
    localStorage.setItem('freight_procurement_company_id', payload.company)
    document.cookie = `freight_procurement_locale=${payload.locale}; path=/`
  }, {
    token: input.token,
    tenant: tenantId,
    company: input.company,
    user: input.user,
    roles: input.roles,
    locale,
  })
}

export async function seedAdminSession(page: Page) {
  await page.addInitScript((payload) => {
    localStorage.setItem(
      'freight_admin_session',
      JSON.stringify({
        token: payload.token,
        user: {
          id: payload.user,
          tenant_id: payload.tenant,
          email: 'e7-admin@freight.test',
          full_name: 'E7 Admin',
          preferred_locale: 'ru-RU',
          status: 'ACTIVE',
          roles: ['PROCUREMENT_MANAGER'],
        },
      }),
    )
    localStorage.setItem('freight_admin_tenant_id', payload.tenant)
    localStorage.setItem('freight_admin_company_id', payload.company)
    document.cookie = 'freight_admin_locale=ru-RU; path=/'
  }, {
    token: buyerJwt,
    tenant: tenantId,
    company: buyerCompanyId,
    user: buyerUserId,
  })
}

export function waitForApi(
  page: Page,
  match: { method: string; pathIncludes: string; pathExcludes?: string[] },
  timeout = 60_000,
) {
  return page.waitForResponse((resp) => {
    const url = resp.url()
    if (!url.includes(match.pathIncludes) || resp.request().method() !== match.method) {
      return false
    }
    return !(match.pathExcludes ?? []).some((fragment) => url.includes(fragment))
  }, { timeout })
}

export async function clickAndCapture(
  page: Page,
  stage: string,
  match: { method: string; pathIncludes: string },
  click: () => Promise<void>,
): Promise<Response> {
  const [resp] = await Promise.all([
    waitForApi(page, match),
    click(),
  ])
  logStage({
    stage,
    method: match.method,
    path: new URL(resp.url()).pathname,
    status: resp.status(),
  })
  return resp
}

export function authHeaders(token: string, companyId: string) {
  return {
    Authorization: `Bearer ${token}`,
    'X-Company-ID': companyId,
  }
}

export async function expectWorkspace(page: Page, rfxNumber: string) {
  await expect(page.getByTestId('tender-rfx-number')).toHaveText(rfxNumber, { timeout: 30_000 })
}

export function assertGatewayHost(url: string, expected = gatewayURL) {
  expect(new URL(url).host).toBe(new URL(expected).host)
}

export async function dumpWizardEvidence(page: Page) {
  const evidence = await page.evaluate(() => {
    const btn = document.querySelector('[data-testid="wizard-next"]') as HTMLButtonElement | null
    const title = document.querySelector('[data-testid="wizard-title"]') as HTMLInputElement | null
    const owner = document.querySelector('[data-testid="wizard-owner-company"]') as HTMLSelectElement | null
    return {
      step: btn?.getAttribute('data-wizard-step') ?? Array.from(document.querySelectorAll('.wizard-step--active')).map((el) => el.textContent?.trim()),
      nextDisabled: btn?.disabled ?? null,
      nextAriaDisabled: btn?.getAttribute('aria-disabled'),
      modelTitle: btn?.getAttribute('data-model-title'),
      modelOwner: btn?.getAttribute('data-model-owner'),
      titleDom: title?.value ?? null,
      ownerDom: owner?.value ?? null,
      generalError: document.querySelector('[data-testid="wizard-general-error"]')?.textContent ?? null,
    }
  })
  console.log(`E7-WIZARD-EVIDENCE ${JSON.stringify(evidence)}`)
  return evidence
}

export function attachLiveDiagnostics(page: Page, label: string) {
  page.on('console', (msg) => {
    if (msg.type() === 'error') {
      console.log(`E7-${label}-CONSOLE ${msg.type()} ${msg.text()}`)
    }
  })
  page.on('pageerror', (error) => {
    console.log(`E7-${label}-PAGEERROR ${error.message}`)
  })
  page.on('requestfailed', (req) => {
    console.log(`E7-${label}-REQUESTFAILED ${req.method()} ${req.url()} ${req.failure()?.errorText ?? ''}`)
  })
}
