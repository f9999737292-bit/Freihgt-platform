import { expect, test } from '@playwright/test'
import {
  analysisId,
  eventId,
  expectPanelVisible,
  expectWorkspaceLoaded,
  fulfillJSON,
  readyPreviewBody,
  seedBuyerSession,
  stubTenderWorkspace,
  withBuyerXlsxCORS,
} from './helpers'

test.describe('buyer XLSX update draft', () => {
  test('hides the panel when the public flag is off or the event is not DRAFT', async ({ page }) => {
    await seedBuyerSession(page)
    await stubTenderWorkspace(page, 'PUBLISHED')
    await page.goto(`/tenders/${eventId}`, { waitUntil: 'domcontentloaded' })
    await expectWorkspaceLoaded(page, 'RFX-XLSX-1')
    await expect(page.getByTestId('buyer-xlsx-panel')).toHaveCount(0)
  })

  test('export, preview, and commit use accepted human routes', async ({ page }) => {
    const seen: string[] = []
    let commitKey = ''

    await seedBuyerSession(page)
    await stubTenderWorkspace(page)
    await page.route(`**/api/v1/rfx-events/${eventId}/xlsx-export`, async (route) => {
      await withBuyerXlsxCORS(route, async (route) => {
        seen.push(`${route.request().method()} ${new URL(route.request().url()).pathname}`)
        expect(route.request().headers().authorization).toMatch(/^Bearer /)
        await route.fulfill({
          status: 200,
          contentType: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
          headers: {
            'content-disposition': 'attachment; filename="draft.xlsx"',
            'Access-Control-Allow-Origin': '*',
          },
          body: 'xlsx',
        })
      })
    })
    await page.route(`**/api/v1/rfx-events/${eventId}/xlsx-import/preview`, async (route) => {
      await withBuyerXlsxCORS(route, async (route) => {
        seen.push(`${route.request().method()} ${new URL(route.request().url()).pathname}`)
        expect(route.request().headers()['content-type'] || '').toContain('multipart/form-data')
        await fulfillJSON(route, 200, readyPreviewBody())
      })
    })
    await page.route(`**/api/v1/rfx-events/${eventId}/xlsx-import/commit`, async (route) => {
      await withBuyerXlsxCORS(route, async (route) => {
        seen.push(`${route.request().method()} ${new URL(route.request().url()).pathname}`)
        commitKey = route.request().headers()['idempotency-key'] || ''
        expect(JSON.parse(route.request().postData() || '{}')).toEqual({ analysis_id: analysisId })
        await fulfillJSON(route, 200, {
          event_id: eventId,
          draft_version_id: '22222222-2222-4222-8222-222222222222',
          analysis_id: analysisId,
          event_version: 2,
          draft_version: 2,
          committed_at: '2026-09-20T06:00:00Z',
          changes: { lots: { added: 0, updated: 1, deleted: 0 } },
        })
      })
    })

    await page.goto(`/tenders/${eventId}`, { waitUntil: 'domcontentloaded' })
    await expectWorkspaceLoaded(page, 'RFX-XLSX-1')
    await expectPanelVisible(page)
    await page.getByTestId('buyer-xlsx-export').click()
    await expect(page.getByTestId('buyer-xlsx-commit')).toBeDisabled()

    const fileInput = page.getByTestId('buyer-xlsx-file')
    await fileInput.setInputFiles({
      name: 'draft.xlsx',
      mimeType: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
      buffer: Buffer.from('xlsx'),
    })
    await expect(page.getByTestId('buyer-xlsx-preview')).toBeVisible()
    await expect(page.getByTestId('buyer-xlsx-commit')).toBeEnabled()
    await page.getByTestId('buyer-xlsx-commit').click()
    await expect(page.getByTestId('buyer-xlsx-committed')).toBeVisible()
    await expect(page.getByTestId('buyer-xlsx-commit')).toBeDisabled()

    expect(seen).toContain(`GET /api/v1/rfx-events/${eventId}/xlsx-export`)
    expect(seen).toContain(`POST /api/v1/rfx-events/${eventId}/xlsx-import/preview`)
    expect(seen).toContain(`POST /api/v1/rfx-events/${eventId}/xlsx-import/commit`)
    expect(commitKey).toBe(`buyer-xlsx-commit:${analysisId}`)
  })

  test('422 preview keeps commit disabled and 409 stale offers retry', async ({ page }) => {
    await seedBuyerSession(page)
    await stubTenderWorkspace(page)

    await page.route(`**/api/v1/rfx-events/${eventId}/xlsx-import/preview`, async (route) => {
      await fulfillJSON(route, 422, {
        ...readyPreviewBody(),
        ready_to_commit: false,
        analysis_id: undefined,
        errors: [{ severity: 'error', machine_code: 'missing_lot', message_key: 'rfx.buyer_xlsx.missing_lot' }],
        summary: { errors: 1, warnings: 0 },
      })
    })

    await page.goto(`/tenders/${eventId}`, { waitUntil: 'domcontentloaded' })
    await expectWorkspaceLoaded(page, 'RFX-XLSX-1')
    await expectPanelVisible(page)
    await page.getByTestId('buyer-xlsx-file').setInputFiles({
      name: 'bad.xlsx',
      mimeType: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
      buffer: Buffer.from('xlsx'),
    })
    await expect(page.getByTestId('buyer-xlsx-error')).toHaveAttribute('data-error-kind', 'preview_invalid')
    await expect(page.getByTestId('buyer-xlsx-issue-text')).toHaveText('A lot is missing required data.')
    await expect(page.getByTestId('buyer-xlsx-issue-text')).not.toContainText('rfx.buyer_xlsx')
    await expect(page.getByTestId('buyer-xlsx-commit')).toBeDisabled()

    await page.unroute(`**/api/v1/rfx-events/${eventId}/xlsx-import/commit`)
    await page.route(`**/api/v1/rfx-events/${eventId}/xlsx-import/preview`, async (route) => {
      await fulfillJSON(route, 200, readyPreviewBody())
    })
    await page.route(`**/api/v1/rfx-events/${eventId}/xlsx-import/commit`, async (route) => {
      await fulfillJSON(route, 409, {
        error: { code: 'CONFLICT', message: 'stale', details: { machine_code: 'stale_target' } },
      })
    })
    await page.getByTestId('buyer-xlsx-file').setInputFiles({
      name: 'ok.xlsx',
      mimeType: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
      buffer: Buffer.from('xlsx'),
    })
    await expect(page.getByTestId('buyer-xlsx-commit')).toBeEnabled()
    await page.getByTestId('buyer-xlsx-commit').click()
    await expect(page.getByTestId('buyer-xlsx-error')).toHaveAttribute('data-error-kind', 'stale_target')
    await expect(page.getByTestId('buyer-xlsx-retry-preview')).toBeVisible()
    await expect(page.getByTestId('buyer-xlsx-retry-preview')).toBeEnabled()
    await expect(page.getByTestId('buyer-xlsx-commit')).toBeDisabled()
  })
})
