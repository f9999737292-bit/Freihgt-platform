import { expect, type Page, type Route } from '@playwright/test'

export const eventId = '11111111-1111-4111-8111-111111111111'
export const requestId = '22222222-2222-4222-8222-222222222222'
export const responseId = '33333333-3333-4333-8333-333333333333'
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

export interface LateSubmissionNetworkProbe {
  console: string[]
  request: string[]
  response: string[]
  requestfailed: string[]
}

export function attachLateSubmissionNetworkProbe(page: Page): LateSubmissionNetworkProbe {
  const probe: LateSubmissionNetworkProbe = {
    console: [],
    request: [],
    response: [],
    requestfailed: [],
  }
  const interesting = (url: string) => url.includes('/api/v1/') || url.includes('late-submission') || url.includes('/submit')
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

export function formatLateSubmissionNetworkProbe(probe: LateSubmissionNetworkProbe): string {
  return [
    `console=${JSON.stringify(probe.console.filter((line) => /error|fail|cors|late|unavailable/i.test(line)))}`,
    `request=${JSON.stringify(probe.request)}`,
    `response=${JSON.stringify(probe.response)}`,
    `requestfailed=${JSON.stringify(probe.requestfailed)}`,
  ].join('\n')
}

export function requireLateSubmissionEnv(name: string): string {
  const value = (process.env[name] || '').trim()
  if (!value) {
    throw new Error(`${name} is required for the late submission browser gate`)
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

export function lateRequestBody(overrides: Record<string, unknown> = {}) {
  return {
    id: requestId,
    rfx_event_id: eventId,
    carrier_company_id: carrierCompanyId,
    reason_code: 'TECHNICAL_FAILURE',
    reason_text: 'studio outage',
    requested_until: '2026-09-21T12:00:00Z',
    status: 'REQUESTED',
    version: 1,
    created_at: '2026-09-20T10:00:00Z',
    updated_at: '2026-09-20T10:00:00Z',
    ...overrides,
  }
}

export function workspaceBody(overrides: Record<string, unknown> = {}) {
  return {
    id: responseId,
    tenant_id: tenantId,
    rfx_event_id: eventId,
    participant_company_id: carrierCompanyId,
    status: 'DRAFT',
    product_status: 'IN_PROGRESS',
    save_version: 1,
    completion_percent: 100,
    questionnaire: {
      event_id: eventId,
      rfx_version_id: '44444444-4444-4444-8444-444444444444',
      version_number: 1,
      questionnaire_enabled: true,
      version_status: 'PUBLISHED',
      sections: [{
        section: {
          id: 'sec-1',
          section_code: 'MAIN',
          title: 'Main',
          sort_order: 1,
          version: 1,
        },
        questions: [],
      }],
      rules: [],
    },
    answers: [],
    ...overrides,
  }
}

export async function seedCarrierSession(
  page: Page,
  options: { roles?: string[]; lateEnabled?: boolean; token?: string } = {},
) {
  await page.addInitScript((input) => {
    localStorage.setItem(
      'freight_procurement_session',
      JSON.stringify({
        token: input.token,
        user: {
          id: input.user,
          tenant_id: input.tenant,
          email: 'carrier-late-e2e@freight.test',
          full_name: 'Carrier Late E2E',
          preferred_locale: 'en-US',
          status: 'ACTIVE',
          roles: input.roles,
        },
      }),
    )
    localStorage.setItem('freight_procurement_tenant_id', input.tenant)
    localStorage.setItem('freight_procurement_company_id', input.company)
    if (input.lateEnabled) {
      localStorage.setItem('freight_procurement_rfx_late_submission', 'true')
    }
    document.cookie = 'freight_procurement_locale=en-US; path=/'
  }, {
    tenant: tenantId,
    company: carrierCompanyId,
    user: userId,
    roles: options.roles ?? ['CARRIER_DISPATCHER'],
    lateEnabled: options.lateEnabled !== false,
    token: options.token ?? 'browser-e2e-carrier-late',
  })
}

export async function seedBuyerSession(
  page: Page,
  options: { roles?: string[]; lateEnabled?: boolean; token?: string } = {},
) {
  await page.addInitScript((input) => {
    localStorage.setItem(
      'freight_procurement_session',
      JSON.stringify({
        token: input.token,
        user: {
          id: input.user,
          tenant_id: input.tenant,
          email: 'buyer-late-e2e@freight.test',
          full_name: 'Buyer Late E2E',
          preferred_locale: 'en-US',
          status: 'ACTIVE',
          roles: input.roles,
        },
      }),
    )
    localStorage.setItem('freight_procurement_tenant_id', input.tenant)
    localStorage.setItem('freight_procurement_company_id', input.company)
    if (input.lateEnabled) {
      localStorage.setItem('freight_procurement_rfx_late_submission', 'true')
    }
    document.cookie = 'freight_procurement_locale=en-US; path=/'
  }, {
    tenant: tenantId,
    company: buyerCompanyId,
    user: userId,
    roles: options.roles ?? ['PROCUREMENT_MANAGER'],
    lateEnabled: options.lateEnabled !== false,
    token: options.token ?? 'browser-e2e-buyer-late',
  })
}

export async function stubCarrierMemberships(
  page: Page,
  role = 'CARRIER_DISPATCHER',
  companies: { carrierCompanyId?: string; buyerCompanyId?: string } = {},
) {
  const carrierId = companies.carrierCompanyId || carrierCompanyId
  const buyerId = companies.buyerCompanyId || buyerCompanyId
  await page.route('**/api/v1/users/**/companies**', async (route) => {
    await fulfillJSON(route, 200, {
      items: [{
        membership_id: `${carrierId}-membership`,
        company_id: carrierId,
        legal_name: 'Carrier A',
        company_type: 'CARRIER',
        membership_status: 'ACTIVE',
        roles: [{ code: role, name: role }],
      }],
    })
  })
  await page.route('**/api/v1/companies**', async (route) => {
    const url = new URL(route.request().url())
    if (url.pathname !== '/api/v1/companies' && !url.pathname.endsWith('/companies')) {
      await route.fallback()
      return
    }
    await fulfillJSON(route, 200, {
      items: [
        { id: buyerId, legal_name: 'Buyer A', company_type: 'SHIPPER', status: 'ACTIVE' },
        { id: carrierId, legal_name: 'Carrier A', company_type: 'CARRIER', status: 'ACTIVE' },
      ],
    })
  })
}

export async function stubCarrierTenderShell(
  page: Page,
  options: { deadline?: string; responseStatus?: 'DRAFT' | 'MISSING' } = {},
) {
  const deadline = options.deadline ?? '2020-01-01T00:00:00Z'
  const responseStatus = options.responseStatus ?? 'DRAFT'
  await stubCarrierMemberships(page)
  await page.route(`**/api/v1/carrier/rfx-events/${eventId}**`, async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback()
      return
    }
    await fulfillJSON(route, 200, {
      id: eventId,
      tenant_id: tenantId,
      owner_company_id: buyerCompanyId,
      rfx_number: 'RFX-LATE-1',
      title: 'Late submission draft',
      status: 'PUBLISHED',
      rfx_type: 'SPOT_RFQ',
      category: 'FREIGHT',
      response_deadline: deadline,
      participant_status: 'INVITED',
      own_response_status: responseStatus === 'MISSING' ? 'NOT_STARTED' : responseStatus,
      own_response_id: responseStatus === 'MISSING' ? null : responseId,
      lot_count: 0,
      participant_company_id: carrierCompanyId,
    })
  })
  await page.route(`**/api/v1/rfx-events/${eventId}/own-participant**`, async (route) => {
    await fulfillJSON(route, 200, {
      id: `${eventId}-participant`,
      company_id: carrierCompanyId,
      participant_type: 'CARRIER',
      status: 'INVITED',
    })
  })
  await page.route(`**/api/v1/rfx-events/${eventId}/lots**`, async (route) => {
    await fulfillJSON(route, 200, { items: [] })
  })
  await page.route(`**/api/v1/rfx-events/${eventId}/own-response**`, async (route) => {
    if (responseStatus === 'MISSING') {
      await fulfillJSON(route, 404, {
        error: { code: 'NOT_FOUND', message: 'own response not found', details: {} },
      })
      return
    }
    await fulfillJSON(route, 200, {
      id: responseId,
      tenant_id: tenantId,
      rfx_event_id: eventId,
      participant_company_id: carrierCompanyId,
      status: 'DRAFT',
      offer_lines: [],
    })
  })
  await page.route(`**/api/v1/rfx-events/${eventId}/own-award**`, async (route) => {
    await fulfillJSON(route, 404, {
      error: { code: 'NOT_FOUND', message: 'award not found', details: {} },
    })
  })
}

export async function stubBuyerTenderShell(page: Page) {
  await page.route('**/api/v1/companies**', async (route) => {
    const url = new URL(route.request().url())
    if (url.pathname !== '/api/v1/companies' && !url.pathname.endsWith('/companies')) {
      await route.fallback()
      return
    }
    await fulfillJSON(route, 200, {
      items: [
        { id: buyerCompanyId, legal_name: 'Buyer A', company_type: 'SHIPPER', status: 'ACTIVE' },
        { id: carrierCompanyId, legal_name: 'Carrier A', company_type: 'CARRIER', status: 'ACTIVE' },
      ],
    })
  })
  await page.route(`**/api/v1/rfx-events/${eventId}**`, async (route) => {
    const url = new URL(route.request().url())
    const method = route.request().method()
    if (method === 'GET' && url.pathname === `/api/v1/rfx-events/${eventId}`) {
      await fulfillJSON(route, 200, {
        id: eventId,
        tenant_id: tenantId,
        owner_company_id: buyerCompanyId,
        rfx_number: 'RFX-LATE-1',
        title: 'Late submission draft',
        status: 'PUBLISHED',
        rfx_type: 'SPOT_RFQ',
        category: 'FREIGHT',
        response_deadline: '2020-01-01T00:00:00Z',
      })
      return
    }
    if (method === 'GET' && url.pathname.endsWith('/lots')) {
      await fulfillJSON(route, 200, { items: [] })
      return
    }
    if (method === 'GET' && url.pathname.endsWith('/participants')) {
      await fulfillJSON(route, 200, { items: [] })
      return
    }
    await route.fallback()
  })
}

export async function stubQuestionnaireWorkspace(page: Page, deadline = '2020-01-01T00:00:00Z') {
  await stubCarrierTenderShell(page, { deadline })
  await page.route(`**/api/v1/rfx-events/${eventId}/carrier-response**`, async (route) => {
    const url = new URL(route.request().url())
    if (url.pathname.endsWith('/validate')) {
      await fulfillJSON(route, 200, {
        valid: true,
        blocking_error_count: 0,
        completion_percent: 100,
        errors: [],
      })
      return
    }
    if (url.pathname.endsWith('/submit')) {
      await route.fallback()
      return
    }
    if (route.request().method() === 'GET' && url.pathname.endsWith('/carrier-response')) {
      await fulfillJSON(route, 200, workspaceBody())
      return
    }
    await route.fallback()
  })
}

export async function expectCarrierWorkspaceLoaded(page: Page) {
  await expect(page.getByRole('button', { name: 'Sign out' })).toBeVisible()
  await expect(page.getByTestId('carrier-tender-rfx-number')).toHaveText('RFX-LATE-1', { timeout: 30_000 })
}

export async function expectBuyerWorkspaceLoaded(page: Page) {
  await expect(page.getByRole('button', { name: 'Sign out' })).toBeVisible()
  await expect(page.getByTestId('tender-rfx-number')).toHaveText('RFX-LATE-1', { timeout: 30_000 })
}
