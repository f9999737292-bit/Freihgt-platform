import { expect, test, type Page } from '@playwright/test'
import {
  attachCarrierXlsxNetworkProbe,
  expectPanelVisible,
  expectWorkspaceLoaded,
  formatCarrierXlsxNetworkProbe,
  fulfillJSON,
  requireCarrierXlsxEnv,
} from './helpers'

function liveFixture() {
  return {
    webURL: requireCarrierXlsxEnv('BROWSER_E2E_WEB_URL'),
    jwt: requireCarrierXlsxEnv('BROWSER_E2E_JWT'),
    tenantId: requireCarrierXlsxEnv('BROWSER_E2E_TENANT_ID'),
    carrierCompanyId: requireCarrierXlsxEnv('BROWSER_E2E_CARRIER_COMPANY_ID'),
    buyerCompanyId: requireCarrierXlsxEnv('BROWSER_E2E_BUYER_COMPANY_ID'),
    eventId: requireCarrierXlsxEnv('BROWSER_E2E_EVENT_ID'),
    responseId: requireCarrierXlsxEnv('BROWSER_E2E_RESPONSE_ID'),
    userId: requireCarrierXlsxEnv('BROWSER_E2E_USER_ID'),
    gatewayURL: requireCarrierXlsxEnv('BROWSER_E2E_GATEWAY_URL'),
    rfxNumber: requireCarrierXlsxEnv('BROWSER_E2E_RFX_NUMBER'),
    competitorOffer: requireCarrierXlsxEnv('BROWSER_E2E_COMPETITOR_OFFER'),
    competitorName: requireCarrierXlsxEnv('BROWSER_E2E_COMPETITOR_NAME'),
    competitorAnswer: requireCarrierXlsxEnv('BROWSER_E2E_COMPETITOR_ANSWER'),
    lotId: requireCarrierXlsxEnv('BROWSER_E2E_LOT_ID'),
  }
}

async function seedLiveCarrierSession(page: Page) {
  const fix = liveFixture()
  await page.addInitScript((input) => {
    localStorage.setItem(
      'freight_procurement_session',
      JSON.stringify({
        token: input.token,
        user: {
          id: input.user,
          tenant_id: input.tenant,
          email: 'carrier-xlsx-live@freight.test',
          full_name: 'Carrier XLSX Live',
          preferred_locale: 'en-US',
          status: 'ACTIVE',
          roles: ['CARRIER_DISPATCHER'],
        },
      }),
    )
    localStorage.setItem('freight_procurement_tenant_id', input.tenant)
    localStorage.setItem('freight_procurement_company_id', input.company)
    localStorage.setItem('freight_procurement_rfx_excel_exchange', 'true')
    document.cookie = 'freight_procurement_locale=en-US; path=/'
  }, {
    token: fix.jwt,
    tenant: fix.tenantId,
    company: fix.carrierCompanyId,
    user: fix.userId,
  })
}

async function stubLiveCarrierShell(page: Page) {
  const fix = liveFixture()
  await page.route(`**/api/v1/users/${fix.userId}/companies**`, async (route) => {
    await fulfillJSON(route, 200, {
      items: [{
        membership_id: `${fix.carrierCompanyId}-membership`,
        company_id: fix.carrierCompanyId,
        legal_name: 'Carrier A',
        company_type: 'CARRIER',
        membership_status: 'ACTIVE',
        roles: [{ code: 'CARRIER_DISPATCHER', name: 'Carrier Dispatcher' }],
      }],
    })
  })
  await page.route('**/api/v1/companies**', async (route) => {
    const url = new URL(route.request().url())
    if (url.pathname !== '/api/v1/companies' && !url.pathname.endsWith('/companies')) {
      await route.fallback()
      return
    }
    await fulfillJSON(route, 200, {
      items: [
        { id: fix.buyerCompanyId, legal_name: 'Buyer A', company_type: 'SHIPPER', status: 'ACTIVE' },
        { id: fix.carrierCompanyId, legal_name: 'Carrier A', company_type: 'CARRIER', status: 'ACTIVE' },
      ],
    })
  })
  await page.route(`**/api/v1/rfx-events/${fix.eventId}`, async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback()
      return
    }
    await fulfillJSON(route, 200, {
      id: fix.eventId,
      tenant_id: fix.tenantId,
      owner_company_id: fix.buyerCompanyId,
      rfx_number: fix.rfxNumber,
      title: 'Carrier XLSX live',
      status: 'PUBLISHED',
      rfx_type: 'SPOT_RFQ',
      category: 'FREIGHT',
      currency_code: 'RUB',
      response_deadline: new Date(Date.now() + 48 * 3600 * 1000).toISOString(),
    })
  })
}

async function openDraft(page: Page) {
  const { eventId, rfxNumber, responseId } = liveFixture()
  await seedLiveCarrierSession(page)
  await stubLiveCarrierShell(page)
  const responseWait = page.waitForResponse((resp) => {
    return (
      resp.url().includes(`/api/v1/rfx-events/${eventId}/own-response`)
      && resp.request().method() === 'GET'
    )
  }, { timeout: 30_000 })
  await page.goto(`/carrier/tenders/${eventId}`, { waitUntil: 'domcontentloaded' })
  const loaded = await responseWait
  if (loaded.status() !== 200) {
    throw new Error(`own-response GET ${loaded.status()} ${loaded.url()}`)
  }
  await expectWorkspaceLoaded(page, rfxNumber)
  await expectPanelVisible(page, 'DRAFT')
  await expect(page.getByTestId('carrier-xlsx-gate')).toHaveAttribute('data-response-id', responseId)
}

async function exportWorkbook(page: Page, probe = attachCarrierXlsxNetworkProbe(page)) {
  const { eventId, responseId, gatewayURL, jwt, carrierCompanyId } = liveFixture()
  const respPromise = page.waitForResponse((resp) => {
    return resp.url().includes(`/rfx-events/${eventId}/carrier-responses/${responseId}/xlsx-export`)
      && resp.request().method() === 'GET'
  }, { timeout: 30_000 })
  await page.getByTestId('carrier-xlsx-export').click()
  const resp = await respPromise.catch((error: Error) => {
    throw new Error(`${error.message}\n${formatCarrierXlsxNetworkProbe(probe)}`)
  })
  if (!resp.ok()) {
    throw new Error(`export status=${resp.status()} ${resp.url()}\n${formatCarrierXlsxNetworkProbe(probe)}`)
  }
  const liveExport = await page.request.get(
    `${gatewayURL}/api/v1/rfx-events/${eventId}/carrier-responses/${responseId}/xlsx-export`,
    {
      headers: {
        Authorization: `Bearer ${jwt}`,
        'X-Company-ID': carrierCompanyId,
      },
    },
  )
  if (!liveExport.ok()) {
    throw new Error(`live export replay status=${liveExport.status()}`)
  }
  const bytes = Buffer.from(await liveExport.body())
  if (bytes.length < 4 || bytes.subarray(0, 2).toString() !== 'PK') {
    throw new Error(`live export replay is not a ZIP/XLSX (${bytes.length} bytes)`)
  }
  const { writeFileSync } = await import('node:fs')
  const { join } = await import('node:path')
  const { tmpdir } = await import('node:os')
  const filePath = join(tmpdir(), `carrier-xlsx-${responseId}.xlsx`)
  writeFileSync(filePath, bytes)
  await expect(page.getByTestId('carrier-xlsx-error')).toHaveCount(0)
  return { filePath, bytes }
}

async function uploadWorkbook(page: Page, filePath: string) {
  await page.getByTestId('carrier-xlsx-file').setInputFiles(filePath)
}

test.describe('carrier XLSX live stack', () => {
  test('invalid file keeps Commit disabled and hides raw message keys', async ({ page }) => {
    const probe = attachCarrierXlsxNetworkProbe(page)
    await openDraft(page)
    await page.getByTestId('carrier-xlsx-file').setInputFiles({
      name: 'bad.xlsx',
      mimeType: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
      buffer: Buffer.from('not-a-xlsx'),
    })
    await expect(page.getByTestId('carrier-xlsx-error'), formatCarrierXlsxNetworkProbe(probe)).toBeVisible()
    await expect(page.getByTestId('carrier-xlsx-error')).toHaveAttribute(
      'data-error-kind',
      /preview_invalid|validation/,
    )
    await expect(page.getByTestId('carrier-xlsx-commit')).toBeDisabled()
    await expect(page.getByTestId('carrier-xlsx-committed')).toHaveCount(0)
    await expect(page.locator('body')).not.toContainText('rfx.carrier_xlsx')
    await expect(page.locator('body')).not.toContainText('rfx.buyer_xlsx')
  })

  test('409 stale disables Commit until a new Preview', async ({ page }) => {
    const probe = attachCarrierXlsxNetworkProbe(page)
    await openDraft(page)
    const { filePath } = await exportWorkbook(page, probe)
    await uploadWorkbook(page, filePath)
    await expect(page.getByTestId('carrier-xlsx-preview')).toBeVisible()
    await expect(page.getByTestId('carrier-xlsx-commit')).toBeEnabled()

    const fix = liveFixture()
    const mutate = await page.request.patch(`${fix.gatewayURL}/api/v1/rfx-responses/${fix.responseId}`, {
      headers: {
        Authorization: `Bearer ${fix.jwt}`,
        'Content-Type': 'application/json',
        'X-Company-ID': fix.carrierCompanyId,
      },
      data: {
        offer_lines: [{
          rfx_lot_id: fix.lotId,
          amount: 88888.12,
          currency_code: 'RUB',
          comment: `stale-after-preview-${Date.now()}`,
        }],
      },
    })
    expect(mutate.ok(), `mutate status=${mutate.status()}`).toBeTruthy()

    await page.getByTestId('carrier-xlsx-commit').click()
    await expect(page.getByTestId('carrier-xlsx-error')).toHaveAttribute('data-error-kind', 'stale_target')
    await expect(page.getByTestId('carrier-xlsx-commit')).toBeDisabled()
    await expect(page.getByTestId('carrier-xlsx-retry-preview')).toBeEnabled()

    const refreshed = await exportWorkbook(page, probe)
    await uploadWorkbook(page, refreshed.filePath)
    await expect(page.getByTestId('carrier-xlsx-preview')).toBeVisible()
    await expect(page.getByTestId('carrier-xlsx-commit')).toBeEnabled()
  })

  test('export, preview, and commit keep the own response in DRAFT without submit or competitor data', async ({ page }) => {
    const fix = liveFixture()
    const probe = attachCarrierXlsxNetworkProbe(page)
    await openDraft(page)
    const { filePath, bytes } = await exportWorkbook(page, probe)
    const workbookText = bytes.toString('utf8')
    expect(workbookText).not.toContain(fix.competitorOffer)
    expect(workbookText).not.toContain(fix.competitorName)
    expect(workbookText).not.toContain(fix.competitorAnswer)
    expect(workbookText).not.toContain('competitor_')
    await uploadWorkbook(page, filePath)
    await expect(page.getByTestId('carrier-xlsx-preview')).toBeVisible()
    await expect(page.getByTestId('carrier-xlsx-commit')).toBeEnabled()
    const commitKeys: string[] = []
    const submitCalls: string[] = []
    page.on('request', (req) => {
      if (req.method() === 'POST' && req.url().includes(`/rfx-events/${fix.eventId}/carrier-responses/${fix.responseId}/xlsx-import/commit`)) {
        commitKeys.push(req.headers()['idempotency-key'] || '')
      }
      if (req.method() === 'POST' && req.url().includes('/submit')) {
        submitCalls.push(req.url())
      }
    })
    await page.getByTestId('carrier-xlsx-commit').click()
    await expect(page.getByTestId('carrier-xlsx-committed')).toBeVisible()
    await expect(page.getByTestId('carrier-xlsx-commit')).toBeDisabled()
    await page.getByTestId('carrier-xlsx-commit').click({ force: true })
    await expect(page.getByTestId('carrier-xlsx-commit')).toBeDisabled()
    expect(commitKeys.length, formatCarrierXlsxNetworkProbe(probe)).toBe(1)
    expect(commitKeys[0]).toMatch(/^carrier-xlsx-commit:[0-9a-f-]{36}$/)
    expect(submitCalls, formatCarrierXlsxNetworkProbe(probe)).toEqual([])
    await expect(page.getByTestId('carrier-tender-response-status')).toContainText(/draft/i)
    await expect(page).toHaveURL(new RegExp(`/carrier/tenders/${fix.eventId}`))
    await expect(page.getByTestId('carrier-submit-response')).toBeVisible()
  })
})
