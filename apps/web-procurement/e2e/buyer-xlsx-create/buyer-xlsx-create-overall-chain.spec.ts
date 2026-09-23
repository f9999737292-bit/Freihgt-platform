import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'

import { expect, test, type Download, type Page, type Request } from '@playwright/test'
import {
  assertLivePreviewMultipart,
  attachCreateNetworkProbe,
  authHeaders,
  buyerCompanyId,
  buyerJwt,
  buyerUserId,
  expectNoProductLeaks,
  fillCreateMetadata,
  formatCreateNetworkProbe,
  gatewayURL,
  seedCreateSession,
  templateFilename,
  templateMime,
  templatePath,
  webURL,
  type CreateNetworkProbe,
} from './helpers'

const downloadButton = '[data-testid="buyer-xlsx-create-template-download"]'

function countRequests(probe: CreateNetworkProbe, method: string, pathPart: string): number {
  return probe.request.filter((line) => line.startsWith(`${method} `) && line.includes(pathPart)).length
}

function forbiddenMutations(probe: CreateNetworkProbe): string[] {
  return probe.request.filter((line) => (
    /\/integrations\/erp/i.test(line)
    || /\/publish\b/i.test(line)
    || /\/submit\b/i.test(line)
    || /\/awards?\b/i.test(line)
    || /transport-orders/i.test(line)
    || (/\/participants\b/.test(line) && !line.startsWith('GET '))
  ))
}

async function openCreatePage(page: Page) {
  await seedCreateSession(page, {
    token: buyerJwt,
    user: buyerUserId,
    company: buyerCompanyId,
    roles: ['PROCUREMENT_MANAGER'],
  })
  await page.goto(`${webURL}/tenders/new-from-xlsx`, { waitUntil: 'domcontentloaded' })
  await expect(page.getByTestId('buyer-xlsx-create-panel')).toBeVisible({ timeout: 30_000 })
}

async function saveDownload(download: Download): Promise<string> {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'f5-overall-template-'))
  const saved = path.join(dir, download.suggestedFilename())
  await download.saveAs(saved)
  const bytes = fs.readFileSync(saved)
  expect(bytes.length).toBeGreaterThan(4)
  expect(Array.from(bytes.subarray(0, 4))).toEqual([0x50, 0x4b, 0x03, 0x04])
  return saved
}

function assertTemplateRequest(request: Request) {
  const url = new URL(request.url())
  expect(request.method()).toBe('GET')
  expect(url.pathname).toBe(templatePath)
  expect(url.search).toBe('')
  expect(request.postData()).toBeNull()
  expect(request.headers()['idempotency-key']).toBeUndefined()
}

test.describe('buyer XLSX create overall chain', () => {
  test('blank template download uploads previews and commits one excel draft', async ({ page }) => {
    const probe = attachCreateNetworkProbe(page)
    await openCreatePage(page)

    const responsePromise = page.waitForResponse((resp) => {
      return resp.request().method() === 'GET' && new URL(resp.url()).pathname === templatePath
    }, { timeout: 30_000 })
    const downloadPromise = page.waitForEvent('download')
    await page.locator(downloadButton).click()
    const templateResponse = await responsePromise
    const download = await downloadPromise
    expect(templateResponse.status()).toBe(200)
    expect(templateResponse.headers()['content-type'] || '').toContain(templateMime)
    assertTemplateRequest(templateResponse.request())
    expect(download.suggestedFilename()).toBe(templateFilename)
    const saved = await saveDownload(download)
    expect(countRequests(probe, 'GET', templatePath)).toBe(1)

    const rfxNumber = `RFX-F5-ALL-${Date.now()}`
    const title = 'F5 overall blank template'
    await fillCreateMetadata(page, rfxNumber, title)
    await expect(page.getByTestId('buyer-xlsx-create-owner')).toHaveValue(buyerCompanyId)
    await page.getByTestId('buyer-xlsx-create-file').setInputFiles(saved)

    const previewPromise = page.waitForResponse((resp) => {
      return resp.request().method() === 'POST' && resp.url().includes('/xlsx-create/preview')
    }, { timeout: 30_000 })
    await page.getByTestId('buyer-xlsx-create-preview').click()
    const preview = await previewPromise
    expect(preview.status(), formatCreateNetworkProbe(probe)).toBe(200)
    await assertLivePreviewMultipart(page, preview.request())
    const previewBody = await preview.json() as {
      mode?: string
      ready_to_commit?: boolean
      analysis_id?: string
      event_id?: string
      normalized_draft_summary?: { lot_count?: number; section_count?: number; question_count?: number }
    }
    expect(previewBody.mode).toBe('CREATE_NEW_DRAFT')
    expect(previewBody.ready_to_commit).toBe(true)
    expect(previewBody.event_id).toBeUndefined()
    expect(previewBody.analysis_id).toMatch(/^[0-9a-f-]{36}$/)
    expect(previewBody.normalized_draft_summary?.lot_count).toBe(0)
    expect(previewBody.normalized_draft_summary?.section_count).toBe(0)
    expect(previewBody.normalized_draft_summary?.question_count).toBe(0)
    const analysisId = previewBody.analysis_id as string
    await expect(page.getByTestId('buyer-xlsx-create-analysis-id')).toHaveText(analysisId)
    expect(countRequests(probe, 'POST', '/xlsx-create/preview')).toBe(1)

    const commitPromise = page.waitForResponse((resp) => {
      return resp.request().method() === 'POST' && resp.url().includes('/xlsx-create/commit')
    }, { timeout: 30_000 })
    await page.getByTestId('buyer-xlsx-create-commit').evaluate((el: HTMLElement) => {
      el.click()
      el.click()
    })
    const commit = await commitPromise
    expect(commit.status(), formatCreateNetworkProbe(probe)).toBe(201)
    const commitRequest = commit.request()
    const commitPayload = JSON.parse(commitRequest.postData() || '{}') as Record<string, unknown>
    expect(Object.keys(commitPayload)).toEqual(['analysis_id'])
    expect(commitPayload).toEqual({ analysis_id: analysisId })
    const idempotencyKey = commitRequest.headers()['idempotency-key']
    expect(idempotencyKey).toBe(`buyer-xlsx-create-commit:${analysisId}`)
    const commitBody = await commit.json() as {
      event_id?: string
      analysis_id?: string
      status?: string
      creation_channel?: string
      questionnaire_enabled?: boolean
      draft_version_id?: string
    }
    expect(commitBody.event_id).toMatch(/^[0-9a-f-]{36}$/)
    expect(commitBody.analysis_id).toBe(analysisId)
    expect(commitBody.status).toBe('DRAFT')
    expect(commitBody.creation_channel).toBe('EXCEL')
    expect(commitBody.questionnaire_enabled).toBe(false)
    expect(commitBody.draft_version_id).toMatch(/^[0-9a-f-]{36}$/)
    const eventId = commitBody.event_id as string

    await expect(page).toHaveURL(new RegExp(`/tenders/${eventId}$`))
    const landed = new URL(page.url())
    expect(landed.pathname).toBe(`/tenders/${eventId}`)
    expect(landed.search).toBe('')

    expect(countRequests(probe, 'GET', templatePath)).toBe(1)
    expect(countRequests(probe, 'POST', '/xlsx-create/preview')).toBe(1)
    expect(countRequests(probe, 'POST', '/xlsx-create/commit')).toBe(1)
    expect(probe.commitKeys).toEqual([idempotencyKey])
    expect(forbiddenMutations(probe), formatCreateNetworkProbe(probe)).toEqual([])

    const eventGet = await page.request.get(`${gatewayURL}/api/v1/rfx-events/${eventId}`, {
      headers: authHeaders(buyerJwt, buyerCompanyId),
    })
    expect(eventGet.status(), `created event GET ${eventGet.status()}`).toBe(200)
    const event = await eventGet.json() as {
      id?: string
      status?: string
      creation_channel?: string
      rfx_number?: string
      title?: string
      rfx_type?: string
      category?: string
      owner_company_id?: string
    }
    expect(event.id).toBe(eventId)
    expect(event.status).toBe('DRAFT')
    expect(event.creation_channel).toBe('EXCEL')
    expect(event.rfx_number).toBe(rfxNumber)
    expect(event.title).toBe(title)
    expect(event.rfx_type).toBe('SPOT_RFQ')
    expect(event.category).toBe('FREIGHT')
    expect(event.owner_company_id).toBe(buyerCompanyId)

    const participantsGet = await page.request.get(`${gatewayURL}/api/v1/rfx-events/${eventId}/participants`, {
      headers: authHeaders(buyerJwt, buyerCompanyId),
    })
    expect(participantsGet.status()).toBe(200)
    const participants = await participantsGet.json() as { items?: unknown[] }
    expect(participants.items ?? []).toEqual([])

    const questionnaireGet = await page.request.get(`${gatewayURL}/api/v1/rfx-events/${eventId}/questionnaire`, {
      headers: authHeaders(buyerJwt, buyerCompanyId),
    })
    expect(questionnaireGet.status(), `questionnaire GET ${questionnaireGet.status()}`).toBe(200)
    const questionnaire = await questionnaireGet.json() as {
      event_id?: string
      questionnaire_enabled?: boolean
      version_status?: string
    }
    expect(questionnaire.event_id).toBe(eventId)
    expect(questionnaire.questionnaire_enabled).toBe(false)
    expect(questionnaire.version_status).toBe('DRAFT')

    expect(countRequests(probe, 'POST', '/xlsx-create/preview')).toBe(1)
    expect(countRequests(probe, 'POST', '/xlsx-create/commit')).toBe(1)
    await expectNoProductLeaks(page, probe)

    const evidencePath = process.env.BROWSER_E2E_OVERALL_CHAIN_EVIDENCE
    if (evidencePath) {
      fs.writeFileSync(evidencePath, JSON.stringify({
        rfx_number: rfxNumber,
        event_id: eventId,
        analysis_id: analysisId,
        idempotency_key: idempotencyKey,
      }), { mode: 0o600 })
    }
  })
})
