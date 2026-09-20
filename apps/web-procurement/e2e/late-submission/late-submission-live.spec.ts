import { expect, test, type Page } from '@playwright/test'
import {
  attachLateSubmissionNetworkProbe,
  formatLateSubmissionNetworkProbe,
  fulfillJSON,
  requireLateSubmissionEnv,
  stubCarrierMemberships,
} from './helpers'

function liveFixture() {
  return {
    webURL: requireLateSubmissionEnv('BROWSER_E2E_WEB_URL'),
    gatewayURL: requireLateSubmissionEnv('BROWSER_E2E_GATEWAY_URL'),
    tenantId: requireLateSubmissionEnv('BROWSER_E2E_TENANT_ID'),
    carrierCompanyId: requireLateSubmissionEnv('BROWSER_E2E_CARRIER_COMPANY_ID'),
    buyerCompanyId: requireLateSubmissionEnv('BROWSER_E2E_BUYER_COMPANY_ID'),
    carrierJwt: requireLateSubmissionEnv('BROWSER_E2E_JWT'),
    buyerJwt: requireLateSubmissionEnv('BROWSER_E2E_BUYER_JWT'),
    logistJwt: requireLateSubmissionEnv('BROWSER_E2E_LOGIST_JWT'),
    carrierUserId: requireLateSubmissionEnv('BROWSER_E2E_USER_ID'),
    buyerUserId: requireLateSubmissionEnv('BROWSER_E2E_BUYER_USER_ID'),
    logistUserId: requireLateSubmissionEnv('BROWSER_E2E_LOGIST_USER_ID'),
    createEventId: requireLateSubmissionEnv('BROWSER_E2E_EVENT_ID'),
    submitEventId: requireLateSubmissionEnv('BROWSER_E2E_SUBMIT_EVENT_ID'),
    rejectEventId: requireLateSubmissionEnv('BROWSER_E2E_REJECT_EVENT_ID'),
    notStartedEventId: requireLateSubmissionEnv('BROWSER_E2E_WINDOW_NOT_STARTED_EVENT_ID'),
    expiredEventId: requireLateSubmissionEnv('BROWSER_E2E_WINDOW_EXPIRED_EVENT_ID'),
    createRfxNumber: requireLateSubmissionEnv('BROWSER_E2E_RFX_NUMBER'),
    submitRfxNumber: requireLateSubmissionEnv('BROWSER_E2E_SUBMIT_RFX_NUMBER'),
    rejectRfxNumber: requireLateSubmissionEnv('BROWSER_E2E_REJECT_RFX_NUMBER'),
    notStartedRfxNumber: requireLateSubmissionEnv('BROWSER_E2E_WINDOW_NOT_STARTED_RFX_NUMBER'),
    expiredRfxNumber: requireLateSubmissionEnv('BROWSER_E2E_WINDOW_EXPIRED_RFX_NUMBER'),
    createResponseId: requireLateSubmissionEnv('BROWSER_E2E_RESPONSE_ID'),
  }
}

async function seedLiveSession(
  page: Page,
  input: { token: string; user: string; company: string; roles: string[] },
) {
  const fix = liveFixture()
  await page.addInitScript((payload) => {
    localStorage.setItem(
      'freight_procurement_session',
      JSON.stringify({
        token: payload.token,
        user: {
          id: payload.user,
          tenant_id: payload.tenant,
          email: 'late-live@freight.test',
          full_name: 'Late Live',
          preferred_locale: 'en-US',
          status: 'ACTIVE',
          roles: payload.roles,
        },
      }),
    )
    localStorage.setItem('freight_procurement_tenant_id', payload.tenant)
    localStorage.setItem('freight_procurement_company_id', payload.company)
    localStorage.setItem('freight_procurement_rfx_late_submission', 'true')
    document.cookie = 'freight_procurement_locale=en-US; path=/'
  }, {
    token: input.token,
    tenant: fix.tenantId,
    company: input.company,
    user: input.user,
    roles: input.roles,
  })
}

async function stubLiveShell(page: Page, role: 'carrier' | 'buyer' | 'logist') {
  const fix = liveFixture()
  if (role === 'carrier') {
    await stubCarrierMemberships(page)
    return
  }
  const userId = role === 'logist' ? fix.logistUserId : fix.buyerUserId
  const membershipRole = role === 'logist' ? 'SHIPPER_LOGIST' : 'PROCUREMENT_MANAGER'
  await page.route(`**/api/v1/users/${userId}/companies**`, async (route) => {
    await fulfillJSON(route, 200, {
      items: [{
        membership_id: `${fix.buyerCompanyId}-membership`,
        company_id: fix.buyerCompanyId,
        legal_name: 'Buyer A',
        company_type: 'SHIPPER',
        membership_status: 'ACTIVE',
        roles: [{ code: membershipRole, name: membershipRole }],
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
        { id: fix.buyerCompanyId, legal_name: 'Buyer A', company_type: 'SHIPPER', status: 'ACTIVE' },
        { id: fix.carrierCompanyId, legal_name: 'Carrier A', company_type: 'CARRIER', status: 'ACTIVE' },
      ],
    })
  })
}

async function authHeaders(token: string, companyId: string) {
  return {
    Authorization: `Bearer ${token}`,
    'X-Company-ID': companyId,
  }
}

async function openCarrierTender(page: Page, eventId: string, rfxNumber: string) {
  const fix = liveFixture()
  const probe = attachLateSubmissionNetworkProbe(page)
  await seedLiveSession(page, {
    token: fix.carrierJwt,
    user: fix.carrierUserId,
    company: fix.carrierCompanyId,
    roles: ['CARRIER_DISPATCHER'],
  })
  await stubLiveShell(page, 'carrier')
  const eventWait = page.waitForResponse((resp) => {
    return new URL(resp.url()).pathname === `/api/v1/carrier/rfx-events/${eventId}`
      && resp.request().method() === 'GET'
  }, { timeout: 30_000 })
  await page.goto(`/carrier/tenders/${eventId}`, { waitUntil: 'domcontentloaded' })
  const eventLoaded = await eventWait.catch((error: Error) => {
    throw new Error(`${error.message}\n${formatLateSubmissionNetworkProbe(probe)}`)
  })
  if (eventLoaded.status() !== 200) {
    throw new Error(`carrier invited event GET ${eventLoaded.status()} ${eventLoaded.url()}\n${formatLateSubmissionNetworkProbe(probe)}`)
  }
  await expect(page.getByTestId('carrier-tender-rfx-number')).toHaveText(rfxNumber, { timeout: 30_000 })
  return probe
}

async function openBuyerTender(page: Page, eventId: string, rfxNumber: string, role: 'buyer' | 'logist' = 'buyer') {
  const fix = liveFixture()
  const probe = attachLateSubmissionNetworkProbe(page)
  await seedLiveSession(page, {
    token: role === 'logist' ? fix.logistJwt : fix.buyerJwt,
    user: role === 'logist' ? fix.logistUserId : fix.buyerUserId,
    company: fix.buyerCompanyId,
    roles: role === 'logist' ? ['SHIPPER_LOGIST'] : ['PROCUREMENT_MANAGER'],
  })
  await stubLiveShell(page, role)
  await page.goto(`/tenders/${eventId}`, { waitUntil: 'domcontentloaded' })
  await expect(page.getByTestId('tender-rfx-number')).toHaveText(rfxNumber, { timeout: 30_000 })
  return probe
}

test.describe('late submission live stack', () => {
  test('carrier creates a late request and GET /mine returns it', async ({ page }) => {
    const fix = liveFixture()
    const probe = await openCarrierTender(page, fix.createEventId, fix.createRfxNumber)
    await expect(page.getByTestId('carrier-late-submission-panel')).toBeVisible()
    await expect(page.getByTestId('carrier-late-commercial-locked')).toBeVisible()
    await expect(page.getByTestId('carrier-submit-response')).toHaveCount(0)
    const createWait = page.waitForResponse((resp) => {
      return resp.request().method() === 'POST'
        && new URL(resp.url()).pathname === `/api/v1/rfx-events/${fix.createEventId}/late-submission-requests`
    }, { timeout: 30_000 })
    await page.getByTestId('carrier-late-reason-text').fill('Live studio outage after the deadline')
    await page.getByTestId('carrier-late-request-submit').click()
    const created = await createWait.catch((error: Error) => {
      throw new Error(`${error.message}\n${formatLateSubmissionNetworkProbe(probe)}`)
    })
    expect(created.status(), formatLateSubmissionNetworkProbe(probe)).toBe(201)
    expect(created.request().headers()['idempotency-key'] || '').toMatch(/^late-create:/)
    await expect(page.getByTestId('carrier-late-request-status')).toContainText(/requested/i)
    const mine = await page.request.get(
      `${fix.gatewayURL}/api/v1/rfx-events/${fix.createEventId}/late-submission-requests/mine?carrier_company_id=${fix.carrierCompanyId}`,
      { headers: await authHeaders(fix.carrierJwt, fix.carrierCompanyId) },
    )
    expect(mine.ok(), `mine status=${mine.status()}`).toBeTruthy()
    const body = await mine.json() as { items: Array<{ status: string; carrier_company_id: string }> }
    expect(body.items.some((item) => item.status === 'REQUESTED')).toBe(true)
    expect(body.items[0]?.carrier_company_id).toBe(fix.carrierCompanyId)
    const commercial = await page.request.post(
      `${fix.gatewayURL}/api/v1/rfx-responses/${fix.createResponseId}/submit`,
      { headers: await authHeaders(fix.carrierJwt, fix.carrierCompanyId) },
    )
    expect(commercial.status()).toBe(409)
  })

  test('buyer approves and carrier late-submits the existing DRAFT questionnaire', async ({ page }) => {
    const fix = liveFixture()
    const probe = await openBuyerTender(page, fix.submitEventId, fix.submitRfxNumber)
    await expect(page.getByTestId('buyer-late-submission-queue')).toBeVisible()
    await expect(page.getByTestId('buyer-late-approve')).toBeVisible()
    const approveWait = page.waitForResponse((resp) => {
      return resp.request().method() === 'POST'
        && resp.url().includes(`/late-submission-requests/`)
        && resp.url().includes('/approve')
    }, { timeout: 30_000 })
    await page.getByTestId('buyer-late-approve').click()
    const approved = await approveWait.catch((error: Error) => {
      throw new Error(`${error.message}\n${formatLateSubmissionNetworkProbe(probe)}`)
    })
    expect(approved.ok(), `approve status=${approved.status()}`).toBeTruthy()
    expect(approved.request().headers()['idempotency-key'] || '').toMatch(/^late-approve:/)
    await expect(page.getByTestId('buyer-late-status')).toContainText(/approved/i)

    await seedLiveSession(page, {
      token: fix.carrierJwt,
      user: fix.carrierUserId,
      company: fix.carrierCompanyId,
      roles: ['CARRIER_DISPATCHER'],
    })
    await stubLiveShell(page, 'carrier')
    page.once('dialog', (dialog) => dialog.accept())
    const submitWait = page.waitForResponse((resp) => {
      return resp.request().method() === 'POST'
        && new URL(resp.url()).pathname === `/api/v1/rfx-events/${fix.submitEventId}/carrier-response/submit`
    }, { timeout: 30_000 })
    await page.goto(`/carrier/tenders/${fix.submitEventId}/questionnaire`, { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('carrier-response-workspace')).toBeVisible({ timeout: 30_000 })
    await expect(page.getByTestId('submit-questionnaire')).toBeEnabled()
    await page.getByTestId('submit-questionnaire').click()
    const submitted = await submitWait.catch((error: Error) => {
      throw new Error(`${error.message}\n${formatLateSubmissionNetworkProbe(probe)}`)
    })
    expect(submitted.ok(), `late submit status=${submitted.status()}`).toBeTruthy()
    expect(submitted.request().headers()['idempotency-key'] || '').toMatch(/^late-submit:/)
    await expect(page.getByTestId('post-submit-lock')).toBeVisible()
  })

  test('buyer reject keeps questionnaire submit blocked', async ({ page }) => {
    const fix = liveFixture()
    const probe = await openBuyerTender(page, fix.rejectEventId, fix.rejectRfxNumber)
    const rejectWait = page.waitForResponse((resp) => {
      return resp.request().method() === 'POST' && resp.url().includes('/reject')
    }, { timeout: 30_000 })
    await page.getByTestId('buyer-late-reject').click()
    const rejected = await rejectWait.catch((error: Error) => {
      throw new Error(`${error.message}\n${formatLateSubmissionNetworkProbe(probe)}`)
    })
    expect(rejected.ok(), `reject status=${rejected.status()}`).toBeTruthy()
    expect(rejected.request().headers()['idempotency-key'] || '').toMatch(/^late-reject:/)

    await seedLiveSession(page, {
      token: fix.carrierJwt,
      user: fix.carrierUserId,
      company: fix.carrierCompanyId,
      roles: ['CARRIER_DISPATCHER'],
    })
    await stubLiveShell(page, 'carrier')
    await page.goto(`/carrier/tenders/${fix.rejectEventId}/questionnaire`, { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('carrier-response-workspace')).toBeVisible({ timeout: 30_000 })
    await expect(page.getByTestId('late-submit-blocked')).toBeVisible()
    await expect(page.getByTestId('submit-questionnaire')).toBeDisabled()
  })

  test('questionnaire submit is forbidden before the approved window starts', async ({ page }) => {
    const fix = liveFixture()
    await seedLiveSession(page, {
      token: fix.carrierJwt,
      user: fix.carrierUserId,
      company: fix.carrierCompanyId,
      roles: ['CARRIER_DISPATCHER'],
    })
    await stubLiveShell(page, 'carrier')
    await page.goto(`/carrier/tenders/${fix.notStartedEventId}/questionnaire`, { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('carrier-response-workspace')).toBeVisible({ timeout: 30_000 })
    await expect(page.getByTestId('late-submit-blocked')).toContainText(/has not started/i)
    await expect(page.getByTestId('submit-questionnaire')).toBeDisabled()
    const workspace = await page.request.get(
      `${fix.gatewayURL}/api/v1/rfx-events/${fix.notStartedEventId}/carrier-response?carrier_company_id=${fix.carrierCompanyId}`,
      { headers: await authHeaders(fix.carrierJwt, fix.carrierCompanyId) },
    )
    expect(workspace.ok()).toBeTruthy()
    const saveVersion = ((await workspace.json()) as { save_version?: number }).save_version ?? 1
    const api = await page.request.post(
      `${fix.gatewayURL}/api/v1/rfx-events/${fix.notStartedEventId}/carrier-response/submit?carrier_company_id=${fix.carrierCompanyId}`,
      {
        headers: {
          ...await authHeaders(fix.carrierJwt, fix.carrierCompanyId),
          'Content-Type': 'application/json',
          'Idempotency-Key': `late-submit-probe-not-started-${Date.now()}`,
        },
        data: { save_version: saveVersion },
      },
    )
    expect(api.status()).toBe(422)
    const body = await api.json() as { error?: { details?: { field?: string } } }
    expect(body.error?.details?.field).toBe('approved_valid_from')
  })

  test('questionnaire submit is forbidden after the approved window ends', async ({ page }) => {
    const fix = liveFixture()
    await seedLiveSession(page, {
      token: fix.carrierJwt,
      user: fix.carrierUserId,
      company: fix.carrierCompanyId,
      roles: ['CARRIER_DISPATCHER'],
    })
    await stubLiveShell(page, 'carrier')
    await page.goto(`/carrier/tenders/${fix.expiredEventId}/questionnaire`, { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('carrier-response-workspace')).toBeVisible({ timeout: 30_000 })
    await expect(page.getByTestId('late-submit-blocked')).toContainText(/has expired/i)
    await expect(page.getByTestId('submit-questionnaire')).toBeDisabled()
    const workspace = await page.request.get(
      `${fix.gatewayURL}/api/v1/rfx-events/${fix.expiredEventId}/carrier-response?carrier_company_id=${fix.carrierCompanyId}`,
      { headers: await authHeaders(fix.carrierJwt, fix.carrierCompanyId) },
    )
    expect(workspace.ok()).toBeTruthy()
    const saveVersion = ((await workspace.json()) as { save_version?: number }).save_version ?? 1
    const api = await page.request.post(
      `${fix.gatewayURL}/api/v1/rfx-events/${fix.expiredEventId}/carrier-response/submit?carrier_company_id=${fix.carrierCompanyId}`,
      {
        headers: {
          ...await authHeaders(fix.carrierJwt, fix.carrierCompanyId),
          'Content-Type': 'application/json',
          'Idempotency-Key': `late-submit-probe-expired-${Date.now()}`,
        },
        data: { save_version: saveVersion },
      },
    )
    expect(api.status()).toBe(422)
    const body = await api.json() as { error?: { details?: { field?: string } } }
    expect(['approved_valid_until', 'late_submission_request', 'late_submission_status']).toContain(body.error?.details?.field)
  })

  test('SHIPPER_LOGIST can read the buyer queue but cannot approve', async ({ page }) => {
    const fix = liveFixture()
    await openBuyerTender(page, fix.createEventId, fix.createRfxNumber, 'logist')
    await expect(page.getByTestId('buyer-late-submission-queue')).toBeVisible()
    await expect(page.getByTestId('buyer-late-readonly')).toBeVisible()
    await expect(page.getByTestId('buyer-late-approve')).toHaveCount(0)
    const queue = await page.request.get(
      `${fix.gatewayURL}/api/v1/rfx-events/${fix.createEventId}/late-submission-requests`,
      { headers: await authHeaders(fix.logistJwt, fix.buyerCompanyId) },
    )
    expect(queue.ok(), `logist queue status=${queue.status()}`).toBeTruthy()
    const forbidden = await page.request.post(
      `${fix.gatewayURL}/api/v1/rfx-events/${fix.createEventId}/late-submission-requests/dddddddd-dddd-4ddd-8ddd-dddddddddddd/approve`,
      {
        headers: {
          ...await authHeaders(fix.logistJwt, fix.buyerCompanyId),
          'Content-Type': 'application/json',
          'Idempotency-Key': `late-approve-logist-${Date.now()}`,
        },
        data: {
          expected_version: 1,
          approved_valid_from: new Date().toISOString(),
          approved_valid_until: new Date(Date.now() + 3600_000).toISOString(),
        },
      },
    )
    expect(forbidden.status()).toBe(403)
  })
})
