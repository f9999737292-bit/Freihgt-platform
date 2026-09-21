import { expect, test, type Page } from '@playwright/test'
import { writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import {
  authHeaders,
  buyerCompanyId,
  buyerJwt,
  buyerUserId,
  buyerXlsxEventId,
  buyerXlsxRfxNumber,
  expectWorkspace,
  gatewayURL,
  procurementURL,
  seedProcurementSession,
} from './helpers'

async function openDraft(page: Page) {
  await seedProcurementSession(page, {
    token: buyerJwt,
    user: buyerUserId,
    company: buyerCompanyId,
    roles: ['PROCUREMENT_MANAGER'],
  })
  await page.goto(`${procurementURL}/tenders/${buyerXlsxEventId}`, { waitUntil: 'domcontentloaded' })
  await expectWorkspace(page, buyerXlsxRfxNumber)
  await expect(page.getByTestId('buyer-xlsx-panel')).toBeVisible()
}

async function exportWorkbook(page: Page) {
  const respPromise = page.waitForResponse((resp) => {
    return resp.url().includes(`/rfx-events/${buyerXlsxEventId}/xlsx-export`) && resp.request().method() === 'GET'
  }, { timeout: 30_000 })
  await page.getByTestId('buyer-xlsx-export').click()
  const resp = await respPromise
  expect(resp.ok(), `export ${resp.status()}`).toBeTruthy()
  const liveExport = await page.request.get(`${gatewayURL}/api/v1/rfx-events/${buyerXlsxEventId}/xlsx-export`, {
    headers: authHeaders(buyerJwt, buyerCompanyId),
  })
  expect(liveExport.ok()).toBeTruthy()
  const bytes = Buffer.from(await liveExport.body())
  expect(bytes.subarray(0, 2).toString()).toBe('PK')
  const filePath = join(tmpdir(), `e7-buyer-xlsx-${buyerXlsxEventId}.xlsx`)
  writeFileSync(filePath, bytes)
  return filePath
}

test.describe('E7 buyer XLSX live', () => {
  test('422 preview keeps Commit disabled', async ({ page }) => {
    await openDraft(page)
    await page.getByTestId('buyer-xlsx-file').setInputFiles({
      name: 'bad.xlsx',
      mimeType: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
      buffer: Buffer.from('not-a-xlsx'),
    })
    await expect(page.getByTestId('buyer-xlsx-error')).toBeVisible()
    await expect(page.getByTestId('buyer-xlsx-error')).toHaveAttribute('data-error-kind', /preview_invalid|validation/)
    await expect(page.getByTestId('buyer-xlsx-commit')).toBeDisabled()
  })

  test('409 stale disables Commit until a new Preview', async ({ page }) => {
    await openDraft(page)
    const workbook = await exportWorkbook(page)
    await page.getByTestId('buyer-xlsx-file').setInputFiles(workbook)
    await expect(page.getByTestId('buyer-xlsx-preview')).toBeVisible()
    await expect(page.getByTestId('buyer-xlsx-commit')).toBeEnabled()
    const mutate = await page.request.patch(`${gatewayURL}/api/v1/rfx-events/${buyerXlsxEventId}`, {
      headers: {
        ...authHeaders(buyerJwt, buyerCompanyId),
        'Content-Type': 'application/json',
      },
      data: { title: `E7 stale ${Date.now()}` },
    })
    expect(mutate.ok()).toBeTruthy()
    await page.getByTestId('buyer-xlsx-commit').click()
    await expect(page.getByTestId('buyer-xlsx-error')).toHaveAttribute('data-error-kind', 'stale_target')
    await expect(page.getByTestId('buyer-xlsx-commit')).toBeDisabled()
    await page.getByTestId('buyer-xlsx-retry-preview').click()
    await expect(page.getByTestId('buyer-xlsx-preview')).toBeVisible()
    await expect(page.getByTestId('buyer-xlsx-commit')).toBeEnabled()
  })

  test('export, preview, and commit update the same DRAFT', async ({ page }) => {
    await openDraft(page)
    const workbook = await exportWorkbook(page)
    await page.getByTestId('buyer-xlsx-file').setInputFiles(workbook)
    await expect(page.getByTestId('buyer-xlsx-preview')).toBeVisible()
    const commitKeys: string[] = []
    page.on('request', (req) => {
      if (req.method() === 'POST' && req.url().includes(`/rfx-events/${buyerXlsxEventId}/xlsx-import/commit`)) {
        commitKeys.push(req.headers()['idempotency-key'] || '')
      }
    })
    await page.getByTestId('buyer-xlsx-commit').click()
    await expect(page.getByTestId('buyer-xlsx-committed')).toBeVisible()
    await expect(page.getByTestId('buyer-xlsx-commit')).toBeDisabled()
    expect(commitKeys.length).toBe(1)
  })
})
