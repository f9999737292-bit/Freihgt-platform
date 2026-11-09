import { expect, test, type Page, type Route } from '@playwright/test'

/**
 * Condensed operator walkthrough (0.4B). Full security matrix remains in carrier-office.spec.ts.
 */

async function installPilotGateway(page: Page, options: { tracking?: 'ready' | 'not_found' } = {}) {
  const calls: string[] = []
  await page.route('**/api/v1/**', async (route: Route) => {
    const request = route.request()
    const headers = request.headers()
    expect(headers['x-tenant-id']).toBeUndefined()
    const url = new URL(request.url())
    calls.push(`${request.method()} ${url.pathname}${url.search}`)
    if (url.pathname === '/api/v1/auth/login') {
      await route.fulfill({
        json: {
          access_token: 'tab-token',
          token_type: 'Bearer',
          expires_in: 3600,
          user: {
            id: 'user-1',
            tenant_id: 'tenant-1',
            email: 'carrier@example.com',
            full_name: 'Carrier User',
            roles: ['CARRIER_DISPATCHER'],
          },
        },
      })
      return
    }
    if (url.pathname === '/api/v1/users/user-1/companies') {
      await route.fulfill({
        json: {
          items: [{
            membership_id: 'm-1',
            company_id: 'co-1',
            legal_name: 'Carrier Co',
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
      await route.fulfill({
        json: {
          transport_order_id: 'order-1',
          transport_order_number: 'TO-1',
          transport_order_status: 'CONFIRMED',
          shipment: { id: 'shipment-A', status: 'IN_TRANSIT' },
        },
      })
      return
    }
    const tracking = url.pathname.match(/^\/api\/v1\/carrier\/shipments\/([^/]+)\/tracking$/)
    if (tracking) {
      expect(url.searchParams.get('carrier_company_id')).toBe('co-1')
      if (options.tracking === 'not_found' || tracking[1] === 'shipment-b') {
        await route.fulfill({ status: 404, json: { message: 'not found' } })
        return
      }
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
    if (url.pathname === '/api/v1/carrier/documents' && url.searchParams.get('shipment_id') === 'shipment-A') {
      await route.fulfill({
        json: {
          items: [{
            id: 'doc-1',
            document_number: 'DOC-1',
            document_type: 'POD',
            document_status: 'SIGNED',
          }],
          total: 1,
          limit: 20,
          offset: 0,
        },
      })
      return
    }
    if (url.pathname.startsWith('/api/v1/carrier/shipments/shipment-A/')) {
      const suffix = url.pathname.replace('/api/v1/carrier/shipments/shipment-A', '')
      if (suffix === '/eta') {
        await route.fulfill({ json: { pickup: { estimatedArrivalAt: '2026-12-01T11:00:00Z' }, delivery: null } })
        return
      }
      if (suffix === '/slots') {
        await route.fulfill({ json: { pickup: { timezone: 'Europe/Moscow' }, delivery: null } })
        return
      }
    }
    await route.fulfill({ status: 404, json: { message: url.pathname } })
  })
  return calls
}

async function signIn(page: Page) {
  await page.goto('/login')
  await page.getByTestId('tenant-id').fill('tenant-1')
  await page.getByTestId('email').fill('carrier@example.com')
  await page.getByTestId('password').fill('secret')
  await page.getByTestId('sign-in').click()
}

async function chooseCompany(page: Page) {
  await expect(page.getByTestId('company-gate')).toBeVisible()
  await page.getByTestId('company-select').selectOption('co-1')
  await page.getByTestId('company-continue').click()
  await expect(page.getByTestId('home-shell')).toBeVisible()
}

test('pilot walkthrough: carrier office happy path in Russian UI', async ({ page }) => {
  await installPilotGateway(page)
  await signIn(page)
  await expect(page.getByText('Кабинет перевозчика')).toBeVisible()
  await chooseCompany(page)

  await page.getByRole('link', { name: 'Заявки' }).click()
  await expect(page.getByText('TO-1')).toBeVisible()
  await page.getByTestId('open-order').click()
  await expect(page.getByTestId('order-status')).toContainText('CONFIRMED')
  await page.getByTestId('open-tracking').click()
  await expect(page.getByTestId('tracking-status')).toHaveText('ACTIVE')
  await expect(page.getByTestId('eta-arrival')).toHaveText('2026-12-01T11:00:00Z')

  await page.getByRole('link', { name: 'Заявки' }).click()
  await page.locator('tr', { hasText: 'TO-1' }).getByTestId('open-documents').click()
  await expect(page.getByTestId('document-number')).toHaveText('DOC-1')
})

test('pilot walkthrough: cross-company tracking is denied for selected company', async ({ page }) => {
  const calls = await installPilotGateway(page, { tracking: 'not_found' })
  await signIn(page)
  await chooseCompany(page)
  await page.goto('/shipments/shipment-b/tracking')
  await expect(page.getByTestId('tracking-section')).toContainText('Отслеживание недоступно для выбранной компании')
  const trackingCalls = calls.filter((call) => call.includes('/api/v1/carrier/shipments/shipment-b/'))
  expect(trackingCalls.length).toBeGreaterThan(0)
  expect(trackingCalls.every((call) => call.includes('carrier_company_id=co-1'))).toBe(true)
  expect(trackingCalls.some((call) => call.includes('carrier_company_id=co-spoof'))).toBe(false)
})
