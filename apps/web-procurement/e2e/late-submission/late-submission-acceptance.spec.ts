import { expect, test } from '@playwright/test'
import {
  attachLateSubmissionNetworkProbe,
  eventId,
  expectBuyerWorkspaceLoaded,
  expectCarrierWorkspaceLoaded,
  formatLateSubmissionNetworkProbe,
  fulfillJSON,
  lateRequestBody,
  requestId,
  seedBuyerSession,
  seedCarrierSession,
  stubBuyerTenderShell,
  stubCarrierTenderShell,
  stubQuestionnaireWorkspace,
  workspaceBody,
} from './helpers'

test.describe('late submission stub UI', () => {
  test('hides the carrier panel before deadline and commercial submit after deadline', async ({ page }) => {
    await seedCarrierSession(page)
    await stubCarrierTenderShell(page, { deadline: '2099-01-01T00:00:00Z' })
    await page.route(`**/api/v1/rfx-events/${eventId}/late-submission-requests/mine**`, async (route) => {
      await fulfillJSON(route, 200, { items: [] })
    })
    await page.goto(`/carrier/tenders/${eventId}`, { waitUntil: 'domcontentloaded' })
    await expectCarrierWorkspaceLoaded(page)
    await expect(page.getByTestId('carrier-late-submission-panel')).toHaveCount(0)
    await expect(page.getByTestId('carrier-submit-response')).toBeVisible()

    await page.unroute(`**/api/v1/carrier/rfx-events/${eventId}**`)
    await page.unroute(`**/api/v1/rfx-events/${eventId}/own-response**`)
    await stubCarrierTenderShell(page, { deadline: '2020-01-01T00:00:00Z' })
    await page.reload({ waitUntil: 'domcontentloaded' })
    await expectCarrierWorkspaceLoaded(page)
    await expect(page.getByTestId('carrier-late-submission-panel')).toBeVisible()
    await expect(page.getByTestId('carrier-late-commercial-locked')).toBeVisible()
    await expect(page.getByTestId('carrier-submit-response')).toHaveCount(0)
  })

  test('creates a carrier request through GET /mine with Idempotency-Key', async ({ page }) => {
    const probe = attachLateSubmissionNetworkProbe(page)
    const seen: string[] = []
    const keys: string[] = []
    await seedCarrierSession(page)
    await stubCarrierTenderShell(page)
    await page.route(`**/api/v1/rfx-events/${eventId}/late-submission-requests/mine**`, async (route) => {
      seen.push(`${route.request().method()} ${new URL(route.request().url()).pathname}`)
      await fulfillJSON(route, 200, { items: [] })
    })
    await page.route(`**/api/v1/rfx-events/${eventId}/late-submission-requests**`, async (route) => {
      const url = new URL(route.request().url())
      if (route.request().method() !== 'POST' || url.pathname.endsWith('/mine')) {
        await route.fallback()
        return
      }
      seen.push(`${route.request().method()} ${url.pathname}`)
      keys.push(route.request().headers()['idempotency-key'] || '')
      expect(url.searchParams.get('carrier_company_id')).toBeTruthy()
      await fulfillJSON(route, 201, lateRequestBody())
    })

    await page.goto(`/carrier/tenders/${eventId}`, { waitUntil: 'domcontentloaded' })
    await expectCarrierWorkspaceLoaded(page)
    await expect(page.getByTestId('carrier-late-submission-panel')).toBeVisible()
    await page.getByTestId('carrier-late-reason-text').fill('VPN outage blocked the draft submit')
    await page.getByTestId('carrier-late-request-submit').click()
    await expect(page.getByTestId('carrier-late-request-status')).toContainText(/requested/i)
    expect(seen, formatLateSubmissionNetworkProbe(probe)).toContain(
      `GET /api/v1/rfx-events/${eventId}/late-submission-requests/mine`,
    )
    expect(seen).toContain(`POST /api/v1/rfx-events/${eventId}/late-submission-requests`)
    expect(keys).toHaveLength(1)
    expect(keys[0]).toMatch(/^late-create:/)
    expect(keys[0].length).toBeLessThanOrEqual(128)
  })

  test('after REJECTED a new create attempt uses a new Idempotency-Key and retries reuse it', async ({ page }) => {
    const firstKeys: string[] = []
    const retryKeys: string[] = []
    const rejected = lateRequestBody({ status: 'REJECTED', version: 2, updated_at: '2026-09-20T11:00:00Z' })
    await seedCarrierSession(page)
    await stubCarrierTenderShell(page)
    await page.route(`**/api/v1/rfx-events/${eventId}/late-submission-requests/mine**`, async (route) => {
      await fulfillJSON(route, 200, { items: [] })
    })
    await page.route(`**/api/v1/rfx-events/${eventId}/late-submission-requests**`, async (route) => {
      const url = new URL(route.request().url())
      if (route.request().method() !== 'POST' || url.pathname.endsWith('/mine')) {
        await route.fallback()
        return
      }
      firstKeys.push(route.request().headers()['idempotency-key'] || '')
      await fulfillJSON(route, 201, lateRequestBody())
    })
    await page.goto(`/carrier/tenders/${eventId}`, { waitUntil: 'domcontentloaded' })
    await expectCarrierWorkspaceLoaded(page)
    await page.getByTestId('carrier-late-reason-text').fill('first attempt after deadline')
    await page.getByTestId('carrier-late-request-submit').click()
    await expect(page.getByTestId('carrier-late-request-status')).toContainText(/requested/i)
    expect(firstKeys).toHaveLength(1)

    await page.unroute(`**/api/v1/rfx-events/${eventId}/late-submission-requests/mine**`)
    await page.unroute(`**/api/v1/rfx-events/${eventId}/late-submission-requests**`)
    await page.route(`**/api/v1/rfx-events/${eventId}/late-submission-requests/mine**`, async (route) => {
      await fulfillJSON(route, 200, { items: [rejected] })
    })
    await page.route(`**/api/v1/rfx-events/${eventId}/late-submission-requests**`, async (route) => {
      const url = new URL(route.request().url())
      if (route.request().method() !== 'POST' || url.pathname.endsWith('/mine')) {
        await route.fallback()
        return
      }
      retryKeys.push(route.request().headers()['idempotency-key'] || '')
      await fulfillJSON(route, 201, lateRequestBody({
        id: '55555555-5555-4555-8555-555555555555',
        status: 'REQUESTED',
        reason_text: 'second attempt after reject',
        updated_at: '2026-09-20T12:00:00Z',
      }))
    })
    await page.reload({ waitUntil: 'domcontentloaded' })
    await expectCarrierWorkspaceLoaded(page)
    await expect(page.getByTestId('carrier-late-request-status')).toContainText(/rejected/i)
    await expect(page.getByTestId('carrier-late-request-submit')).toBeVisible()
    await page.getByTestId('carrier-late-reason-text').fill('second attempt after reject')
    await page.getByTestId('carrier-late-request-submit').click()
    await expect(page.getByTestId('carrier-late-request-status')).toContainText(/requested/i)
    expect(retryKeys).toHaveLength(1)
    expect(retryKeys[0]).toMatch(/^late-create:/)
    expect(retryKeys[0].length).toBeLessThanOrEqual(128)
    expect(retryKeys[0]).not.toBe(firstKeys[0])
  })

  test('buyer queue approve/reject stay manage-only and SHIPPER_LOGIST is read-only', async ({ page }) => {
    const keys: string[] = []
    await seedBuyerSession(page, { roles: ['SHIPPER_LOGIST'] })
    await stubBuyerTenderShell(page)
    await page.route(`**/api/v1/rfx-events/${eventId}/late-submission-requests`, async (route) => {
      if (route.request().method() === 'GET') {
        await fulfillJSON(route, 200, { items: [lateRequestBody()] })
        return
      }
      await route.fallback()
    })
    await page.goto(`/tenders/${eventId}`, { waitUntil: 'domcontentloaded' })
    await expectBuyerWorkspaceLoaded(page)
    await expect(page.getByTestId('buyer-late-submission-queue')).toBeVisible()
    await expect(page.getByTestId('buyer-late-readonly')).toBeVisible()
    await expect(page.getByTestId('buyer-late-approve')).toHaveCount(0)
    await expect(page.getByTestId('buyer-late-reject')).toHaveCount(0)

    await seedBuyerSession(page, { roles: ['PROCUREMENT_MANAGER'] })
    await page.route(`**/api/v1/rfx-events/${eventId}/late-submission-requests/${requestId}/approve`, async (route) => {
      keys.push(route.request().headers()['idempotency-key'] || '')
      await fulfillJSON(route, 200, lateRequestBody({
        status: 'APPROVED',
        version: 2,
        approved_valid_from: '2026-09-20T11:00:00Z',
        approved_valid_until: '2026-09-21T11:00:00Z',
      }))
    })
    await page.route(`**/api/v1/rfx-events/${eventId}/late-submission-requests/${requestId}/reject`, async (route) => {
      keys.push(route.request().headers()['idempotency-key'] || '')
      await fulfillJSON(route, 200, lateRequestBody({ status: 'REJECTED', version: 2 }))
    })
    await page.reload({ waitUntil: 'domcontentloaded' })
    await expectBuyerWorkspaceLoaded(page)
    await expect(page.getByTestId('buyer-late-approve')).toBeVisible()
    await page.getByTestId('buyer-late-approve').click()
    await expect(page.getByTestId('buyer-late-status')).toContainText(/approved/i)
    expect(keys[0]).toMatch(/^late-approve:/)
  })

  test('late questionnaire submit sends Idempotency-Key and stays blocked outside the window', async ({ page }) => {
    const submitKeys: string[] = []
    await seedCarrierSession(page)
    await stubQuestionnaireWorkspace(page)
    await page.route(`**/api/v1/rfx-events/${eventId}/late-submission-requests/mine**`, async (route) => {
      await fulfillJSON(route, 200, {
        items: [lateRequestBody({
          status: 'APPROVED',
          approved_valid_from: '2099-01-01T00:00:00Z',
          approved_valid_until: '2099-01-02T00:00:00Z',
        })],
      })
    })
    await page.goto(`/carrier/tenders/${eventId}/questionnaire`, { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('carrier-response-workspace')).toBeVisible()
    await expect(page.getByTestId('late-submit-blocked')).toContainText(/has not started/i)
    await expect(page.getByTestId('submit-questionnaire')).toBeDisabled()

    await page.unroute(`**/api/v1/rfx-events/${eventId}/late-submission-requests/mine**`)
    await page.route(`**/api/v1/rfx-events/${eventId}/late-submission-requests/mine**`, async (route) => {
      await fulfillJSON(route, 200, {
        items: [lateRequestBody({
          status: 'APPROVED',
          approved_valid_from: '2020-01-01T00:00:00Z',
          approved_valid_until: '2020-01-02T00:00:00Z',
        })],
      })
    })
    await page.reload({ waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('late-submit-blocked')).toContainText(/has expired/i)
    await expect(page.getByTestId('submit-questionnaire')).toBeDisabled()

    await page.unroute(`**/api/v1/rfx-events/${eventId}/late-submission-requests/mine**`)
    await page.route(`**/api/v1/rfx-events/${eventId}/late-submission-requests/mine**`, async (route) => {
      await fulfillJSON(route, 200, {
        items: [lateRequestBody({
          status: 'APPROVED',
          approved_valid_from: '2020-01-01T00:00:00Z',
          approved_valid_until: '2099-01-01T00:00:00Z',
        })],
      })
    })
    await page.route(`**/api/v1/rfx-events/${eventId}/carrier-response/submit**`, async (route) => {
      const url = new URL(route.request().url())
      if (route.request().method() !== 'POST' || !url.pathname.endsWith('/carrier-response/submit')) {
        await route.fallback()
        return
      }
      submitKeys.push(route.request().headers()['idempotency-key'] || '')
      await fulfillJSON(route, 200, {
        ...workspaceBody({ product_status: 'SUBMITTED', status: 'SUBMITTED', save_version: 2 }),
        submitted_at: '2026-09-20T12:00:00Z',
      })
    })
    page.once('dialog', (dialog) => dialog.accept())
    await page.reload({ waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('submit-questionnaire')).toBeEnabled()
    await page.getByTestId('submit-questionnaire').click()
    await expect(page.getByTestId('post-submit-lock')).toBeVisible()
    expect(submitKeys).toHaveLength(1)
    expect(submitKeys[0]).toMatch(/^late-submit:/)
  })
})
