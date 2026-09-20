import { expect, type Page, type Route } from '@playwright/test'

export const eventId = '11111111-1111-4111-8111-111111111111'
export const responseId = '22222222-2222-4222-8222-222222222222'
export const analysisId = '33333333-3333-4333-8333-333333333333'
export const tenantId = 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa'
export const carrierCompanyId = 'cccccccc-cccc-4ccc-8ccc-cccccccccccc'
export const buyerCompanyId = 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb'
export const userId = '8541a3a3-bde7-4fed-9501-37b9953bf904'

const corsHeaders = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Headers':
    'Accept, Authorization, Content-Type, X-Tenant-ID, X-Company-ID, X-User-ID, X-Request-ID, X-Locale, Idempotency-Key',
  'Access-Control-Allow-Methods': 'GET, POST, PATCH, PUT, DELETE, OPTIONS',
}

export interface CarrierXlsxNetworkProbe {
  console: string[]
  request: string[]
  response: string[]
  requestfailed: string[]
}

export function attachCarrierXlsxNetworkProbe(page: Page): CarrierXlsxNetworkProbe {
  const probe: CarrierXlsxNetworkProbe = {
    console: [],
    request: [],
    response: [],
    requestfailed: [],
  }
  const interesting = (url: string) => url.includes('/api/v1/') || url.includes('/xlsx') || url.includes('/submit')
  page.on('console', (msg) => {
    probe.console.push(`${msg.type()} ${msg.text()}`)
  })
  page.on('request', (req) => {
    if (!interesting(req.url())) return
    probe.request.push(`${req.method()} ${req.url()}`)
  })
  page.on('response', (resp) => {
    if (!interesting(resp.url())) return
    probe.response.push(`${resp.request().method()} ${resp.status()} ${resp.url()}`)
  })
  page.on('requestfailed', (req) => {
    if (!interesting(req.url())) return
    probe.requestfailed.push(`${req.method()} ${req.failure()?.errorText || 'failed'} ${req.url()}`)
  })
  return probe
}

export function formatCarrierXlsxNetworkProbe(probe: CarrierXlsxNetworkProbe): string {
  return [
    `console=${JSON.stringify(probe.console.filter((line) => /error|fail|cors|xlsx|unavailable/i.test(line)))}`,
    `request=${JSON.stringify(probe.request)}`,
    `response=${JSON.stringify(probe.response)}`,
    `requestfailed=${JSON.stringify(probe.requestfailed)}`,
  ].join('\n')
}

export function requireCarrierXlsxEnv(name: string): string {
  const value = (process.env[name] || '').trim()
  if (!value) {
    throw new Error(`${name} is required for the carrier XLSX browser gate`)
  }
  return value
}

export async function fulfillJSON(route: Route, status: number, body: unknown) {
  if (route.request().method() === 'OPTIONS') {
    await route.fulfill({ status: 204, headers: corsHeaders })
    return
  }
  await route.fulfill({
    status,
    contentType: 'application/json',
    headers: corsHeaders,
    body: JSON.stringify(body),
  })
}

export async function withCarrierXlsxCORS(route: Route, handler: (route: Route) => Promise<void>) {
  if (route.request().method() === 'OPTIONS') {
    await route.fulfill({ status: 204, headers: corsHeaders })
    return
  }
  await handler(route)
}

export async function seedCarrierSession(page: Page, roles = ['CARRIER_DISPATCHER']) {
  await page.addInitScript((input) => {
    localStorage.setItem(
      'freight_procurement_session',
      JSON.stringify({
        token: 'browser-e2e-carrier-xlsx',
        user: {
          id: input.user,
          tenant_id: input.tenant,
          email: 'carrier-xlsx-e2e@freight.test',
          full_name: 'Carrier XLSX E2E',
          preferred_locale: 'en-US',
          status: 'ACTIVE',
          roles: input.roles,
        },
      }),
    )
    localStorage.setItem('freight_procurement_tenant_id', input.tenant)
    localStorage.setItem('freight_procurement_company_id', input.company)
    localStorage.setItem('freight_procurement_rfx_excel_exchange', 'true')
    document.cookie = 'freight_procurement_locale=en-US; path=/'
  }, {
    tenant: tenantId,
    company: carrierCompanyId,
    user: userId,
    roles,
  })
}

function ownResponseBody(status: 'DRAFT' | 'SUBMITTED') {
  return {
    id: responseId,
    tenant_id: tenantId,
    rfx_event_id: eventId,
    participant_company_id: carrierCompanyId,
    status,
    offer_lines: [{ rfx_lot_id: '44444444-4444-4444-8444-444444444444', amount: 1000, currency_code: 'RUB' }],
  }
}

export async function stubCarrierTenderWorkspace(
  page: Page,
  options: { responseStatus?: 'DRAFT' | 'SUBMITTED' | 'MISSING' } = {},
) {
  const responseStatus = options.responseStatus ?? 'DRAFT'
  await page.route('**/api/v1/users/**/companies**', async (route) => {
    await fulfillJSON(route, 200, {
      items: [{
        membership_id: `${carrierCompanyId}-membership`,
        company_id: carrierCompanyId,
        legal_name: 'Carrier A',
        company_type: 'CARRIER',
        membership_status: 'ACTIVE',
        roles: [{ code: 'CARRIER_DISPATCHER', name: 'Carrier Dispatcher' }],
      }],
    })
  })
  await page.route('**/api/v1/companies**', async (route) => {
    await fulfillJSON(route, 200, {
      items: [
        { id: buyerCompanyId, legal_name: 'Buyer A', company_type: 'SHIPPER', status: 'ACTIVE' },
        { id: carrierCompanyId, legal_name: 'Carrier A', company_type: 'CARRIER', status: 'ACTIVE' },
      ],
    })
  })
  await page.route('**/api/v1/rfx-events/**', async (route: Route) => {
    if (route.request().method() === 'OPTIONS') {
      await route.fulfill({ status: 204, headers: corsHeaders })
      return
    }
    const url = new URL(route.request().url())
    const method = route.request().method()
    if (method === 'GET' && url.pathname === `/api/v1/rfx-events/${eventId}`) {
      await fulfillJSON(route, 200, {
        id: eventId,
        tenant_id: tenantId,
        owner_company_id: buyerCompanyId,
        rfx_number: 'RFX-CARRIER-XLSX-1',
        title: 'Carrier XLSX draft',
        status: 'PUBLISHED',
        rfx_type: 'LANE_TENDER',
        category: 'FREIGHT',
        currency_code: 'RUB',
        response_deadline: '2026-10-01T12:00:00Z',
      })
      return
    }
    if (method === 'GET' && url.pathname.endsWith('/lots')) {
      await fulfillJSON(route, 200, { items: [] })
      return
    }
    if (method === 'GET' && url.pathname.endsWith('/own-participant')) {
      await fulfillJSON(route, 200, {
        id: '55555555-5555-4555-8555-555555555555',
        company_id: carrierCompanyId,
        participant_type: 'CARRIER',
        status: 'INVITED',
      })
      return
    }
    if (method === 'GET' && url.pathname.endsWith('/own-response')) {
      if (responseStatus === 'MISSING') {
        await fulfillJSON(route, 404, {
          error: { code: 'NOT_FOUND', message: 'own response not found', details: {} },
        })
        return
      }
      await fulfillJSON(route, 200, ownResponseBody(responseStatus))
      return
    }
    if (method === 'GET' && url.pathname.endsWith('/own-award')) {
      await fulfillJSON(route, 404, {
        error: { code: 'NOT_FOUND', message: 'award not found', details: {} },
      })
      return
    }
    if (url.pathname.includes('/xlsx-') || url.pathname.includes('/submit')) {
      await fulfillJSON(route, 599, {
        error: { code: 'INTERNAL_ERROR', message: 'xlsx route was not stubbed', details: {} },
      })
      return
    }
    await route.fallback()
  })
}

export function readyPreviewBody() {
  return {
    schema_name: 'BINTRANS_RFX_CARRIER_XLSX_V1',
    schema_version: '1',
    mode: 'UPDATE_CARRIER_DRAFT',
    target_event_id: eventId,
    target_response_id: responseId,
    target_rfx_version_id: '66666666-6666-4666-8666-666666666666',
    target_version_number: 1,
    target_event_row_version: 1,
    target_response_save_version: 1,
    analysis_id: analysisId,
    expires_at: '2026-09-21T00:00:00Z',
    ready_to_commit: true,
    summary: { errors: 0, warnings: 0 },
    answers_diff: {},
    offer_lines_diff: {},
    errors: [],
    warnings: [],
  }
}

export async function expectWorkspaceLoaded(page: Page, rfxNumber: string) {
  await expect(page.getByRole('button', { name: 'Sign out' })).toBeVisible()
  await expect(page.getByTestId('carrier-tender-rfx-number')).toHaveText(rfxNumber, { timeout: 30_000 })
}

export async function expectPanelVisible(page: Page, status = 'DRAFT') {
  await expect(page.getByTestId('carrier-xlsx-slot')).toBeVisible({ timeout: 30_000 })
  const gate = page.getByTestId('carrier-xlsx-gate')
  await expect(gate).toBeVisible({ timeout: 30_000 })
  await expect(gate).toHaveAttribute('data-enabled', 'true')
  await expect(gate).toHaveAttribute('data-status', new RegExp(status, 'i'))
  await expect(page.getByTestId('carrier-xlsx-panel')).toBeVisible({ timeout: 30_000 })
}
