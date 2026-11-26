import { type Page } from '@playwright/test'
import {
  buyerCompanyId,
  carrierCompanyId,
  eventId,
  fulfillJSON,
  seedBuyerSession,
  tenantId,
} from '../late-submission/helpers'

export const responseId = '33333333-3333-4333-8333-333333333333'

export async function stubEvaluationWorkspace(page: Page) {
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
        rfx_number: 'RFX-AWARD-ROLE',
        title: 'Award role gate',
        status: 'EVALUATION',
        rfx_type: 'SPOT_RFQ',
        category: 'FREIGHT',
        response_deadline: '2030-01-01T00:00:00Z',
      })
      return
    }
    if (method === 'GET' && url.pathname.endsWith('/responses')) {
      await fulfillJSON(route, 200, {
        items: [{
          id: responseId,
          participant_company_id: carrierCompanyId,
          status: 'RESPONSE_SUBMITTED',
          offer_complete: true,
          awarded: false,
          comparable: true,
          total_amount: 1000,
          currency_code: 'RUB',
          offer_lines: [{ amount: 1000, currency_code: 'RUB' }],
        }],
      })
      return
    }
    if (method === 'GET' && (url.pathname.endsWith('/score-model') || url.pathname.includes('/score-models'))) {
      await fulfillJSON(route, 404, { error: { code: 'NOT_FOUND', message: 'not found', details: {} } })
      return
    }
    if (method === 'GET' && url.pathname.endsWith('/audit-events')) {
      await fulfillJSON(route, 200, { items: [] })
      return
    }
    await route.fallback()
  })
}

export async function seedCarrierSession(page: Page) {
  await page.addInitScript((input) => {
    localStorage.setItem(
      'freight_procurement_session',
      JSON.stringify({
        token: input.token,
        user: {
          id: input.user,
          tenant_id: input.tenant,
          email: 'carrier-award-ui@freight.test',
          full_name: 'Carrier User',
          preferred_locale: 'en-US',
          status: 'ACTIVE',
          roles: input.roles,
        },
      }),
    )
    localStorage.setItem('freight_procurement_tenant_id', input.tenant)
    localStorage.setItem('freight_procurement_company_id', input.carrier)
    document.cookie = 'freight_procurement_locale=en-US; path=/'
  }, {
    tenant: tenantId,
    carrier: carrierCompanyId,
    user: '99999999-9999-4999-8999-999999999999',
    roles: ['CARRIER_DISPATCHER'],
    token: 'browser-e2e-carrier-award-ui',
  })
}

export { eventId, seedBuyerSession }
