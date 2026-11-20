import { expect, test, type Page, type Route } from '@playwright/test'

/**
 * 0.5A core pilot path — MOCK ONLY (routed gateway). Not live staging E2E.
 * Covers carrier leg: tender respond → award read → transport order → tracking → documents list.
 * Buyer award/publish remains in web-procurement (see CP05A-B1/B2).
 */

function loginPayload(roles: string[]) {
  return {
    access_token: 'tab-token',
    token_type: 'Bearer',
    expires_in: 3600,
    user: {
      id: 'user-1',
      tenant_id: 'tenant-1',
      email: 'carrier@example.com',
      full_name: 'Carrier User',
      roles,
    },
  }
}

async function installCorePilotMock(page: Page) {
  let responseReady = false
  await page.route('**/api/v1/**', async (route: Route) => {
    const request = route.request()
    const headers = request.headers()
    expect(headers['x-tenant-id']).toBeUndefined()
    const url = new URL(request.url())
    if (url.pathname === '/api/v1/auth/login') {
      await route.fulfill({ json: loginPayload(['CARRIER_DISPATCHER']) })
      return
    }
    if (url.pathname === '/api/v1/users/user-1/companies') {
      await route.fulfill({
        json: {
          items: [{
            membership_id: 'm-1',
            company_id: 'co-1',
            legal_name: 'Carrier Co',
            company_type: 'CARRIER',
            membership_status: 'ACTIVE',
            roles: [{ code: 'CARRIER_DISPATCHER' }],
          }],
        },
      })
      return
    }
    if (headers['x-company-id'] && headers['x-company-id'] !== 'co-1') {
      await route.fulfill({ status: 403, json: { message: 'forbidden' } })
      return
    }
    if (url.pathname === '/api/v1/carrier/rfx-events') {
      await route.fulfill({
        json: {
          items: [{
            id: 'event-1',
            rfx_number: 'RFX-1',
            title: 'North lane',
            status: 'RESPONSES_OPEN',
            response_deadline: '2026-12-01T00:00:00Z',
          }],
        },
      })
      return
    }
    if (url.pathname === '/api/v1/carrier/rfx-events/event-1') {
      await route.fulfill({
        json: {
          id: 'event-1',
          rfx_number: 'RFX-1',
          title: 'North lane',
          status: 'RESPONSES_OPEN',
          response_deadline: '2026-12-01T00:00:00Z',
          participant_status: 'INVITED',
        },
      })
      return
    }
    if (url.pathname === '/api/v1/rfx-events/event-1/own-response') {
      if (!responseReady) {
        await route.fulfill({ status: 404, json: { message: 'not found' } })
        return
      }
      await route.fulfill({
        json: { id: 'resp-1', status: 'SUBMITTED', offer_lines: [{ amount: 1500, currency_code: 'RUB' }] },
      })
      return
    }
    if (url.pathname === '/api/v1/rfx-events/event-1/responses' && request.method() === 'POST') {
      responseReady = true
      await route.fulfill({ json: { id: 'resp-1', status: 'DRAFT', offer_lines: [] } })
      return
    }
    if (url.pathname === '/api/v1/rfx-responses/resp-1' && request.method() === 'PATCH') {
      await route.fulfill({
        json: { id: 'resp-1', status: 'DRAFT', offer_lines: [{ amount: 1500, currency_code: 'RUB' }] },
      })
      return
    }
    if (url.pathname === '/api/v1/rfx-responses/resp-1/submit' && request.method() === 'POST') {
      responseReady = true
      await route.fulfill({ json: { id: 'resp-1', status: 'SUBMITTED' } })
      return
    }
    if (url.pathname === '/api/v1/rfx-events/event-1/own-award') {
      if (!responseReady) {
        await route.fulfill({ status: 404, json: { message: 'not found' } })
        return
      }
      await route.fulfill({ json: { id: 'award-1', total_amount: 1500, currency_code: 'RUB' } })
      return
    }
    if (url.pathname === '/api/v1/carrier/transport-orders') {
      expect(url.searchParams.get('carrier_company_id')).toBe('co-1')
      await route.fulfill({
        json: {
          items: [{
            transport_order_id: 'order-1',
            transport_order_number: 'TO-1',
            transport_order_status: 'CONFIRMED',
            shipment_id: 'shipment-A',
            shipment_status: 'IN_TRANSIT',
          }],
        },
      })
      return
    }
    if (url.pathname === '/api/v1/order-execution/transport-orders/order-1') {
      expect(url.searchParams.get('company_id')).toBe('co-1')
      expect(url.searchParams.get('actor')).toBe('CARRIER')
      await route.fulfill({
        json: {
          transport_order_id: 'order-1',
          transport_order_number: 'TO-1',
          transport_order_status: 'CONFIRMED',
          shipment: { id: 'shipment-A', status: 'IN_TRANSIT' },
          provenance: { amount: 1500, currency_code: 'RUB' },
        },
      })
      return
    }
    if (url.pathname === '/api/v1/carrier/shipments/shipment-A/tracking') {
      expect(url.searchParams.get('carrier_company_id')).toBe('co-1')
      await route.fulfill({
        json: {
          trackingStatus: 'ACTIVE',
          freshness: { status: 'fresh', ageSeconds: 12 },
          quality: { status: 'good' },
        },
      })
      return
    }
    if (url.pathname === '/api/v1/carrier/documents' && url.searchParams.get('shipment_id') === 'shipment-A') {
      expect(url.searchParams.get('carrier_company_id')).toBe('co-1')
      await route.fulfill({
        json: {
          items: [{ id: 'doc-1', document_number: 'DOC-1', document_type: 'POD', document_status: 'SIGNED' }],
          total: 1,
          limit: 20,
          offset: 0,
        },
      })
      return
    }
    await route.fulfill({ status: 404, json: { message: url.pathname } })
  })
}

async function signInAndChooseCompany(page: Page) {
  await page.goto('/login')
  await page.getByTestId('tenant-id').fill('tenant-1')
  await page.getByTestId('email').fill('carrier@example.com')
  await page.getByTestId('password').fill('secret')
  await page.getByTestId('sign-in').click()
  await page.getByTestId('company-select').selectOption('co-1')
  await page.getByTestId('company-continue').click()
  await expect(page.getByTestId('home-shell')).toBeVisible()
}

test('mock core pilot: carrier sourcing through execution surfaces (not live E2E)', async ({ page }) => {
  await installCorePilotMock(page)
  await signInAndChooseCompany(page)

  await page.getByRole('link', { name: 'Тендеры' }).click()
  await page.getByTestId('open-tender').click()
  await page.getByTestId('create-response').click()
  await page.getByTestId('offer-amount').fill('1500')
  await page.getByTestId('save-offer').click()
  await page.getByTestId('submit-response').click()
  await expect(page.getByTestId('award-amount')).toContainText('1500')

  await page.getByRole('link', { name: 'Заявки' }).click()
  await page.getByTestId('open-order').click()
  await expect(page.getByTestId('order-status')).toContainText('CONFIRMED')
  await page.getByTestId('open-tracking').click()
  await expect(page.getByTestId('tracking-status')).toHaveText('ACTIVE')

  await page.getByRole('link', { name: 'Заявки' }).click()
  await page.locator('tr', { hasText: 'TO-1' }).getByTestId('open-documents').click()
  await expect(page.getByTestId('document-number')).toHaveText('DOC-1')
})
