import { expect, test, type Page, type Route } from '@playwright/test'

const FACT_KEYS = [
  'plannedPickupAt',
  'plannedDeliveryAt',
  'actualPickupAt',
  'actualDeliveryAt',
  'shipmentStatus',
  'pickupEtaStatus',
  'deliveryEtaStatus',
  'pickupEstimatedArrivalAt',
  'deliveryEstimatedArrivalAt',
  'pickupEtaFreshness',
  'deliveryEtaFreshness',
  'pickupEtaQuality',
  'deliveryEtaQuality',
]

const tracking = {
  shipmentId: 'shp-1',
  trackingStatus: 'ACTIVE',
  freshness: { status: 'fresh', ageSeconds: 12 },
  quality: { status: 'good' },
  lastKnownPosition: {
    latitude: 55.75,
    longitude: 37.62,
    recordedAt: '2026-12-01T10:00:00Z',
    ageSeconds: 12,
  },
  speedKph: 40,
  headingDegrees: 90,
}

const locations = {
  items: [{
    latitude: 55.75,
    longitude: 37.62,
    recordedAt: '2026-12-01T10:00:00Z',
    speedKph: 40,
    headingDegrees: 90,
    accuracyMeters: 8,
    quality: { status: 'good' },
    sourceType: 'telematics',
  }],
  total: 1,
  limit: 50,
  offset: 0,
}

const eta = {
  shipmentId: 'shp-1',
  pickup: {
    status: 'AVAILABLE',
    freshnessStatus: 'fresh',
    qualityStatus: 'good',
    estimatedArrivalAt: '2026-12-01T11:00:00Z',
    sourceType: 'schedule',
  },
  delivery: {
    status: 'AVAILABLE',
    freshnessStatus: 'fresh',
    qualityStatus: 'good',
    estimatedArrivalAt: '2026-12-02T16:00:00Z',
    sourceType: 'schedule',
  },
}

const slots = {
  shipmentId: 'shp-1',
  pickup: {
    windowStatus: 'OPEN',
    slotStatus: 'CONFIRMED',
    windowStart: '2026-12-01T08:00:00Z',
    windowEnd: '2026-12-01T10:00:00Z',
    timezone: 'Europe/Moscow',
    qualityStatus: 'good',
  },
  delivery: {
    windowStatus: 'OPEN',
    slotStatus: 'CONFIRMED',
    windowStart: '2026-12-02T15:00:00Z',
    windowEnd: '2026-12-02T18:00:00Z',
    timezone: 'Europe/Moscow',
    qualityStatus: 'good',
  },
}

function assertSafeRead(url: URL, headers: Record<string, string>) {
  expect(url.pathname.startsWith('/api/v1/shipper/shipments/')).toBe(true)
  expect(url.searchParams.get('shipper_company_id')).toBe('co-shipper')
  expect(headers['x-company-id']).toBe('co-shipper')
  for (const key of FACT_KEYS) expect(url.searchParams.has(key)).toBe(false)
}

const shipment = {
  id: 'shp-1',
  shipment_number: 'SHP-1',
  status: 'BOOKED',
  transport_order_id: 'to-1',
  shipper_company_id: 'co-shipper',
  consignee_company_id: 'co-consignee',
  carrier_company_id: 'co-carrier',
  forwarder_company_id: 'co-forwarder',
  origin_location_id: 'loc-origin',
  destination_location_id: 'loc-dest',
  cargo_id: 'cargo-1',
  transport_mode: 'ROAD',
  planned_pickup_at: '2026-12-01T08:00:00Z',
  planned_delivery_at: '2026-12-02T18:00:00Z',
  actual_pickup_at: '2026-12-01T09:00:00Z',
  actual_delivery_at: null,
}

function loginPayload(roles: string[]) {
  return {
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
  }
}

function membership(companyId: string, companyType: string, roles: string[]) {
  return {
    membership_id: `m-${companyId}`,
    company_id: companyId,
    legal_name: companyId,
    company_type: companyType,
    membership_status: 'ACTIVE',
    roles: roles.map((code) => ({ code })),
  }
}

async function installGateway(
  page: Page,
  roles: string[],
  memberships = [membership('co-shipper', 'SHIPPER', roles)],
) {
  const calls: string[] = []
  await page.route('**/api/v1/**', async (route: Route) => {
    const request = route.request()
    const headers = request.headers()
    expect(headers['x-tenant-id']).toBeUndefined()
    expect(headers['x-user-id']).toBeUndefined()
    expect(headers['x-user-email']).toBeUndefined()
    expect(headers['x-actor-kind']).toBeUndefined()
    expect(headers['x-user-role']).toBeUndefined()
    const url = new URL(request.url())
    calls.push(`${request.method()} ${url.pathname}${url.search}`)
    expect(url.pathname === '/api/v1/shipments' || url.pathname.startsWith('/api/v1/shipments/')).toBe(false)
    if (url.pathname === '/api/v1/auth/login') {
      await route.fulfill({ json: loginPayload(roles) })
      return
    }
    if (url.pathname === '/api/v1/users/user-1/companies') {
      expect(url.searchParams.get('tenant_id')).toBe('tenant-1')
      await route.fulfill({ json: { items: memberships } })
      return
    }
    if (url.pathname === '/api/v1/shipper/shipments') {
      expect(url.searchParams.get('shipper_company_id')).toBe('co-shipper')
      expect(headers['x-company-id']).toBe('co-shipper')
      await route.fulfill({ json: { items: [shipment], total: 1 } })
      return
    }
    if (url.pathname === '/api/v1/shipper/shipments/shp-1') {
      assertSafeRead(url, headers)
      await route.fulfill({ json: shipment })
      return
    }
    if (url.pathname === '/api/v1/shipper/shipments/shp-1/tracking') {
      assertSafeRead(url, headers)
      await route.fulfill({ json: tracking })
      return
    }
    if (url.pathname === '/api/v1/shipper/shipments/shp-1/tracking/locations') {
      assertSafeRead(url, headers)
      expect(url.searchParams.get('limit')).toBe('50')
      expect(url.searchParams.get('offset')).toBe('0')
      await route.fulfill({ json: locations })
      return
    }
    if (url.pathname === '/api/v1/shipper/shipments/shp-1/eta') {
      assertSafeRead(url, headers)
      await route.fulfill({ json: eta })
      return
    }
    if (url.pathname === '/api/v1/shipper/shipments/shp-1/eta/history') {
      assertSafeRead(url, headers)
      expect(['pickup', 'delivery']).toContain(url.searchParams.get('targetType'))
      expect(url.searchParams.get('limit')).toBe('50')
      const target = url.searchParams.get('targetType')
      await route.fulfill({
        json: {
          items: [{
            targetType: target,
            estimatedArrivalAt: target === 'pickup' ? '2026-12-01T11:00:00Z' : '2026-12-02T16:00:00Z',
            sourceObservedAt: '2026-12-01T09:00:00Z',
            receivedAt: '2026-12-01T09:01:00Z',
            qualityStatus: 'good',
            sourceType: 'schedule',
          }],
          total: 1,
          limit: 50,
          offset: 0,
        },
      })
      return
    }
    if (url.pathname === '/api/v1/shipper/shipments/shp-1/slots') {
      assertSafeRead(url, headers)
      await route.fulfill({ json: slots })
      return
    }
    if (url.pathname === '/api/v1/shipper/shipments/shp-1/slots/history') {
      assertSafeRead(url, headers)
      expect(['pickup', 'delivery']).toContain(url.searchParams.get('slotType'))
      expect(url.searchParams.get('limit')).toBe('50')
      const slotType = url.searchParams.get('slotType')
      await route.fulfill({
        json: {
          items: [{
            slotType,
            windowStart: slotType === 'pickup' ? '2026-12-01T08:00:00Z' : '2026-12-02T15:00:00Z',
            windowEnd: slotType === 'pickup' ? '2026-12-01T10:00:00Z' : '2026-12-02T18:00:00Z',
            timezone: 'Europe/Moscow',
            slotStatus: 'CONFIRMED',
            qualityStatus: 'good',
          }],
          total: 1,
          limit: 50,
          offset: 0,
        },
      })
      return
    }
    await route.fulfill({ status: 404, json: { message: url.pathname } })
  })
  return calls
}

async function signIn(page: Page) {
  await page.goto('/login')
  await page.getByTestId('tenant-id').fill('tenant-1')
  await page.getByTestId('email').fill('shipper@example.com')
  await page.getByTestId('password').fill('secret')
  await page.getByTestId('sign-in').click()
}

async function chooseCompany(page: Page) {
  await expect(page.getByTestId('company-gate')).toBeVisible()
  await page.getByTestId('company-select').selectOption('co-shipper')
  await page.getByTestId('company-continue').click()
  await expect(page.getByTestId('home-shell')).toBeVisible()
}

test('shipper admin reads the company-scoped shipment inbox and detail', async ({ page }) => {
  const calls = await installGateway(page, ['SHIPPER_ADMIN'])
  await signIn(page)
  await chooseCompany(page)
  expect(calls.some((call) => call.includes('/api/v1/shipper/shipments'))).toBe(false)

  await page.getByRole('link', { name: 'Отправления' }).click()
  await expect(page.getByTestId('shipment-inbox')).toBeVisible()
  await expect(page.getByText('SHP-1')).toBeVisible()
  await expect(page.getByText('co-carrier')).toBeVisible()
  await expect(page.getByText('co-consignee')).toBeVisible()
  await page.getByTestId('status-filter').fill('BOOKED')
  await page.getByTestId('apply-status').click()
  await expect.poll(() => calls.some((call) => call.includes('status=BOOKED') && call.includes('shipper_company_id=co-shipper'))).toBe(true)

  await page.getByTestId('open-shipment').click()
  await expect(page.getByTestId('shipment-detail')).toBeVisible()
  await expect(page.getByTestId('shipment-status')).toContainText('BOOKED')
  await expect(page.getByTestId('transport-mode')).toContainText('ROAD')
  await expect(page.getByTestId('planned-pickup')).toContainText('2026-12-01T08:00:00Z')
  await expect(page.getByTestId('actual-delivery')).toContainText('—')
  expect(calls.some((call) => call.includes('/api/v1/shipper/shipments/shp-1') && call.includes('shipper_company_id=co-shipper'))).toBe(true)

  await page.getByTestId('open-tracking').click()
  await expect(page.getByTestId('shipment-tracking')).toBeVisible()
  await expect(page.getByTestId('tracking-status')).toContainText('ACTIVE')
  await expect(page.getByTestId('tracking-freshness')).toContainText('fresh')
  await expect(page.getByTestId('position-latitude')).toContainText('55.75')
  await expect(page.getByTestId('position-longitude')).toContainText('37.62')
  await expect(page.getByTestId('location-row')).toContainText('55.75')
  await expect(page.getByTestId('eta-pickup')).toContainText('2026-12-01T11:00:00Z')
  await expect(page.getByTestId('eta-delivery')).toContainText('2026-12-02T16:00:00Z')
  await expect(page.getByTestId('slot-pickup')).toContainText('Europe/Moscow')
  await expect(page.getByTestId('slot-delivery')).toContainText('2026-12-02T18:00:00Z')
  const body = await page.locator('body').innerText()
  expect(body).not.toContain('providerDeviceId')
  expect(body).not.toContain('providerEventId')
  expect(body).not.toContain('providerSlotId')
  expect(body).not.toContain('X-Internal-Service-Token')
  const trackingCalls = calls.filter((call) => call.includes('/api/v1/shipper/shipments/shp-1/'))
  expect(trackingCalls.some((call) => call.includes('/tracking?') || call.includes('/tracking&'))).toBe(true)
  expect(trackingCalls.every((call) => call.includes('shipper_company_id=co-shipper'))).toBe(true)
  expect(calls.some((call) => call.includes('/api/v1/shipments/'))).toBe(false)
})

test('shipper logist can open the shipment inbox and tracking', async ({ page }) => {
  const calls = await installGateway(page, ['SHIPPER_LOGIST'])
  await signIn(page)
  await chooseCompany(page)
  await page.getByRole('link', { name: 'Отправления' }).click()
  await expect(page.getByText('SHP-1')).toBeVisible()
  await page.getByTestId('open-shipment').click()
  await page.getByTestId('open-tracking').click()
  await expect(page.getByTestId('tracking-status')).toContainText('ACTIVE')
  await expect(page.getByTestId('eta-pickup')).toBeVisible()
  await expect(page.getByTestId('slot-delivery')).toBeVisible()
  expect(calls.some((call) => call.includes('/api/v1/shipper/shipments/shp-1/tracking') && call.includes('shipper_company_id=co-shipper'))).toBe(true)
  expect(calls.some((call) => call.includes('/api/v1/shipments/'))).toBe(false)
})

test('tampered company is discarded before a direct tracking visit', async ({ page }) => {
  const calls = await installGateway(page, ['SHIPPER_ADMIN'])
  await signIn(page)
  await chooseCompany(page)
  await page.evaluate(() => {
    const key = 'freight_shipper_tab_session'
    const raw = JSON.parse(sessionStorage.getItem(key) ?? '{}') as Record<string, unknown>
    raw.selectedCompanyId = 'co-spoof'
    raw.memberships = [{
      membershipId: 'm-spoof',
      companyId: 'co-spoof',
      legalName: 'Spoof Co',
      companyType: 'SHIPPER',
      membershipStatus: 'ACTIVE',
      roleCodes: ['SHIPPER_ADMIN'],
    }]
    sessionStorage.setItem(key, JSON.stringify(raw))
  })
  const marked = calls.length
  await page.goto('/shipments/shp-1/tracking')
  await expect(page.getByTestId('company-gate')).toBeVisible()
  const after = calls.slice(marked)
  expect(after.some((call) => call.includes('co-spoof'))).toBe(false)
  expect(after.some((call) => call.includes('/tracking') || call.includes('/eta') || call.includes('/slots'))).toBe(false)
  expect(after.some((call) => call.includes('/api/v1/shipments/'))).toBe(false)
})

test('a shipper role on a non-shipper company cannot be selected', async ({ page }) => {
  await installGateway(page, ['SHIPPER_ADMIN'], [
    membership('co-other', 'CARRIER', ['SHIPPER_ADMIN']),
    membership('co-shipper', 'SHIPPER', ['SHIPPER_LOGIST']),
  ])
  await signIn(page)
  await expect(page.getByTestId('company-gate')).toBeVisible()
  const values = await page.getByTestId('company-select').locator('option').evaluateAll((options) =>
    options.map((option) => (option as HTMLOptionElement).value),
  )
  expect(values).toEqual(['co-shipper'])
})

test('tampered selected company is cleared before shipment calls', async ({ page }) => {
  const calls = await installGateway(page, ['SHIPPER_ADMIN'])
  await signIn(page)
  await chooseCompany(page)
  await page.evaluate(() => {
    const key = 'freight_shipper_tab_session'
    const raw = JSON.parse(sessionStorage.getItem(key) ?? '{}') as Record<string, unknown>
    raw.selectedCompanyId = 'co-spoof'
    raw.memberships = [{
      membershipId: 'm-spoof',
      companyId: 'co-spoof',
      legalName: 'Spoof Co',
      companyType: 'SHIPPER',
      membershipStatus: 'ACTIVE',
      roleCodes: ['SHIPPER_ADMIN'],
    }]
    sessionStorage.setItem(key, JSON.stringify(raw))
  })
  const marked = calls.length
  await page.reload()
  await expect(page.getByTestId('company-gate')).toBeVisible()
  const afterReload = calls.slice(marked)
  expect(afterReload.some((call) => call.includes('co-spoof'))).toBe(false)
  expect(afterReload.some((call) => call.includes('/api/v1/shipper/shipments'))).toBe(false)
  expect(await page.evaluate(() => sessionStorage.getItem('freight_carrier_tab_session'))).toBeNull()
})

test.describe('non-shipper roles stay out of the office', () => {
  const cases = [
    { name: 'forwarder', roles: ['FORWARDER_MANAGER'], companyType: 'FORWARDER' },
    { name: 'carrier', roles: ['CARRIER_ADMIN'], companyType: 'CARRIER' },
    { name: 'procurement', roles: ['PROCUREMENT_MANAGER'], companyType: 'SHIPPER' },
    { name: 'driver', roles: ['DRIVER'], companyType: 'CARRIER' },
  ]
  for (const entry of cases) {
    test(`${entry.name} cannot enter`, async ({ page }) => {
      const calls = await installGateway(page, entry.roles, [
        membership('co-other', entry.companyType, entry.roles),
      ])
      await signIn(page)
      await expect(page.getByText('Эта роль не входит в кабинет грузоотправителя.')).toBeVisible()
      expect(calls.some((call) => call.includes('/api/v1/shipper/shipments'))).toBe(false)
    })
  }
})
