import { expect, type Page, type Route } from '@playwright/test'

export const eventId = '11111111-1111-4111-8111-111111111111'
export const analysisId = '33333333-3333-4333-8333-333333333333'

export async function seedBuyerSession(page: Page, roles = ['PROCUREMENT_MANAGER']) {
  await page.addInitScript((input) => {
    localStorage.setItem(
      'freight_procurement_session',
      JSON.stringify({
        token: 'browser-e2e-buyer-xlsx',
        user: {
          id: '8541a3a3-bde7-4fed-9501-37b9953bf904',
          tenant_id: input.tenant,
          email: 'buyer-xlsx-e2e@freight.test',
          full_name: 'Buyer XLSX E2E',
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

export async function stubTenderWorkspace(page: Page, status = 'DRAFT') {
  await page.route('**/api/v1/rfx-events/**', async (route: Route) => {
    const url = new URL(route.request().url())
    const method = route.request().method()
    if (method === 'GET' && url.pathname === `/api/v1/rfx-events/${eventId}`) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          id: eventId,
          tenant_id: 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa',
          owner_company_id: 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb',
          rfx_number: 'RFX-XLSX-1',
          title: 'Buyer XLSX draft',
          status,
          rfx_type: 'LANE_TENDER',
          category: 'FREIGHT',
          response_deadline: '2026-10-01T12:00:00Z',
        }),
      })
      return
    }
    if (method === 'GET' && url.pathname.endsWith('/lots')) {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([]) })
      return
    }
    if (method === 'GET' && url.pathname.endsWith('/participants')) {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ items: [] }) })
      return
    }
    await route.continue()
  })
}

export function readyPreviewBody() {
  return {
    schema_name: 'BINTRANS_RFX_BUYER_XLSX_V1',
    schema_version: '1',
    mode: 'UPDATE_DRAFT',
    target_event_id: eventId,
    target_draft_version_id: '22222222-2222-4222-8222-222222222222',
    target_version_number: 1,
    target_event_row_version: 1,
    target_draft_row_version: 1,
    analysis_id: analysisId,
    expires_at: '2026-09-21T00:00:00Z',
    ready_to_commit: true,
    summary: { errors: 0, warnings: 0 },
    questionnaire_diff: {},
    lots_diff: {},
    errors: [],
    warnings: [],
  }
}

export async function expectPanelVisible(page: Page) {
  await expect(page.getByTestId('buyer-xlsx-panel')).toBeVisible({ timeout: 30_000 })
}
