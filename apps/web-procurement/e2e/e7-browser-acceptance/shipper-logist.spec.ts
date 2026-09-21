import { expect, test } from '@playwright/test'
import {
  authHeaders,
  buyerCompanyId,
  gatewayURL,
  lateCreateEventId,
  lateCreateRfxNumber,
  logistJwt,
  logistUserId,
  procurementURL,
  seedProcurementSession,
} from './helpers'

test('E7 SHIPPER_LOGIST sees the late queue but cannot approve or reject', async ({ page }) => {
  await seedProcurementSession(page, {
    token: logistJwt,
    user: logistUserId,
    company: buyerCompanyId,
    roles: ['SHIPPER_LOGIST'],
  })
  await page.goto(`${procurementURL}/tenders/${lateCreateEventId}`, { waitUntil: 'domcontentloaded' })
  await expect(page.getByTestId('tender-rfx-number')).toHaveText(lateCreateRfxNumber, { timeout: 30_000 })
  await expect(page.getByTestId('buyer-late-submission-queue')).toBeVisible()
  await expect(page.getByTestId('buyer-late-readonly')).toBeVisible()
  await expect(page.getByTestId('buyer-late-approve')).toHaveCount(0)
  await expect(page.getByTestId('buyer-late-reject')).toHaveCount(0)

  const queue = await page.request.get(
    `${gatewayURL}/api/v1/rfx-events/${lateCreateEventId}/late-submission-requests`,
    { headers: authHeaders(logistJwt, buyerCompanyId) },
  )
  expect(queue.ok(), `logist queue ${queue.status()}`).toBeTruthy()

  const forbidden = await page.request.post(
    `${gatewayURL}/api/v1/rfx-events/${lateCreateEventId}/late-submission-requests/dddddddd-dddd-4ddd-8ddd-dddddddddddd/approve`,
    {
      headers: {
        ...authHeaders(logistJwt, buyerCompanyId),
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

  const reject = await page.request.post(
    `${gatewayURL}/api/v1/rfx-events/${lateCreateEventId}/late-submission-requests/dddddddd-dddd-4ddd-8ddd-dddddddddddd/reject`,
    {
      headers: {
        ...authHeaders(logistJwt, buyerCompanyId),
        'Content-Type': 'application/json',
        'Idempotency-Key': `late-reject-logist-${Date.now()}`,
      },
      data: { expected_version: 1, decision_comment: 'logist cannot decide' },
    },
  )
  expect(reject.status()).toBe(403)
})
