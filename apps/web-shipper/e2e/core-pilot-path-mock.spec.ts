import { expect, test, type Page, type Route } from '@playwright/test'

/**
 * 0.5A core pilot path — MOCK ONLY (routed gateway). Not live staging E2E.
 * Covers shipper execution leg after award exists; buyer RFx/award is out of web-shipper.
 */

const shipment = {
  id: 'shp-1',
  shipment_number: 'SHP-1',
  status: 'BOOKED',
  transport_order_id: 'to-1',
  shipper_company_id: 'co-shipper',
  consignee_company_id: 'co-consignee',
  carrier_company_id: 'co-carrier',
  transport_mode: 'ROAD',
  planned_pickup_at: '2026-12-01T08:00:00Z',
  planned_delivery_at: '2026-12-02T18:00:00Z',
  actual_delivery_at: null,
}

async function installMockGateway(page: Page, roles: string[]) {
  await page.route('**/api/v1/**', async (route: Route) => {
    const request = route.request()
    const headers = request.headers()
    expect(headers['x-tenant-id']).toBeUndefined()
    const url = new URL(request.url())
    if (url.pathname === '/api/v1/auth/login') {
      await route.fulfill({
        json: {
          access_token: 'tab-token',
          token_type: 'Bearer',
          expires_in: 3600,
          user: {
            id: 'user-1',
            tenant_id: 'tenant-1',
            email: 'shipper@example.com',
            full_name: 'Shipper User',
            roles,
          },
        },
      })
      return
    }
    if (url.pathname === '/api/v1/users/user-1/companies') {
      expect(url.searchParams.get('tenant_id')).toBe('tenant-1')
      await route.fulfill({
        json: {
          items: [{
            membership_id: 'm-1',
            company_id: 'co-shipper',
            legal_name: 'Shipper Co',
            company_type: 'SHIPPER',
            membership_status: 'ACTIVE',
            roles: [{ code: roles[0] ?? 'SHIPPER_ADMIN' }],
          }],
        },
      })
      return
    }
    if (url.pathname === '/api/v1/shipper/shipments') {
      expect(url.searchParams.get('shipper_company_id')).toBe('co-shipper')
      expect(headers['x-company-id']).toBe('co-shipper')
      await route.fulfill({ json: { items: [shipment], total: 1 } })
      return
    }
    if (url.pathname === '/api/v1/shipper/shipments/shp-1') {
      await route.fulfill({ json: shipment })
      return
    }
    if (url.pathname === '/api/v1/shipper/shipments/shp-1/tracking') {
      await route.fulfill({
        json: {
          trackingStatus: 'ACTIVE',
          freshness: { status: 'fresh', ageSeconds: 12 },
          quality: { status: 'good' },
          lastKnownPosition: { latitude: 55.75, longitude: 37.62, recordedAt: '2026-12-01T10:00:00Z', ageSeconds: 12 },
        },
      })
      return
    }
    if (url.pathname === '/api/v1/shipper/shipments/shp-1/eta') {
      await route.fulfill({
        json: {
          pickup: { status: 'AVAILABLE', estimatedArrivalAt: '2026-12-01T11:00:00Z' },
          delivery: null,
        },
      })
      return
    }
    if (url.pathname === '/api/v1/shipper/shipments/shp-1/slots') {
      await route.fulfill({
        json: {
          pickup: {
            windowStatus: 'OPEN',
            windowStart: '2026-12-01T08:00:00Z',
            windowEnd: '2026-12-01T10:00:00Z',
            timezone: 'Europe/Moscow',
          },
          delivery: null,
        },
      })
      return
    }
    if (url.pathname === '/api/v1/shipper/documents') {
      expect(url.searchParams.get('shipper_company_id')).toBe('co-shipper')
      await route.fulfill({
        json: {
          items: [{
            id: 'doc-1',
            document_number: 'DOC-1',
            document_type: 'POD',
            document_status: 'SIGNED',
            related_entity_id: 'shp-1',
          }],
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

async function signIn(page: Page) {
  await page.goto('/login')
  await page.getByTestId('tenant-id').fill('tenant-1')
  await page.getByTestId('email').fill('shipper@example.com')
  await page.getByTestId('password').fill('secret')
  await page.getByTestId('sign-in').click()
}

test('mock core pilot: shipper execution leg after award (not live E2E)', async ({ page }) => {
  await installMockGateway(page, ['SHIPPER_ADMIN'])
  await signIn(page)
  await page.getByTestId('company-select').selectOption('co-shipper')
  await page.getByTestId('company-continue').click()
  await expect(page.getByTestId('home-shell')).toBeVisible()

  await page.getByRole('link', { name: 'Отправления' }).click()
  await page.getByTestId('open-shipment').click()
  await page.getByTestId('open-tracking').click()
  await expect(page.getByTestId('tracking-status')).toContainText('ACTIVE')

  await page.getByRole('link', { name: 'Отправления' }).click()
  await page.getByTestId('open-shipment').click()
  await page.getByTestId('open-documents').click()
  await expect(page.getByTestId('document-row')).toContainText('DOC-1')
})

test('mock core pilot: web-shipper has no RFx navigation', async ({ page }) => {
  await installMockGateway(page, ['SHIPPER_ADMIN'])
  await signIn(page)
  await page.getByTestId('company-select').selectOption('co-shipper')
  await page.getByTestId('company-continue').click()
  await expect(page.getByRole('link', { name: 'Тендеры' })).toHaveCount(0)
  await expect(page.getByRole('link', { name: 'Заявки' })).toHaveCount(0)
})

test('mock core pilot: procurement manager cannot enter shipper office', async ({ page }) => {
  await page.route('**/api/v1/**', async (route: Route) => {
    const request = route.request()
    const url = new URL(request.url())
    if (url.pathname === '/api/v1/auth/login') {
      await route.fulfill({
        json: {
          access_token: 'tab-token',
          token_type: 'Bearer',
          expires_in: 3600,
          user: {
            id: 'user-1',
            tenant_id: 'tenant-1',
            email: 'procurement@example.com',
            full_name: 'Procurement User',
            roles: ['PROCUREMENT_MANAGER'],
          },
        },
      })
      return
    }
    if (url.pathname === '/api/v1/users/user-1/companies') {
      await route.fulfill({
        json: {
          items: [{
            membership_id: 'm-p',
            company_id: 'co-shipper',
            legal_name: 'Shipper Co',
            company_type: 'SHIPPER',
            membership_status: 'ACTIVE',
            roles: [{ code: 'PROCUREMENT_MANAGER' }],
          }],
        },
      })
      return
    }
    await route.fulfill({ status: 404, json: { message: url.pathname } })
  })
  await page.goto('/login')
  await page.getByTestId('tenant-id').fill('tenant-1')
  await page.getByTestId('email').fill('procurement@example.com')
  await page.getByTestId('password').fill('secret')
  await page.getByTestId('sign-in').click()
  await expect(page.getByText('Эта роль не входит в кабинет грузоотправителя.')).toBeVisible()
})
