import { expect, test, type Page } from '@playwright/test'
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
  flagOffRfxNumber,
  flagOffWebURL,
  foreignJwt,
  foreignTenantId,
  foreignUserId,
  formatCreateNetworkProbe,
  gatewayURL,
  logistJwt,
  logistUserId,
  openTendersList,
  otherCompanyId,
  seedCreateSession,
  tenantId,
  webURL,
  workbookPath,
} from './helpers'

async function openCreatePage(page: Page) {
  await seedCreateSession(page, {
    token: buyerJwt,
    user: buyerUserId,
    company: buyerCompanyId,
    roles: ['PROCUREMENT_MANAGER'],
  })
  await page.goto(`${webURL}/tenders/new-from-xlsx`, { waitUntil: 'domcontentloaded' })
  await expect(page.getByTestId('buyer-xlsx-create-page')).toBeVisible({ timeout: 30_000 })
  await expect(page.getByTestId('buyer-xlsx-create-panel')).toBeVisible()
}

async function previewValidWorkbook(page: Page, rfxNumber: string, title: string) {
  await fillCreateMetadata(page, rfxNumber, title)
  await page.getByTestId('buyer-xlsx-create-file').setInputFiles(workbookPath)
  const previewResp = page.waitForResponse((resp) => {
    return resp.url().includes('/xlsx-create/preview') && resp.request().method() === 'POST'
  }, { timeout: 30_000 })
  await page.getByTestId('buyer-xlsx-create-preview').click()
  return previewResp
}

test.describe('buyer XLSX create live stack', () => {
  test('allowed buyer sees entry and can reach create page while manual wizard stays available', async ({ page }) => {
    const probe = attachCreateNetworkProbe(page)
    await openTendersList(page)
    await expect(page.getByTestId('buyer-xlsx-create-entry')).toBeVisible()
    await page.getByTestId('buyer-xlsx-create-entry').click()
    await expect(page).toHaveURL(/\/tenders\/new-from-xlsx/)
    await expect(page.getByTestId('buyer-xlsx-create-panel')).toBeVisible()
    await page.getByTestId('buyer-xlsx-create-back-manual').click()
    await expect(page).toHaveURL(/\/tenders\/new/)
    await expect(page.getByTestId('wizard-title')).toBeVisible()
    await expectNoProductLeaks(page, probe)
  })

  test('valid preview is ready, findings stay safe, and metadata or file changes require a new preview', async ({ page }) => {
    const probe = attachCreateNetworkProbe(page)
    await openCreatePage(page)
    const rfxNumber = `RFX-F5-PV-${Date.now()}`
    const previewResp = await previewValidWorkbook(page, rfxNumber, 'F5 preview ready')
    const preview = await previewResp
    expect(preview.status(), formatCreateNetworkProbe(probe)).toBe(200)
    await assertLivePreviewMultipart(page, preview.request())

    await expect(page.getByTestId('buyer-xlsx-create-result')).toBeVisible()
    await expect(page.getByTestId('buyer-xlsx-create-ready')).toHaveText(/yes/i)
    await expect(page.getByTestId('buyer-xlsx-create-analysis-id')).not.toHaveText('—')
    await expect(page.getByTestId('buyer-xlsx-create-summary')).toBeVisible()
    await expect(page.getByTestId('buyer-xlsx-create-commit')).toBeEnabled()
    await expect(page.locator('body')).not.toContainText('canonical_payload_hash')
    await expect(page.locator('body')).not.toContainText('=SUM(')

    await page.getByTestId('buyer-xlsx-create-type').selectOption('NOT_A_TYPE').catch(async () => {
      await page.locator('[data-testid="buyer-xlsx-create-type"]').evaluate((el) => {
        const select = el as HTMLSelectElement
        const option = document.createElement('option')
        option.value = 'NOT_A_TYPE'
        option.textContent = 'NOT_A_TYPE'
        select.appendChild(option)
        select.value = 'NOT_A_TYPE'
        select.dispatchEvent(new Event('input', { bubbles: true }))
        select.dispatchEvent(new Event('change', { bubbles: true }))
      })
    })
    const findingsResp = page.waitForResponse((resp) => {
      return resp.url().includes('/xlsx-create/preview') && resp.request().method() === 'POST'
    }, { timeout: 30_000 })
    await page.getByTestId('buyer-xlsx-create-preview').click()
    const findings = await findingsResp
    expect([200, 422]).toContain(findings.status())
    await expect(page.getByTestId('buyer-xlsx-create-error').or(page.getByTestId('buyer-xlsx-create-findings'))).toBeVisible()
    await expect(page.locator('body')).not.toContainText('rfx.buyer_xlsx')
    await expect(page.getByTestId('buyer-xlsx-create-commit')).toBeDisabled()

    await page.getByTestId('buyer-xlsx-create-type').selectOption('SPOT_RFQ')
    const readyAgain = await previewValidWorkbook(page, rfxNumber, 'F5 preview ready')
    expect((await readyAgain).status()).toBe(200)
    await expect(page.getByTestId('buyer-xlsx-create-commit')).toBeEnabled()

    await page.getByTestId('buyer-xlsx-create-title').fill('F5 preview title changed')
    await expect(page.getByTestId('buyer-xlsx-create-commit')).toBeDisabled()
    await expect(page.getByTestId('buyer-xlsx-create-result')).toHaveCount(0)

    const afterMeta = await previewValidWorkbook(page, rfxNumber, 'F5 preview title changed')
    expect((await afterMeta).status()).toBe(200)
    await expect(page.getByTestId('buyer-xlsx-create-commit')).toBeEnabled()

    await page.getByTestId('buyer-xlsx-create-file').setInputFiles({
      name: 'replacement.xlsx',
      mimeType: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
      buffer: Buffer.from('not-a-xlsx'),
    })
    await expect(page.getByTestId('buyer-xlsx-create-commit')).toBeDisabled()
    await expect(page.getByTestId('buyer-xlsx-create-result')).toHaveCount(0)
    await expectNoProductLeaks(page, probe)
  })

  test('commit returns 201, redirects to the created DRAFT, and never publishes or adds participants', async ({ page }) => {
    const probe = attachCreateNetworkProbe(page)
    await openCreatePage(page)
    const rfxNumber = `RFX-F5-CM-${Date.now()}`
    const previewResp = await previewValidWorkbook(page, rfxNumber, 'F5 commit draft')
    expect((await previewResp).status(), formatCreateNetworkProbe(probe)).toBe(200)
    await expect(page.getByTestId('buyer-xlsx-create-commit')).toBeEnabled()

    const commitRespPromise = page.waitForResponse((resp) => {
      return resp.url().includes('/xlsx-create/commit') && resp.request().method() === 'POST'
    }, { timeout: 30_000 })
    await page.getByTestId('buyer-xlsx-create-commit').click()
    const commitResp = await commitRespPromise
    expect(commitResp.status(), formatCreateNetworkProbe(probe)).toBe(201)
    const commitRequest = commitResp.request()
    expect(JSON.parse(commitRequest.postData() || '{}')).toEqual({
      analysis_id: expect.stringMatching(/^[0-9a-f-]{36}$/),
    })
    expect(commitRequest.headers()['idempotency-key']).toMatch(/^buyer-xlsx-create-commit:[0-9a-f-]{36}$/)
    const body = await commitResp.json() as { event_id: string; status: string; creation_channel: string }
    expect(body.event_id).toMatch(/^[0-9a-f-]{36}$/)
    await expect(page).toHaveURL(new RegExp(`/tenders/${body.event_id}`))

    const eventGet = await page.request.get(`${gatewayURL}/api/v1/rfx-events/${body.event_id}`, {
      headers: authHeaders(buyerJwt, buyerCompanyId),
    })
    expect(eventGet.ok(), `created event GET ${eventGet.status()}`).toBeTruthy()
    const event = await eventGet.json() as {
      id: string
      status: string
      creation_channel?: string
      rfx_number: string
    }
    expect(event.id).toBe(body.event_id)
    expect(event.status).toBe('DRAFT')
    expect(event.creation_channel).toBe('EXCEL')
    expect(event.rfx_number).toBe(rfxNumber)

    const participantsGet = await page.request.get(`${gatewayURL}/api/v1/rfx-events/${body.event_id}/participants`, {
      headers: authHeaders(buyerJwt, buyerCompanyId),
    })
    expect(participantsGet.ok()).toBeTruthy()
    const participants = await participantsGet.json() as { items?: unknown[] }
    expect(participants.items ?? []).toEqual([])

    expect(probe.commitKeys).toEqual([`buyer-xlsx-create-commit:${JSON.parse(commitRequest.postData() || '{}').analysis_id}`])
    await expectNoProductLeaks(page, probe)
  })

  test('flag-off hides the UI and raw preview or commit return 404 without writes', async ({ page }) => {
    await seedCreateSession(page, {
      token: buyerJwt,
      user: buyerUserId,
      company: buyerCompanyId,
      roles: ['PROCUREMENT_MANAGER'],
      excelFlag: false,
    })
    await page.goto(`${flagOffWebURL}/tenders`, { waitUntil: 'domcontentloaded' })
    await expect(page.getByRole('button', { name: 'Sign out' })).toBeVisible({ timeout: 30_000 })
    await expect(page.getByTestId('buyer-xlsx-create-entry')).toHaveCount(0)

    await page.goto(`${flagOffWebURL}/tenders/new-from-xlsx`, { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('buyer-xlsx-create-unavailable')).toBeVisible()
    await expect(page.getByTestId('buyer-xlsx-create-panel')).toHaveCount(0)

    const preview = await page.request.post(`${flagOffGatewayURL}/api/v1/rfx-events/xlsx-create/preview`, {
      headers: authHeaders(buyerJwt, buyerCompanyId),
      multipart: {
        file: {
          name: 'flag-off.xlsx',
          mimeType: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
          buffer: Buffer.from('PK'),
        },
        owner_company_id: buyerCompanyId,
        rfx_number: flagOffRfxNumber,
        title: 'Flag off must not write',
        rfx_type: 'SPOT_RFQ',
        category: 'FREIGHT',
      },
    })
    expect(preview.status(), `flag-off preview ${preview.status()}`).toBe(404)
    const previewText = await preview.text()
    expect(previewText.trim().length === 0 || !previewText.trim().startsWith('{')).toBeTruthy()

    const commit = await page.request.post(`${flagOffGatewayURL}/api/v1/rfx-events/xlsx-create/commit`, {
      headers: {
        ...authHeaders(buyerJwt, buyerCompanyId),
        'Content-Type': 'application/json',
        'Idempotency-Key': 'buyer-xlsx-create-commit:00000000-0000-4000-8000-000000000001',
      },
      data: { analysis_id: '00000000-0000-4000-8000-000000000001' },
    })
    expect(commit.status(), `flag-off commit ${commit.status()}`).toBe(404)
  })

  test('unauthorized roles cannot use create-from-xlsx and tenant or company isolation stays fail-closed', async ({ page }) => {
    await seedCreateSession(page, {
      token: logistJwt,
      user: logistUserId,
      company: buyerCompanyId,
      roles: ['SHIPPER_LOGIST'],
    })
    await page.goto(`${webURL}/tenders`, { waitUntil: 'domcontentloaded' })
    await expect(page.getByRole('button', { name: 'Sign out' })).toBeVisible({ timeout: 30_000 })
    await expect(page.getByTestId('buyer-xlsx-create-entry')).toHaveCount(0)
    await page.goto(`${webURL}/tenders/new-from-xlsx`, { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('buyer-xlsx-create-unavailable')).toBeVisible()

    await seedCreateSession(page, {
      token: carrierJwt,
      user: carrierUserId,
      company: carrierCompanyId,
      roles: ['CARRIER_DISPATCHER'],
    })
    await page.goto(`${webURL}/tenders`, { waitUntil: 'domcontentloaded' })
    await expect(page.getByRole('button', { name: 'Sign out' })).toBeVisible({ timeout: 30_000 })
    await expect(page.getByTestId('buyer-xlsx-create-entry')).toHaveCount(0)

    const logistPreview = await page.request.post(`${gatewayURL}/api/v1/rfx-events/xlsx-create/preview`, {
      headers: authHeaders(logistJwt, buyerCompanyId),
      multipart: {
        file: { name: 'role.xlsx', mimeType: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet', buffer: Buffer.from('PK') },
        owner_company_id: buyerCompanyId,
        rfx_number: `RFX-F5-LOG-${Date.now()}`,
        title: 'Logist denied',
        rfx_type: 'SPOT_RFQ',
        category: 'FREIGHT',
      },
    })
    expect(logistPreview.status()).toBe(403)

    const carrierPreview = await page.request.post(`${gatewayURL}/api/v1/rfx-events/xlsx-create/preview`, {
      headers: authHeaders(carrierJwt, carrierCompanyId),
      multipart: {
        file: { name: 'role.xlsx', mimeType: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet', buffer: Buffer.from('PK') },
        owner_company_id: carrierCompanyId,
        rfx_number: `RFX-F5-CAR-${Date.now()}`,
        title: 'Carrier denied',
        rfx_type: 'SPOT_RFQ',
        category: 'FREIGHT',
      },
    })
    expect(carrierPreview.status()).toBe(403)

    const foreignPreview = await page.request.post(`${gatewayURL}/api/v1/rfx-events/xlsx-create/preview`, {
      headers: authHeaders(foreignJwt, buyerCompanyId, foreignTenantId),
      multipart: {
        file: { name: 'tenant.xlsx', mimeType: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet', buffer: Buffer.from('PK') },
        owner_company_id: buyerCompanyId,
        rfx_number: `RFX-F5-FOR-${Date.now()}`,
        title: 'Foreign denied',
        rfx_type: 'SPOT_RFQ',
        category: 'FREIGHT',
      },
    })
    expect([403, 404]).toContain(foreignPreview.status())

    const otherCompany = await page.request.post(`${gatewayURL}/api/v1/rfx-events/xlsx-create/preview`, {
      headers: authHeaders(buyerJwt, otherCompanyId),
      multipart: {
        file: { name: 'company.xlsx', mimeType: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet', buffer: Buffer.from('PK') },
        owner_company_id: otherCompanyId,
        rfx_number: `RFX-F5-CO-${Date.now()}`,
        title: 'Other company denied',
        rfx_type: 'SPOT_RFQ',
        category: 'FREIGHT',
      },
    })
    expect([403, 404]).toContain(otherCompany.status())
  })
})
