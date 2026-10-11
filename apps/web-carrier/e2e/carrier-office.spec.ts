import { expect, test, type Page, type Route } from '@playwright/test'

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

type GatewayOptions = {
  orders?: Array<Record<string, unknown>>
  orderDetail?: Record<string, unknown>
  tracking?: 'ready' | 'not_found' | 'tracking_unavailable' | 'tracking_forbidden'
}

function trackingBody(suffix: string, url: URL) {
  if (suffix === '/tracking') {
    return {
      trackingStatus: 'ACTIVE',
      freshness: { status: 'fresh', ageSeconds: 12 },
      quality: { status: 'good' },
      lastKnownPosition: { latitude: 55.75, longitude: 37.62, recordedAt: '2026-12-01T10:00:00Z', ageSeconds: 12 },
      providerDeviceId: 'provider-device-secret',
      providerEventId: 'provider-event-secret',
    }
  }
  if (suffix === '/tracking/locations') {
    return {
      items: [{
        recordedAt: '2026-12-01T10:00:00Z',
        latitude: 55.75,
        longitude: 37.62,
        quality: { status: 'good' },
        providerEventId: 'provider-event-secret',
        providerCode: 'provider-code-secret',
      }],
      total: 1,
      limit: 50,
      offset: 0,
    }
  }
  if (suffix === '/eta') {
    return {
      pickup: { status: 'AVAILABLE', freshnessStatus: 'fresh', qualityStatus: 'good', estimatedArrivalAt: '2026-12-01T11:00:00Z' },
      delivery: null,
    }
  }
  if (suffix === '/eta/history') {
    return {
      items: [{ estimatedArrivalAt: '2026-12-01T11:00:00Z', targetType: url.searchParams.get('targetType'), qualityStatus: 'good' }],
      total: 1,
      limit: 50,
      offset: 0,
    }
  }
  if (suffix === '/slots') {
    return {
      pickup: {
        windowStatus: 'OPEN',
        windowStart: '2026-12-01T08:00:00Z',
        windowEnd: '2026-12-01T10:00:00Z',
        timezone: 'Europe/Moscow',
        qualityStatus: 'good',
      },
      delivery: null,
      providerSlotId: 'provider-slot-secret',
      providerConfidence: 'provider-confidence-secret',
    }
  }
  return {
    items: [{
      windowStart: '2026-12-01T08:00:00Z',
      windowEnd: '2026-12-01T10:00:00Z',
      timezone: 'Europe/Moscow',
      slotStatus: 'BOOKED',
      slotType: url.searchParams.get('slotType'),
    }],
    total: 1,
    limit: 50,
    offset: 0,
  }
}

async function installGateway(page: Page, roles: string[], options: GatewayOptions = {}) {
  let responseReady = false
  const calls: string[] = []
  await page.route('**/api/v1/**', async (route: Route) => {
    const request = route.request()
    const headers = request.headers()
    expect(headers['x-tenant-id']).toBeUndefined()
    expect(headers['x-user-id']).toBeUndefined()
    expect(headers['x-user-email']).toBeUndefined()
    const url = new URL(request.url())
    calls.push(`${request.method()} ${url.pathname}${url.search}`)
    if (url.pathname === '/api/v1/auth/login') {
      await route.fulfill({ json: loginPayload(roles) })
      return
    }
    if (url.pathname === '/api/v1/users/user-1/companies') {
      expect(url.searchParams.get('tenant_id')).toBe('tenant-1')
      const officeRole = roles.find((role) => role === 'CARRIER_ADMIN' || role === 'CARRIER_DISPATCHER')
      await route.fulfill({
        json: {
          items: officeRole
            ? [{
                membership_id: 'm-1',
                company_id: 'co-1',
                legal_name: 'Carrier Co',
                membership_status: 'ACTIVE',
                roles: [{ code: officeRole }],
              }]
            : [{
                membership_id: 'm-driver',
                company_id: 'co-driver',
                legal_name: 'Driver Co',
                membership_status: 'ACTIVE',
                roles: [{ code: roles[0] ?? 'DRIVER' }],
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
      const body = request.postDataJSON() as { participant_company_id?: string }
      expect(body.participant_company_id).toBe('co-1')
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
          items: options.orders ?? [{
            transport_order_id: 'order-1',
            transport_order_number: 'TO-1',
            transport_order_status: 'CONFIRMED',
          }],
        },
      })
      return
    }
    if (url.pathname === '/api/v1/order-execution/transport-orders/order-1') {
      expect(url.searchParams.get('company_id')).toBe('co-1')
      expect(url.searchParams.get('actor')).toBe('CARRIER')
      await route.fulfill({
        json: options.orderDetail ?? {
          transport_order_id: 'order-1',
          transport_order_number: 'TO-1',
          transport_order_status: 'CONFIRMED',
          provenance: { amount: 1500, currency_code: 'RUB' },
        },
      })
      return
    }
    if (url.pathname.startsWith('/api/v1/shipments/')) {
      await route.fulfill({ status: 500, json: { message: 'legacy tracking route' } })
      return
    }
    const carrierShipment = url.pathname.match(/^\/api\/v1\/carrier\/shipments\/([^/]+)(\/tracking(?:\/locations)?|\/eta(?:\/history)?|\/slots(?:\/history)?)$/)
    if (carrierShipment) {
      expect(url.searchParams.get('carrier_company_id')).toBe('co-1')
      expect(headers['x-company-id']).toBe('co-1')
      const suffix = carrierShipment[2] ?? ''
      if (options.tracking === 'not_found') {
        await route.fulfill({ status: 404, json: { message: 'not found' } })
        return
      }
      if (options.tracking === 'tracking_forbidden' && suffix === '/tracking') {
        await route.fulfill({ status: 403, json: { message: 'forbidden' } })
        return
      }
      if (options.tracking === 'tracking_unavailable' && suffix === '/tracking') {
        await route.fulfill({ status: 503, json: { message: 'unavailable' } })
        return
      }
      if (suffix === '/eta/history') {
        expect(['pickup', 'delivery']).toContain(url.searchParams.get('targetType'))
      }
      if (suffix === '/slots/history') {
        expect(['pickup', 'delivery']).toContain(url.searchParams.get('slotType'))
      }
      if (suffix.endsWith('/history') || suffix.endsWith('/locations')) {
        expect(Number(url.searchParams.get('limit'))).toBeLessThanOrEqual(50)
      }
      await route.fulfill({ json: trackingBody(suffix, url) })
      return
    }
    if (url.pathname === '/api/v1/drivers') {
      expect(url.searchParams.get('carrier_company_id')).toBe('co-1')
      await route.fulfill({ json: { items: [{ id: 'd-1', full_name: 'Driver A', status: 'ACTIVE' }] } })
      return
    }
    if (url.pathname === '/api/v1/vehicles') {
      expect(url.searchParams.get('carrier_company_id')).toBe('co-1')
      await route.fulfill({ json: { items: [{ id: 'v-1', plate_number: 'A100AA', status: 'ACTIVE' }] } })
      return
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

test('carrier office journey covers tenders, award, transport orders, and fleet', async ({ page }) => {
  const calls = await installGateway(page, ['CARRIER_DISPATCHER'])
  await signIn(page)
  await chooseCompany(page)
  expect(calls.some((call) => call.includes('/carrier/') || call.includes('/drivers'))).toBe(false)

  await page.getByRole('link', { name: 'Тендеры' }).click()
  await expect(page.getByTestId('tender-inbox')).toBeVisible()
  await expect(page.getByText('North lane')).toBeVisible()
  await page.getByTestId('open-tender').click()
  await expect(page.getByTestId('tender-detail')).toBeVisible()
  await expect(page.getByTestId('tender-deadline')).toContainText('2026-12-01T00:00:00Z')
  await page.getByTestId('create-response').click()
  await page.getByTestId('offer-amount').fill('1500')
  await page.getByTestId('offer-currency').fill('RUB')
  await page.getByTestId('save-offer').click()
  await page.getByTestId('submit-response').click()
  await expect(page.getByTestId('award-amount')).toContainText('1500')

  await page.getByRole('link', { name: 'Заявки' }).click()
  await expect(page.getByText('TO-1')).toBeVisible()
  await page.getByTestId('open-order').click()
  await expect(page.getByTestId('order-status')).toContainText('CONFIRMED')

  await page.getByRole('link', { name: 'Автопарк' }).click()
  await expect(page.getByTestId('fleet-view')).toBeVisible()
  await expect(page.getByTestId('fleet-create')).toHaveCount(0)
  await expect(page.getByText('Driver A')).toBeVisible()
})

test('carrier admin fleet stays read-only', async ({ page }) => {
  await installGateway(page, ['CARRIER_ADMIN'])
  await signIn(page)
  await chooseCompany(page)
  await page.getByRole('link', { name: 'Автопарк' }).click()
  await expect(page.getByTestId('fleet-view')).toBeVisible()
  await expect(page.getByTestId('fleet-create')).toHaveCount(0)
})

test('tampered selected company is cleared before business calls', async ({ page }) => {
  const calls = await installGateway(page, ['CARRIER_DISPATCHER'])
  await signIn(page)
  await chooseCompany(page)
  await page.evaluate(() => {
    const key = 'freight_carrier_tab_session'
    const raw = JSON.parse(sessionStorage.getItem(key) ?? '{}') as Record<string, unknown>
    raw.selectedCompanyId = 'co-spoof'
    raw.memberships = [{
      membershipId: 'm-spoof',
      companyId: 'co-spoof',
      legalName: 'Spoof Co',
      membershipStatus: 'ACTIVE',
      roleCodes: ['CARRIER_ADMIN'],
    }]
    sessionStorage.setItem(key, JSON.stringify(raw))
  })
  const marked = calls.length
  await page.reload()
  await expect(page.getByTestId('company-gate')).toBeVisible()
  const afterReload = calls.slice(marked)
  expect(afterReload.some((call) => call.includes('co-spoof'))).toBe(false)
  expect(afterReload.some((call) => call.includes('/api/v1/drivers') || call.includes('/api/v1/vehicles') || call.includes('/api/v1/carrier/'))).toBe(false)
})

const trackedOrders = [
  {
    transport_order_id: 'order-1',
    transport_order_number: 'TO-1',
    transport_order_status: 'CONFIRMED',
    shipment_id: 'shipment-A',
    shipment_status: 'IN_TRANSIT',
  },
  {
    transport_order_id: 'order-2',
    transport_order_number: 'TO-2',
    transport_order_status: 'CONFIRMED',
  },
]

test('tracking opens from the server shipment reference and reads only carrier routes', async ({ page }) => {
  const calls = await installGateway(page, ['CARRIER_DISPATCHER'], { orders: trackedOrders, tracking: 'ready' })
  await signIn(page)
  await chooseCompany(page)
  await expect(page.getByRole('link', { name: 'Отслеживание' })).toHaveCount(0)
  await page.getByRole('link', { name: 'Заявки' }).click()
  const linked = page.locator('tr', { hasText: 'TO-1' })
  const unlinked = page.locator('tr', { hasText: 'TO-2' })
  await expect(linked.getByTestId('shipment-status')).toHaveText('IN_TRANSIT')
  await expect(linked.getByTestId('open-tracking')).toHaveAttribute('href', '/shipments/shipment-A/tracking')
  await expect(unlinked.getByTestId('open-tracking')).toHaveCount(0)
  await linked.getByTestId('open-tracking').click()
  await expect(page.getByTestId('tracking-status')).toHaveText('ACTIVE')
  await expect(page.getByTestId('position-latitude')).toHaveText('55.75')
  await expect(page.getByTestId('eta-arrival')).toHaveText('2026-12-01T11:00:00Z')
  await expect(page.getByTestId('window-timezone')).toHaveText('Europe/Moscow')
  await expect(page.getByText('provider-device-secret')).toHaveCount(0)
  await expect(page.getByText('provider-slot-secret')).toHaveCount(0)
  const families = [
    '/api/v1/carrier/shipments/shipment-A/tracking?',
    '/api/v1/carrier/shipments/shipment-A/tracking/locations?',
    '/api/v1/carrier/shipments/shipment-A/eta?',
    '/api/v1/carrier/shipments/shipment-A/eta/history?',
    '/api/v1/carrier/shipments/shipment-A/slots?',
    '/api/v1/carrier/shipments/shipment-A/slots/history?',
  ]
  for (const family of families) {
    expect(calls.some((call) => call.startsWith(`GET ${family}`) && call.includes('carrier_company_id=co-1'))).toBe(true)
  }
  expect(calls.some((call) => call.includes('targetType=pickup'))).toBe(true)
  expect(calls.some((call) => call.includes('targetType=delivery'))).toBe(true)
  expect(calls.some((call) => call.includes('slotType=pickup'))).toBe(true)
  expect(calls.some((call) => call.includes('slotType=delivery'))).toBe(true)
  expect(calls.some((call) => call.includes('/api/v1/shipments/'))).toBe(false)
  expect(calls.some((call) => call.includes('execution_stop'))).toBe(false)
})

test('transport order detail uses the nested shipment id', async ({ page }) => {
  await installGateway(page, ['CARRIER_ADMIN'], {
    orderDetail: {
      transport_order_id: 'order-1',
      transport_order_number: 'TO-1',
      transport_order_status: 'CONFIRMED',
      shipment: { id: 'shipment-A', shipment_number: 'SHP-A', status: 'IN_TRANSIT' },
      provenance: { amount: 1500, currency_code: 'RUB' },
    },
    tracking: 'ready',
  })
  await signIn(page)
  await chooseCompany(page)
  await page.getByRole('link', { name: 'Заявки' }).click()
  await page.getByTestId('open-order').click()
  await expect(page.getByTestId('open-tracking')).toHaveAttribute('href', '/shipments/shipment-A/tracking')
  await expect(page.getByTestId('shipment-status')).toHaveText('IN_TRANSIT')
})

test('a cross-carrier tracking deep link stays on the selected company', async ({ page }) => {
  const calls = await installGateway(page, ['CARRIER_DISPATCHER'], { tracking: 'not_found' })
  await signIn(page)
  await chooseCompany(page)
  await page.goto('/shipments/shipment-b/tracking')
  await expect(page.getByTestId('tracking-section')).toContainText('Отслеживание недоступно для выбранной компании')
  await expect(page.getByText('another carrier')).toHaveCount(0)
  await expect(page.getByText('Carrier B')).toHaveCount(0)
  await expect(page.getByText('co-2')).toHaveCount(0)
  const trackingCalls = calls.filter((call) => call.includes('/api/v1/carrier/shipments/shipment-b/'))
  expect(trackingCalls.length).toBeGreaterThan(0)
  expect(trackingCalls.every((call) => call.includes('carrier_company_id=co-1'))).toBe(true)
  expect(calls.some((call) => call.includes('/api/v1/shipments/'))).toBe(false)
})

test('a forbidden tracking section leaves the other sections visible', async ({ page }) => {
  await installGateway(page, ['CARRIER_DISPATCHER'], { tracking: 'tracking_forbidden' })
  await signIn(page)
  await chooseCompany(page)
  await page.goto('/shipments/shipment-A/tracking')
  await expect(page.getByTestId('tracking-section')).toContainText('Доступ запрещён')
  await expect(page.getByTestId('eta-arrival')).toHaveText('2026-12-01T11:00:00Z')
  await expect(page.getByTestId('window-timezone')).toHaveText('Europe/Moscow')
})

test('an unavailable tracking section leaves locations and forecasts visible', async ({ page }) => {
  await installGateway(page, ['CARRIER_DISPATCHER'], { tracking: 'tracking_unavailable' })
  await signIn(page)
  await chooseCompany(page)
  await page.goto('/shipments/shipment-A/tracking')
  await expect(page.getByTestId('tracking-section')).toContainText('Сервис недоступен')
  await expect(page.getByTestId('location-row')).toContainText('55.75')
  await expect(page.getByTestId('eta-arrival')).toHaveText('2026-12-01T11:00:00Z')
  await expect(page.getByTestId('slot-pickup')).toContainText('Europe/Moscow')
})

test('driver cannot enter the carrier office', async ({ page }) => {
  const calls = await installGateway(page, ['DRIVER'])
  await signIn(page)
  await expect(page.getByText('Эта роль не входит в кабинет перевозчика.')).toBeVisible()
  expect(calls.some((call) => call.includes('/carrier/') || call.includes('/drivers'))).toBe(false)
})
