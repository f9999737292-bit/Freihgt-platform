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

async function installGateway(page: Page, roles: string[]) {
  let responseReady = false
  const calls: string[] = []
  await page.route('**/api/v1/**', async (route: Route) => {
    const request = route.request()
    const headers = request.headers()
    expect(headers['x-tenant-id']).toBeUndefined()
    expect(headers['x-user-id']).toBeUndefined()
    expect(headers['x-user-email']).toBeUndefined()
    const url = new URL(request.url())
    calls.push(`${request.method()} ${url.pathname}`)
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
      await route.fulfill({
        json: {
          items: [{
            transport_order_id: 'order-1',
            transport_order_number: 'TO-1',
            transport_order_status: 'CONFIRMED',
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
          provenance: { amount: 1500, currency_code: 'RUB' },
        },
      })
      return
    }
    if (url.pathname === '/api/v1/drivers') {
      await route.fulfill({ json: { items: [{ id: 'd-1', full_name: 'Driver A', status: 'ACTIVE' }] } })
      return
    }
    if (url.pathname === '/api/v1/vehicles') {
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

test('driver cannot enter the carrier office', async ({ page }) => {
  const calls = await installGateway(page, ['DRIVER'])
  await signIn(page)
  await expect(page.getByText('Эта роль не входит в кабинет перевозчика.')).toBeVisible()
  expect(calls.some((call) => call.includes('/carrier/') || call.includes('/drivers'))).toBe(false)
})
