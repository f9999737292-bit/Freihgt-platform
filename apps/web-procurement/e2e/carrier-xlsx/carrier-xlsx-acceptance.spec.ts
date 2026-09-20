import { expect, test } from '@playwright/test'
import {
  analysisId,
  attachCarrierXlsxNetworkProbe,
  eventId,
  expectPanelVisible,
  expectWorkspaceLoaded,
  formatCarrierXlsxNetworkProbe,
  fulfillJSON,
  readyPreviewBody,
  responseId,
  seedCarrierSession,
  stubCarrierTenderWorkspace,
  withCarrierXlsxCORS,
} from './helpers'

test.describe('carrier XLSX update draft', () => {
  test('hides the panel without a response and keeps SUBMITTED export-only', async ({ page }) => {
    await seedCarrierSession(page)
    await stubCarrierTenderWorkspace(page, { responseStatus: 'MISSING' })
    await page.goto(`/carrier/tenders/${eventId}`, { waitUntil: 'domcontentloaded' })
    await expectWorkspaceLoaded(page, 'RFX-CARRIER-XLSX-1')
    await expect(page.getByTestId('carrier-xlsx-panel')).toHaveCount(0)
    await expect(page.getByTestId('carrier-xlsx-export')).toHaveCount(0)
    await expect(page.getByTestId('carrier-xlsx-commit')).toHaveCount(0)

    await page.unroute('**/api/v1/rfx-events/**')
    await page.unroute('**/api/v1/carrier/rfx-events/**')
    await page.unroute('**/api/v1/users/**/companies**')
    await page.unroute('**/api/v1/companies**')
    await stubCarrierTenderWorkspace(page, { responseStatus: 'SUBMITTED' })
    await page.reload({ waitUntil: 'domcontentloaded' })
    await expectWorkspaceLoaded(page, 'RFX-CARRIER-XLSX-1')
    await expectPanelVisible(page, 'SUBMITTED')
    await expect(page.getByTestId('carrier-xlsx-export')).toBeVisible()
    await expect(page.getByTestId('carrier-xlsx-file')).toHaveCount(0)
    await expect(page.getByTestId('carrier-xlsx-commit')).toHaveCount(0)
  })

  test('export, preview, and commit use accepted human routes without submit', async ({ page }) => {
    const seen: string[] = []
    const commitKeys: string[] = []
    const probe = attachCarrierXlsxNetworkProbe(page)

    await seedCarrierSession(page)
    await stubCarrierTenderWorkspace(page)
    await page.route(`**/api/v1/rfx-events/${eventId}/carrier-responses/${responseId}/xlsx-export`, async (route) => {
      await withCarrierXlsxCORS(route, async (route) => {
        seen.push(`${route.request().method()} ${new URL(route.request().url()).pathname}`)
        await route.fulfill({
          status: 200,
          contentType: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
          headers: {
            'content-disposition': 'attachment; filename="carrier-draft.xlsx"',
            'Access-Control-Allow-Origin': '*',
          },
          body: 'xlsx',
        })
      })
    })
    await page.route(`**/api/v1/rfx-events/${eventId}/carrier-responses/${responseId}/xlsx-import/preview`, async (route) => {
      await withCarrierXlsxCORS(route, async (route) => {
        seen.push(`${route.request().method()} ${new URL(route.request().url()).pathname}`)
        await fulfillJSON(route, 200, readyPreviewBody())
      })
    })
    await page.route(`**/api/v1/rfx-events/${eventId}/carrier-responses/${responseId}/xlsx-import/commit`, async (route) => {
      await withCarrierXlsxCORS(route, async (route) => {
        seen.push(`${route.request().method()} ${new URL(route.request().url()).pathname}`)
        commitKeys.push(route.request().headers()['idempotency-key'] || '')
        await fulfillJSON(route, 200, {
          event_id: eventId,
          response_id: responseId,
          analysis_id: analysisId,
          save_version: 2,
          committed_at: '2026-09-20T06:00:00Z',
          changes: { offer_lines: { added: 0, updated: 1, deleted: 0 } },
        })
      })
    })

    await page.goto(`/carrier/tenders/${eventId}`, { waitUntil: 'domcontentloaded' })
    await expectWorkspaceLoaded(page, 'RFX-CARRIER-XLSX-1')
    await expectPanelVisible(page)
    await page.getByTestId('carrier-xlsx-export').click()
    await expect(page.getByTestId('carrier-xlsx-commit')).toBeDisabled()

    const fileInput = page.getByTestId('carrier-xlsx-file')
    await fileInput.setInputFiles({
      name: 'draft.xlsx',
      mimeType: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
      buffer: Buffer.from('xlsx'),
    })
    await expect(page.getByTestId('carrier-xlsx-preview')).toBeVisible()
    await expect(page.getByTestId('carrier-xlsx-commit')).toBeEnabled()
    await page.getByTestId('carrier-xlsx-commit').click()
    await expect(page.getByTestId('carrier-xlsx-committed')).toBeVisible()
    await expect(page.getByTestId('carrier-xlsx-commit')).toBeDisabled()
    await page.getByTestId('carrier-xlsx-commit').click({ force: true })
    await expect(page.getByTestId('carrier-xlsx-commit')).toBeDisabled()
    expect(commitKeys, formatCarrierXlsxNetworkProbe(probe)).toEqual([`carrier-xlsx-commit:${analysisId}`])

    expect(seen, formatCarrierXlsxNetworkProbe(probe)).toContain(
      `GET /api/v1/rfx-events/${eventId}/carrier-responses/${responseId}/xlsx-export`,
    )
    expect(seen).toContain(
      `POST /api/v1/rfx-events/${eventId}/carrier-responses/${responseId}/xlsx-import/preview`,
    )
    expect(seen).toContain(
      `POST /api/v1/rfx-events/${eventId}/carrier-responses/${responseId}/xlsx-import/commit`,
    )
    expect(seen.every((item) => !item.includes('/submit'))).toBe(true)
    expect(probe.request.every((item) => !item.includes('/submit'))).toBe(true)
    await expect(page.getByTestId('carrier-submit-response')).toBeVisible()
  })

  test('422 preview keeps commit disabled and 409 stale offers retry', async ({ page }) => {
    const probe = attachCarrierXlsxNetworkProbe(page)
    await seedCarrierSession(page)
    await stubCarrierTenderWorkspace(page)

    await page.route(`**/api/v1/rfx-events/${eventId}/carrier-responses/${responseId}/xlsx-import/preview`, async (route) => {
      await fulfillJSON(route, 422, {
        ...readyPreviewBody(),
        ready_to_commit: false,
        analysis_id: undefined,
        errors: [{ severity: 'error', machine_code: 'unknown_lot', message_key: 'rfx.carrier_xlsx_import.unknown_lot' }],
        summary: { errors: 1, warnings: 0 },
      })
    })

    await page.goto(`/carrier/tenders/${eventId}`, { waitUntil: 'domcontentloaded' })
    await expectWorkspaceLoaded(page, 'RFX-CARRIER-XLSX-1')
    await expectPanelVisible(page)
    await page.getByTestId('carrier-xlsx-file').setInputFiles({
      name: 'bad.xlsx',
      mimeType: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
      buffer: Buffer.from('xlsx'),
    })
    await expect(page.getByTestId('carrier-xlsx-error'), formatCarrierXlsxNetworkProbe(probe)).toHaveAttribute('data-error-kind', 'preview_invalid')
    await expect(page.getByTestId('carrier-xlsx-issue-text')).toHaveText('A lot number was not found.')
    await expect(page.getByTestId('carrier-xlsx-issue-text')).not.toContainText('rfx.carrier_xlsx')
    await expect(page.getByTestId('carrier-xlsx-commit')).toBeDisabled()

    await page.unroute(`**/api/v1/rfx-events/${eventId}/carrier-responses/${responseId}/xlsx-import/preview`)
    await page.route(`**/api/v1/rfx-events/${eventId}/carrier-responses/${responseId}/xlsx-import/preview`, async (route) => {
      await fulfillJSON(route, 200, readyPreviewBody())
    })
    await page.route(`**/api/v1/rfx-events/${eventId}/carrier-responses/${responseId}/xlsx-import/commit`, async (route) => {
      await fulfillJSON(route, 409, {
        error: { code: 'CONFLICT', message: 'stale', details: { machine_code: 'stale_target' } },
      })
    })
    await page.getByTestId('carrier-xlsx-file').setInputFiles({
      name: 'ok.xlsx',
      mimeType: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
      buffer: Buffer.from('xlsx'),
    })
    await expect(page.getByTestId('carrier-xlsx-commit')).toBeEnabled()
    await page.getByTestId('carrier-xlsx-commit').click()
    await expect(page.getByTestId('carrier-xlsx-error')).toHaveAttribute('data-error-kind', 'stale_target')
    await expect(page.getByTestId('carrier-xlsx-retry-preview')).toBeVisible()
    await expect(page.getByTestId('carrier-xlsx-retry-preview')).toBeEnabled()
    await expect(page.getByTestId('carrier-xlsx-commit')).toBeDisabled()
  })
})
