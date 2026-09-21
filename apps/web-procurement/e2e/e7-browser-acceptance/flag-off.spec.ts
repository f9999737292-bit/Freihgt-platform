import { expect, test } from '@playwright/test'
import {
  authHeaders,
  buyerCompanyId,
  buyerJwt,
  buyerUserId,
  carrierCompanyId,
  carrierJwt,
  carrierUserId,
  flagOffGatewayURL,
  flagOffLateEventId,
  flagOffWebURL,
  flagOffXlsxEventId,
  seedProcurementSession,
} from './helpers'

test.describe('E7 flag-off scoped stack', () => {
  test('UI hides XLSX and late controls and mutating POSTs return 404', async ({ page }) => {
    await seedProcurementSession(page, {
      token: buyerJwt,
      user: buyerUserId,
      company: buyerCompanyId,
      roles: ['PROCUREMENT_MANAGER'],
    })
    await page.goto(`${flagOffWebURL}/tenders/${flagOffXlsxEventId}`, { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('tender-rfx-number')).toBeVisible({ timeout: 30_000 })
    await expect(page.getByTestId('buyer-xlsx-panel')).toHaveCount(0)
    await expect(page.getByTestId('buyer-xlsx-export')).toHaveCount(0)
    await expect(page.getByTestId('buyer-late-submission-queue')).toHaveCount(0)
    await expect(page.getByTestId('buyer-late-approve')).toHaveCount(0)

    const xlsxPreview = await page.request.post(
      `${flagOffGatewayURL}/api/v1/rfx-events/${flagOffXlsxEventId}/xlsx-import/preview`,
      {
        headers: {
          ...authHeaders(buyerJwt, buyerCompanyId),
          'Content-Type': 'application/octet-stream',
        },
        data: Buffer.from('not-a-xlsx'),
      },
    )
    expect(xlsxPreview.status(), `flag-off xlsx preview ${xlsxPreview.status()}`).toBe(404)

    await seedProcurementSession(page, {
      token: carrierJwt,
      user: carrierUserId,
      company: carrierCompanyId,
      roles: ['CARRIER_DISPATCHER'],
    })
    await page.goto(`${flagOffWebURL}/carrier/tenders/${flagOffLateEventId}`, { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('carrier-xlsx-panel')).toHaveCount(0)
    await expect(page.getByTestId('carrier-late-submission-panel')).toHaveCount(0)

    const lateCreate = await page.request.post(
      `${flagOffGatewayURL}/api/v1/rfx-events/${flagOffLateEventId}/late-submission-requests?carrier_company_id=${carrierCompanyId}`,
      {
        headers: {
          ...authHeaders(carrierJwt, carrierCompanyId),
          'Content-Type': 'application/json',
          'Idempotency-Key': `late-create-flag-off-${Date.now()}`,
        },
        data: {
          reason_code: 'TECHNICAL_FAILURE',
          reason_text: 'flag-off must not persist',
          requested_until: new Date(Date.now() + 86400_000).toISOString(),
        },
      },
    )
    expect(lateCreate.status(), `flag-off late create ${lateCreate.status()}`).toBe(404)
  })
})
