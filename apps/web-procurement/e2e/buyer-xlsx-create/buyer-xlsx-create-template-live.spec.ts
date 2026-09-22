import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'

import { expect, test, type Download, type Page, type Request, type Response } from '@playwright/test'
import {
  assertLivePreviewMultipart,
  attachCreateNetworkProbe,
  authHeaders,
  buyerCompanyId,
  buyerJwt,
  buyerUserId,
  carrierCompanyId,
  carrierJwt,
  carrierUserId,
  expectNoProductLeaks,
  fillCreateMetadata,
  flagOffGatewayURL,
  flagOffWebURL,
  formatCreateNetworkProbe,
  gatewayURL,
  logistJwt,
  logistUserId,
  seedCreateSession,
  sourceEventId,
  templateFilename,
  templateMime,
  templatePath,
  webURL,
  workbookPath,
  type CreateNetworkProbe,
} from './helpers'

const downloadButton = '[data-testid="buyer-xlsx-create-template-download"]'

async function openCreatePage(page: Page, locale: 'ru-RU' | 'en-US' | 'zh-CN' = 'en-US') {
  await seedCreateSession(page, {
    token: buyerJwt,
    user: buyerUserId,
    company: buyerCompanyId,
    roles: ['PROCUREMENT_MANAGER'],
    locale,
  })
  await page.goto(`${webURL}/tenders/new-from-xlsx`, { waitUntil: 'domcontentloaded' })
  await expect(page.getByTestId('buyer-xlsx-create-panel')).toBeVisible({ timeout: 30_000 })
}

function templateGets(probe: CreateNetworkProbe): string[] {
  return probe.request.filter((line) => line.startsWith('GET ') && line.includes(templatePath))
}

function assertTemplateRequest(request: Request) {
  const url = new URL(request.url())
  expect(request.method()).toBe('GET')
  expect(url.pathname).toBe(templatePath)
  expect(url.search).toBe('')
  expect(url.searchParams.has('tenant_id')).toBe(false)
  expect(url.searchParams.has('owner_company_id')).toBe(false)
  expect(request.postData()).toBeNull()
  expect(request.headers()['idempotency-key']).toBeUndefined()
}

function assertTemplateResponse(response: Response) {
  expect(response.status()).toBe(200)
  const headers = response.headers()
  expect(headers['content-type'] || '').toContain(templateMime)
  expect(headers['content-disposition'] || '').toContain('attachment')
  expect(headers['content-disposition'] || '').toContain(templateFilename)
  expect(headers['cache-control'] || '').toContain('no-store')
  expect(headers['x-content-type-options'] || '').toBe('nosniff')
}

async function saveDownload(download: Download): Promise<string> {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'f5-w3-template-'))
  const saved = path.join(dir, download.suggestedFilename())
  await download.saveAs(saved)
  const bytes = fs.readFileSync(saved)
  expect(bytes.length).toBeGreaterThan(4)
  expect(Array.from(bytes.subarray(0, 4))).toEqual([0x50, 0x4b, 0x03, 0x04])
  return saved
}

async function armBusyObserver(page: Page) {
  await page.evaluate(() => {
    const el = document.querySelector('[data-testid="buyer-xlsx-create-template-download"]')
    const state = { seen: el?.getAttribute('aria-busy') === 'true' }
    const target = window as Window & { __w3TemplateBusy?: { seen: boolean } }
    target.__w3TemplateBusy = state
    if (!el) return
    const observer = new MutationObserver(() => {
      if (el.getAttribute('aria-busy') === 'true') state.seen = true
    })
    observer.observe(el, { attributes: true, attributeFilter: ['aria-busy'] })
  })
}

async function expectBusySeen(page: Page) {
  const seen = await page.evaluate(() => {
    const target = window as Window & { __w3TemplateBusy?: { seen: boolean } }
    return target.__w3TemplateBusy?.seen === true
  })
  expect(seen).toBe(true)
}

async function downloadBlankTemplate(page: Page) {
  const probe = attachCreateNetworkProbe(page)
  const button = page.locator(downloadButton)
  await expect(button).toBeVisible()
  await expect(button).toBeEnabled()
  await armBusyObserver(page)
  const responsePromise = page.waitForResponse((resp) => {
    return resp.request().method() === 'GET' && new URL(resp.url()).pathname === templatePath
  }, { timeout: 30_000 })
  const downloadPromise = page.waitForEvent('download')
  await button.evaluate((el: HTMLButtonElement) => {
    el.click()
    el.click()
  })
  const [response, download] = await Promise.all([responsePromise, downloadPromise])
  await expectBusySeen(page)
  assertTemplateRequest(response.request())
  assertTemplateResponse(response)
  expect(download.suggestedFilename()).toBe(templateFilename)
  const saved = await saveDownload(download)
  expect(templateGets(probe)).toHaveLength(1)
  return { probe, saved }
}

test.describe('buyer XLSX blank template live download', () => {
  test('blank template download writes nothing', async ({ page }) => {
    const probe = attachCreateNetworkProbe(page)
    await openCreatePage(page)
    const fileCount = await page.getByTestId('buyer-xlsx-create-file').evaluate((el: HTMLInputElement) => el.files?.length ?? 0)
    expect(fileCount).toBe(0)
    const button = page.getByTestId('buyer-xlsx-create-template-download')
    await expect(button).toBeVisible()
    await expect(button).toHaveAttribute('type', 'button')
    await button.focus()
    await expect(button).toBeFocused()
    await expect(button).toHaveAccessibleName('Download blank template')
    await expect(page.locator('body')).not.toContainText('tenders.buyerXlsxCreate.template')

    await armBusyObserver(page)
    const responsePromise = page.waitForResponse((resp) => {
      return resp.request().method() === 'GET' && new URL(resp.url()).pathname === templatePath
    }, { timeout: 30_000 })
    const downloadPromise = page.waitForEvent('download')
    await button.evaluate((el: HTMLButtonElement) => {
      el.click()
      el.click()
    })
    const response = await responsePromise
    const download = await downloadPromise
    await expectBusySeen(page)
    assertTemplateRequest(response.request())
    assertTemplateResponse(response)
    expect(download.suggestedFilename()).toBe(templateFilename)
    await saveDownload(download)
    expect(templateGets(probe)).toHaveLength(1)
    expect(probe.request.filter((line) => line.includes('/xlsx-create/preview'))).toEqual([])
    expect(probe.request.filter((line) => line.includes('/xlsx-create/commit'))).toEqual([])
    expect(probe.commitKeys).toEqual([])
    await expectNoProductLeaks(page, probe)
    await expect(page).toHaveURL(/\/tenders\/new-from-xlsx/)
    await expect(page.getByTestId('buyer-xlsx-create-template-download-status')).toContainText('Blank template downloaded')
    await expect(page.getByTestId('buyer-xlsx-create-template-download-status')).toHaveAttribute('role', 'status')
    await expect(page.getByTestId('buyer-xlsx-create-template-download-status')).toHaveAttribute('aria-live', 'polite')
    await expect(page.getByTestId('buyer-xlsx-create-template-download-error')).toHaveCount(0)
    await expect(button).toBeEnabled()
    await expect(page.getByTestId('buyer-xlsx-create-file')).toBeEnabled()

    await page.goto(`${webURL}/tenders`, { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('buyer-xlsx-create-entry')).toBeVisible()
    await expect(page.locator(downloadButton)).toHaveCount(0)

    await page.goto(`${webURL}/tenders/${sourceEventId}`, { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('buyer-xlsx-export')).toBeVisible({ timeout: 30_000 })
    await expect(page.locator(downloadButton)).toHaveCount(0)

    await page.goto(`${webURL}/tenders/new`, { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('wizard-title')).toBeVisible()
    await expect(page.locator(downloadButton)).toHaveCount(0)
  })

  test('downloaded blank template previews as a new draft without commit', async ({ page }) => {
    const probe = attachCreateNetworkProbe(page)
    await openCreatePage(page)
    const { saved } = await downloadBlankTemplate(page)
    const rfxNumber = `RFX-F5-TPL-${Date.now()}`
    const title = 'F5 blank template preview'
    await fillCreateMetadata(page, rfxNumber, title)
    await page.getByTestId('buyer-xlsx-create-file').setInputFiles(saved)
    const previewPromise = page.waitForResponse((resp) => {
      return resp.request().method() === 'POST' && resp.url().includes('/xlsx-create/preview')
    }, { timeout: 30_000 })
    await page.getByTestId('buyer-xlsx-create-preview').click()
    const preview = await previewPromise
    expect(preview.status(), formatCreateNetworkProbe(probe)).toBe(200)
    await assertLivePreviewMultipart(page, preview.request())
    const body = await preview.json() as {
      mode?: string
      ready_to_commit?: boolean
      event_id?: string
      target_event_id?: string
      analysis_id?: string
      normalized_draft_summary?: { lot_count?: number; section_count?: number; question_count?: number }
      errors?: Array<{ machine_code?: string; message_key?: string }>
      warnings?: Array<{ machine_code?: string; message_key?: string }>
    }
    expect(body.mode).toBe('CREATE_NEW_DRAFT')
    expect(body.ready_to_commit).toBe(true)
    expect(body.event_id).toBeUndefined()
    expect(body.target_event_id).toBeUndefined()
    expect(body.analysis_id).toMatch(/^[0-9a-f-]{36}$/)
    expect(body.normalized_draft_summary?.lot_count).toBe(0)
    expect(body.normalized_draft_summary?.section_count).toBe(0)
    expect(body.normalized_draft_summary?.question_count).toBe(0)
    const findingText = JSON.stringify([...(body.errors || []), ...(body.warnings || [])])
    expect(findingText).not.toContain('canonical_payload_hash')
    expect(findingText).not.toContain('=SUM(')
    expect(findingText).not.toContain('rfx.buyer_xlsx')
    await expect(page.getByTestId('buyer-xlsx-create-ready')).toHaveText(/yes/i)
    await expect(page.locator('body')).toContainText('CREATE_NEW_DRAFT')
    await expect(page).toHaveURL(/\/tenders\/new-from-xlsx/)
    expect(probe.request.filter((line) => line.includes('/xlsx-create/commit'))).toEqual([])
    await expectNoProductLeaks(page, probe)
  })

  test('download keeps the selected workbook, preview, and commit available', async ({ page }) => {
    const probe = attachCreateNetworkProbe(page)
    await openCreatePage(page)
    const rfxNumber = `RFX-F5-KEEP-${Date.now()}`
    const title = 'F5 preserve after download'
    await fillCreateMetadata(page, rfxNumber, title)
    await page.getByTestId('buyer-xlsx-create-file').setInputFiles(workbookPath)
    const previewPromise = page.waitForResponse((resp) => {
      return resp.request().method() === 'POST' && resp.url().includes('/xlsx-create/preview')
    }, { timeout: 30_000 })
    await page.getByTestId('buyer-xlsx-create-preview').click()
    expect((await previewPromise).status()).toBe(200)
    const analysisId = (await page.getByTestId('buyer-xlsx-create-analysis-id').textContent())?.trim()
    expect(analysisId).toMatch(/^[0-9a-f-]{36}$/)
    const findings = (await page.getByTestId('buyer-xlsx-create-findings').textContent()) || ''
    expect(findings.trim().length).toBeGreaterThan(0)
    await expect(page.getByTestId('buyer-xlsx-create-commit')).toBeEnabled()
    const previewCount = probe.request.filter((line) => line.includes('/xlsx-create/preview')).length

    const responsePromise = page.waitForResponse((resp) => {
      return resp.request().method() === 'GET' && new URL(resp.url()).pathname === templatePath
    }, { timeout: 30_000 })
    const downloadPromise = page.waitForEvent('download')
    await page.getByTestId('buyer-xlsx-create-template-download').click()
    const response = await responsePromise
    const download = await downloadPromise
    expect(response.status()).toBe(200)
    expect(download.suggestedFilename()).toBe(templateFilename)
    await saveDownload(download)

    await expect(page.getByTestId('buyer-xlsx-create-analysis-id')).toHaveText(analysisId || '')
    expect(await page.getByTestId('buyer-xlsx-create-findings').textContent()).toBe(findings)
    await expect(page.getByTestId('buyer-xlsx-create-rfx-number')).toHaveValue(rfxNumber)
    await expect(page.getByTestId('buyer-xlsx-create-title')).toHaveValue(title)
    await expect(page.getByTestId('buyer-xlsx-create-type')).toHaveValue('SPOT_RFQ')
    await expect(page.getByTestId('buyer-xlsx-create-category')).toHaveValue('FREIGHT')
    await expect(page.getByTestId('buyer-xlsx-create-owner')).not.toHaveValue('')
    const selectedName = await page.getByTestId('buyer-xlsx-create-file').evaluate((el: HTMLInputElement) => el.files?.[0]?.name || '')
    expect(selectedName.endsWith('.xlsx')).toBe(true)
    await expect(page.getByTestId('buyer-xlsx-create-commit')).toBeEnabled()
    expect(probe.request.filter((line) => line.includes('/xlsx-create/preview'))).toHaveLength(previewCount)
    expect(probe.request.filter((line) => line.includes('/xlsx-create/commit'))).toEqual([])
    await expect(page).toHaveURL(/\/tenders\/new-from-xlsx/)
    await expect(page.getByTestId('buyer-xlsx-create-template-download-status')).toContainText('Blank template downloaded')
  })

  test('hidden roles and flag-off cannot download the blank template', async ({ page }) => {
    await seedCreateSession(page, {
      token: logistJwt,
      user: logistUserId,
      company: buyerCompanyId,
      roles: ['SHIPPER_LOGIST'],
    })
    await page.goto(`${webURL}/tenders/new-from-xlsx`, { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('buyer-xlsx-create-unavailable')).toBeVisible({ timeout: 30_000 })
    await expect(page.locator(downloadButton)).toHaveCount(0)
    const logistGet = await page.request.get(`${gatewayURL}${templatePath}`, {
      headers: authHeaders(logistJwt, buyerCompanyId),
    })
    expect(logistGet.status()).toBe(403)

    await seedCreateSession(page, {
      token: carrierJwt,
      user: carrierUserId,
      company: carrierCompanyId,
      roles: ['CARRIER_DISPATCHER'],
    })
    await page.goto(`${webURL}/tenders/new-from-xlsx`, { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('buyer-xlsx-create-unavailable')).toBeVisible()
    await expect(page.locator(downloadButton)).toHaveCount(0)
    const carrierGet = await page.request.get(`${gatewayURL}${templatePath}`, {
      headers: authHeaders(carrierJwt, carrierCompanyId),
    })
    expect(carrierGet.status()).toBe(403)

    await seedCreateSession(page, {
      token: buyerJwt,
      user: buyerUserId,
      company: buyerCompanyId,
      roles: ['PROCUREMENT_MANAGER'],
      excelFlag: false,
    })
    await page.goto(`${flagOffWebURL}/tenders/new-from-xlsx`, { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('buyer-xlsx-create-unavailable')).toBeVisible()
    await expect(page.locator(downloadButton)).toHaveCount(0)
    const flagOffGet = await page.request.get(`${flagOffGatewayURL}${templatePath}`, {
      headers: authHeaders(buyerJwt, buyerCompanyId),
    })
    expect(flagOffGet.status()).toBe(404)

    const anonymous = await page.request.get(`${gatewayURL}${templatePath}`)
    expect(anonymous.status()).toBe(401)
  })

  test('blank template button is localized in Russian and Chinese', async ({ page }) => {
    await openCreatePage(page, 'ru-RU')
    await expect(page.getByTestId('buyer-xlsx-create-template-download')).toHaveAccessibleName('Скачать пустой шаблон')
    await expect(page.locator('body')).toContainText('Скачать пустой шаблон')
    await expect(page.locator('body')).not.toContainText('tenders.buyerXlsxCreate.template')

    await openCreatePage(page, 'zh-CN')
    await expect(page.getByTestId('buyer-xlsx-create-template-download')).toHaveAccessibleName('下载空白模板')
    await expect(page.locator('body')).toContainText('下载空白模板')
    await expect(page.locator('body')).not.toContainText('tenders.buyerXlsxCreate.template')
  })
})
