import { expect, test, type Page } from '@playwright/test'
import {
  buyerCompanyId,
  buyerJwt,
  buyerUserId,
  carrierCompanyId,
  carrierJwt,
  carrierUserId,
  lateCreateEventId,
  lateExpiredEventId,
  lateNotStartedEventId,
  lateRejectEventId,
  lateRejectRfxNumber,
  lateSubmitEventId,
  lateSubmitRfxNumber,
  procurementURL,
  seedProcurementSession,
} from './helpers'

async function openCarrier(page: Page, eventId: string) {
  await seedProcurementSession(page, {
    token: carrierJwt,
    user: carrierUserId,
    company: carrierCompanyId,
    roles: ['CARRIER_DISPATCHER'],
  })
  await page.goto(`${procurementURL}/carrier/tenders/${eventId}`, { waitUntil: 'domcontentloaded' })
  await expect(page.getByTestId('carrier-late-submission-panel')).toBeVisible({ timeout: 30_000 })
}

async function openBuyer(page: Page, eventId: string, rfxNumber: string) {
  await seedProcurementSession(page, {
    token: buyerJwt,
    user: buyerUserId,
    company: buyerCompanyId,
    roles: ['PROCUREMENT_MANAGER'],
  })
  await page.goto(`${procurementURL}/tenders/${eventId}`, { waitUntil: 'domcontentloaded' })
  await expect(page.getByTestId('tender-rfx-number')).toHaveText(rfxNumber, { timeout: 30_000 })
  await expect(page.getByTestId('buyer-late-submission-queue')).toBeVisible()
}

test.describe('E7 late submission live', () => {
  test('request, approve, and submit inside the window', async ({ page }) => {
    await openCarrier(page, lateCreateEventId)
    const createWait = page.waitForResponse((resp) =>
      resp.request().method() === 'POST'
      && new URL(resp.url()).pathname === `/api/v1/rfx-events/${lateCreateEventId}/late-submission-requests`,
    )
    await page.getByTestId('carrier-late-reason-text').fill('E7 live studio outage after the deadline')
    await page.getByTestId('carrier-late-request-submit').click()
    const created = await createWait
    expect(created.status()).toBe(201)
    await expect(page.getByTestId('carrier-late-request-status')).toContainText(/requested/i)

    await openBuyer(page, lateSubmitEventId, lateSubmitRfxNumber)
    const approveWait = page.waitForResponse((resp) =>
      resp.request().method() === 'POST' && resp.url().includes('/approve'),
    )
    await page.getByTestId('buyer-late-approve').click()
    const approved = await approveWait
    expect(approved.ok(), `approve ${approved.status()}`).toBeTruthy()
    await expect(page.getByTestId('buyer-late-status')).toContainText(/approved/i)

    await seedProcurementSession(page, {
      token: carrierJwt,
      user: carrierUserId,
      company: carrierCompanyId,
      roles: ['CARRIER_DISPATCHER'],
    })
    page.once('dialog', (dialog) => dialog.accept())
    const submitWait = page.waitForResponse((resp) =>
      resp.request().method() === 'POST'
      && new URL(resp.url()).pathname === `/api/v1/rfx-events/${lateSubmitEventId}/carrier-response/submit`,
    )
    await page.goto(`${procurementURL}/carrier/tenders/${lateSubmitEventId}/questionnaire`, { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('carrier-response-workspace')).toBeVisible({ timeout: 30_000 })
    await page.getByTestId('submit-questionnaire').click()
    const submitted = await submitWait
    expect(submitted.ok(), `late submit ${submitted.status()}`).toBeTruthy()
    await expect(page.getByTestId('post-submit-lock')).toBeVisible()
  })

  test('buyer reject keeps submit blocked', async ({ page }) => {
    await openBuyer(page, lateRejectEventId, lateRejectRfxNumber)
    const rejectWait = page.waitForResponse((resp) =>
      resp.request().method() === 'POST' && resp.url().includes('/reject'),
    )
    await page.getByTestId('buyer-late-reject').click()
    expect((await rejectWait).ok()).toBeTruthy()

    await seedProcurementSession(page, {
      token: carrierJwt,
      user: carrierUserId,
      company: carrierCompanyId,
      roles: ['CARRIER_DISPATCHER'],
    })
    await page.goto(`${procurementURL}/carrier/tenders/${lateRejectEventId}/questionnaire`, { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('late-submit-blocked')).toBeVisible()
    await expect(page.getByTestId('submit-questionnaire')).toBeDisabled()
  })

  test('submit is forbidden before and after the approved window', async ({ page }) => {
    await seedProcurementSession(page, {
      token: carrierJwt,
      user: carrierUserId,
      company: carrierCompanyId,
      roles: ['CARRIER_DISPATCHER'],
    })
    await page.goto(`${procurementURL}/carrier/tenders/${lateNotStartedEventId}/questionnaire`, { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('late-submit-blocked')).toContainText(/has not started/i)
    await expect(page.getByTestId('submit-questionnaire')).toBeDisabled()

    await page.goto(`${procurementURL}/carrier/tenders/${lateExpiredEventId}/questionnaire`, { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('late-submit-blocked')).toContainText(/has expired/i)
    await expect(page.getByTestId('submit-questionnaire')).toBeDisabled()
  })
})
