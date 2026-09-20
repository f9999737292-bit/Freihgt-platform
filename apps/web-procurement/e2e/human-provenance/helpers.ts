import { expect, type Page, type Route } from '@playwright/test'

export const eventId = '11111111-1111-4111-8111-111111111111'

const corsHeaders = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Headers':
    'Accept, Authorization, Content-Type, X-Tenant-ID, X-Company-ID, X-User-ID, X-Request-ID, X-Locale, Idempotency-Key',
  'Access-Control-Allow-Methods': 'GET, POST, PATCH, PUT, DELETE, OPTIONS',
}

export interface HumanProvenanceNetworkProbe {
  request: string[]
  response: string[]
}

export function attachHumanProvenanceNetworkProbe(page: Page): HumanProvenanceNetworkProbe {
  const probe: HumanProvenanceNetworkProbe = { request: [], response: [] }
  page.on('request', (req) => {
    probe.request.push(`${req.method()} ${req.url()}`)
  })
  page.on('response', (resp) => {
    probe.response.push(`${resp.request().method()} ${resp.status()} ${resp.url()}`)
  })
  return probe
}

export function formatHumanProvenanceNetworkProbe(probe: HumanProvenanceNetworkProbe): string {
  return `request=${JSON.stringify(probe.request)}\nresponse=${JSON.stringify(probe.response)}`
}

export function requireHumanProvenanceEnv(name: string): string {
  const value = (process.env[name] || '').trim()
  if (!value) {
    throw new Error(`${name} is required for the human provenance browser gate`)
  }
  return value
}

export function assertNoErpIntegrationCalls(probe: HumanProvenanceNetworkProbe) {
  const leaked = [...probe.request, ...probe.response].filter((line) => line.includes('/integrations/erp/'))
  expect(leaked, formatHumanProvenanceNetworkProbe(probe)).toEqual([])
}

async function fulfillJSON(route: Route, status: number, body: unknown) {
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

export async function seedBuyerSession(page: Page, roles = ['PROCUREMENT_MANAGER']) {
  await page.addInitScript((input) => {
    localStorage.setItem(
      'freight_procurement_session',
      JSON.stringify({
        token: 'browser-e2e-human-provenance',
        user: {
          id: '8541a3a3-bde7-4fed-9501-37b9953bf904',
          tenant_id: input.tenant,
          email: 'buyer-f4-e2e@freight.test',
          full_name: 'Buyer F4 E2E',
          preferred_locale: 'en-US',
          status: 'ACTIVE',
          roles: input.roles,
        },
      }),
    )
    localStorage.setItem('freight_procurement_tenant_id', input.tenant)
    localStorage.setItem('freight_procurement_company_id', input.company)
    document.cookie = 'freight_procurement_locale=en-US; path=/'
  }, {
    tenant: 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa',
    company: 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb',
    roles,
  })
}

export async function stubTenderWorkspace(
  page: Page,
  options: { status?: number; body?: Record<string, unknown> } = {},
) {
  await page.route('**/api/v1/**', async (route: Route) => {
    if (route.request().method() === 'OPTIONS') {
      await route.fulfill({ status: 204, headers: corsHeaders })
      return
    }
    const url = new URL(route.request().url())
    const method = route.request().method()
    if (method === 'GET' && url.pathname === `/api/v1/rfx-events/${eventId}`) {
      await fulfillJSON(route, options.status ?? 200, options.body ?? {
        id: eventId,
        tenant_id: 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa',
        owner_company_id: 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb',
        rfx_number: 'RFX-F4-1',
        title: 'Human provenance draft',
        status: 'DRAFT',
        rfx_type: 'LANE_TENDER',
        category: 'FREIGHT',
        creation_channel: 'MANUAL',
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
    await fulfillJSON(route, 404, {
      error: { code: 'NOT_FOUND', message: 'not stubbed', details: {} },
    })
  })
}

export async function expectWorkspaceLoaded(page: Page, rfxNumber: string) {
  await expect(page.getByRole('button', { name: 'Sign out' })).toBeVisible()
  await expect(page.getByTestId('tender-rfx-number')).toHaveText(rfxNumber, { timeout: 30_000 })
}
