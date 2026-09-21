import { expect, test, type Page } from '@playwright/test'
import { readFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import {
  authHeaders,
  carrierCompanyId,
  carrierJwt,
  carrierUserId,
  carrierXlsxEventId,
  carrierXlsxResponseId,
  carrierXlsxRfxNumber,
  competitorAnswer,
  competitorName,
  competitorOffer,
  gatewayURL,
  procurementURL,
  seedProcurementSession,
} from './helpers'

async function openCarrierDraft(page: Page) {
  await seedProcurementSession(page, {
    token: carrierJwt,
    user: carrierUserId,
    company: carrierCompanyId,
    roles: ['CARRIER_DISPATCHER'],
  })
  await page.goto(`${procurementURL}/carrier/tenders/${carrierXlsxEventId}`, { waitUntil: 'domcontentloaded' })
  await expect(page.getByTestId('carrier-tender-response-status')).toBeVisible({ timeout: 30_000 })
  await expect(page.getByTestId('carrier-tender-response-status')).toContainText(/Draft|Черновик|草稿/i)
  await expect(page.getByTestId('carrier-xlsx-panel')).toBeVisible({ timeout: 30_000 })
}

async function exportWorkbook(page: Page) {
  const exportWait = page.waitForResponse((resp) => {
    return resp.url().includes(`/carrier-responses/${carrierXlsxResponseId}/xlsx-export`)
      && resp.request().method() === 'GET'
  }, { timeout: 30_000 })
  const [download, resp] = await Promise.all([
    page.waitForEvent('download', { timeout: 30_000 }),
    exportWait,
    page.getByTestId('carrier-xlsx-export').click(),
  ])
  expect(resp.ok(), `carrier export ${resp.status()}`).toBeTruthy()
  const filePath = join(tmpdir(), `e7-carrier-xlsx-${carrierXlsxEventId}.xlsx`)
  await download.saveAs(filePath)
  const bytes = readFileSync(filePath)
  expect(bytes.subarray(0, 2).toString()).toBe('PK')
  const text = bytes.toString('utf8')
  expect(text).not.toContain(competitorOffer)
  expect(text).not.toContain(competitorName)
  expect(text).not.toContain(competitorAnswer)
  return filePath
}

test.describe('E7 carrier XLSX live', () => {
  test('export, preview, and commit keep own response in DRAFT without submit', async ({ page }) => {
    await openCarrierDraft(page)
    const workbook = await exportWorkbook(page)
    await page.getByTestId('carrier-xlsx-file').setInputFiles(workbook)
    await expect(page.getByTestId('carrier-xlsx-preview')).toBeVisible()
    const submitCalls: string[] = []
    page.on('request', (req) => {
      if (req.url().includes('/submit')) {
        submitCalls.push(`${req.method()} ${req.url()}`)
      }
    })
    await page.getByTestId('carrier-xlsx-commit').click()
    await expect(page.getByTestId('carrier-xlsx-committed')).toBeVisible()
    expect(submitCalls).toEqual([])
    await expect(page.getByTestId('carrier-tender-response-status')).toContainText(/Draft|Черновик|草稿/i)
    const own = await page.request.get(
      `${gatewayURL}/api/v1/rfx-events/${carrierXlsxEventId}/own-response?carrier_company_id=${carrierCompanyId}`,
      { headers: authHeaders(carrierJwt, carrierCompanyId) },
    )
    expect(own.ok()).toBeTruthy()
    const body = await own.json() as { status?: string; id?: string }
    expect(body.id).toBe(carrierXlsxResponseId)
    expect(body.status).toBe('DRAFT')
  })
})
