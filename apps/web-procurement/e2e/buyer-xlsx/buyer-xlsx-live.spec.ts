import { expect, test, type Page } from '@playwright/test'
import {
  attachBuyerXlsxNetworkProbe,
  expectPanelVisible,
  expectWorkspaceLoaded,
  formatBuyerXlsxNetworkProbe,
  requireBuyerXlsxEnv,
} from './helpers'

function liveFixture() {
  return {
    webURL: requireBuyerXlsxEnv('BROWSER_E2E_WEB_URL'),
    jwt: requireBuyerXlsxEnv('BROWSER_E2E_JWT'),
    tenantId: requireBuyerXlsxEnv('BROWSER_E2E_TENANT_ID'),
    companyId: requireBuyerXlsxEnv('BROWSER_E2E_BUYER_COMPANY_ID'),
    eventId: requireBuyerXlsxEnv('BROWSER_E2E_EVENT_ID'),
    userId: requireBuyerXlsxEnv('BROWSER_E2E_USER_ID'),
    gatewayURL: requireBuyerXlsxEnv('BROWSER_E2E_GATEWAY_URL'),
    rfxNumber: requireBuyerXlsxEnv('BROWSER_E2E_RFX_NUMBER'),
  }
}

async function seedLiveBuyerSession(page: Page) {
  const fix = liveFixture()
  await page.addInitScript((input) => {
    localStorage.setItem(
      'freight_procurement_session',
      JSON.stringify({
        token: input.token,
        user: {
          id: input.user,
          tenant_id: input.tenant,
          email: 'buyer-xlsx-live@freight.test',
          full_name: 'Buyer XLSX Live',
          preferred_locale: 'en-US',
          status: 'ACTIVE',
          roles: ['PROCUREMENT_MANAGER'],
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
    company: fix.companyId,
    user: fix.userId,
  })
}

async function openDraft(page: Page) {
  const { eventId, rfxNumber } = liveFixture()
  await seedLiveBuyerSession(page)
  const eventResp = page.waitForResponse((resp) => {
    const url = resp.url()
    return (
      url.includes(`/api/v1/rfx-events/${eventId}`)
      && !url.includes('/lots')
      && !url.includes('/participants')
      && !url.includes('/xlsx')
      && resp.request().method() === 'GET'
    )
  }, { timeout: 30_000 })
  await page.goto(`/tenders/${eventId}`, { waitUntil: 'domcontentloaded' })
  const loaded = await eventResp
  if (loaded.status() !== 200) {
    throw new Error(`workspace event GET ${loaded.status()} ${loaded.url()}`)
  }
  await expectWorkspaceLoaded(page, rfxNumber)
  await expectPanelVisible(page)
}

async function exportWorkbook(page: Page, probe = attachBuyerXlsxNetworkProbe(page)) {
  const { eventId } = liveFixture()
  const respPromise = page.waitForResponse((resp) => {
    return resp.url().includes(`/rfx-events/${eventId}/xlsx-export`) && resp.request().method() === 'GET'
  }, { timeout: 30_000 })
  await page.getByTestId('buyer-xlsx-export').click()
  const resp = await respPromise.catch((error: Error) => {
    throw new Error(`${error.message}\n${formatBuyerXlsxNetworkProbe(probe)}`)
  })
  if (!resp.ok()) {
    throw new Error(`export status=${resp.status()} ${resp.url()}\n${formatBuyerXlsxNetworkProbe(probe)}`)
  }
  const { writeFileSync } = await import('node:fs')
  const { join } = await import('node:path')
  const { tmpdir } = await import('node:os')
  const filePath = join(tmpdir(), `buyer-xlsx-${eventId}.xlsx`)
  writeFileSync(filePath, Buffer.from(await resp.body()))
  await expect(page.getByTestId('buyer-xlsx-error')).toHaveCount(0)
  return filePath
}

async function uploadWorkbook(page: Page, filePath: string) {
  await page.getByTestId('buyer-xlsx-file').setInputFiles(filePath)
}

test.describe('buyer XLSX live stack', () => {
  test('422 preview keeps Commit disabled', async ({ page }) => {
    const probe = attachBuyerXlsxNetworkProbe(page)
    await openDraft(page)
    await page.getByTestId('buyer-xlsx-file').setInputFiles({
      name: 'bad.xlsx',
      mimeType: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
      buffer: Buffer.from('not-a-xlsx'),
    })
    await expect(page.getByTestId('buyer-xlsx-error'), formatBuyerXlsxNetworkProbe(probe)).toHaveAttribute(
      'data-error-kind',
      'preview_invalid',
    )
    await expect(page.getByTestId('buyer-xlsx-commit')).toBeDisabled()
    await expect(page.getByTestId('buyer-xlsx-committed')).toHaveCount(0)
    await expect(page.locator('body')).not.toContainText('rfx.buyer_xlsx')
  })

  test('409 stale disables Commit until a new Preview', async ({ page }) => {
    const probe = attachBuyerXlsxNetworkProbe(page)
    await openDraft(page)
    const workbook = await exportWorkbook(page, probe)
    await uploadWorkbook(page, workbook)
    await expect(page.getByTestId('buyer-xlsx-preview')).toBeVisible()
    await expect(page.getByTestId('buyer-xlsx-commit')).toBeEnabled()

    const fix = liveFixture()
    const mutate = await page.request.patch(`${fix.gatewayURL}/api/v1/rfx-events/${fix.eventId}`, {
      headers: {
        Authorization: `Bearer ${fix.jwt}`,
        'Content-Type': 'application/json',
        'X-Company-ID': fix.companyId,
      },
      data: { title: `Stale after preview ${Date.now()}` },
    })
    expect(mutate.ok()).toBeTruthy()

    await page.getByTestId('buyer-xlsx-commit').click()
    await expect(page.getByTestId('buyer-xlsx-error')).toHaveAttribute('data-error-kind', 'stale_target')
    await expect(page.getByTestId('buyer-xlsx-commit')).toBeDisabled()
    await expect(page.getByTestId('buyer-xlsx-retry-preview')).toBeEnabled()

    await page.getByTestId('buyer-xlsx-retry-preview').click()
    await expect(page.getByTestId('buyer-xlsx-preview')).toBeVisible()
    await expect(page.getByTestId('buyer-xlsx-commit')).toBeEnabled()
  })

  test('export, preview, and commit update the same DRAFT', async ({ page }) => {
    const { eventId } = liveFixture()
    const probe = attachBuyerXlsxNetworkProbe(page)
    await openDraft(page)
    const workbook = await exportWorkbook(page, probe)
    await uploadWorkbook(page, workbook)
    await expect(page.getByTestId('buyer-xlsx-preview')).toBeVisible()
    await expect(page.getByTestId('buyer-xlsx-commit')).toBeEnabled()
    const commitKeys: string[] = []
    page.on('request', (req) => {
      if (req.method() === 'POST' && req.url().includes(`/rfx-events/${eventId}/xlsx-import/commit`)) {
        commitKeys.push(req.headers()['idempotency-key'] || '')
      }
    })
    await page.getByTestId('buyer-xlsx-commit').click()
    await expect(page.getByTestId('buyer-xlsx-committed')).toBeVisible()
    await expect(page.getByTestId('buyer-xlsx-commit')).toBeDisabled()
    await page.getByTestId('buyer-xlsx-commit').click({ force: true })
    await expect(page.getByTestId('buyer-xlsx-commit')).toBeDisabled()
    expect(commitKeys.length, formatBuyerXlsxNetworkProbe(probe)).toBe(1)
    expect(commitKeys[0]).toMatch(/^buyer-xlsx-commit:[0-9a-f-]{36}$/)
    await expect(page.locator('.badge, [data-status], dd').filter({ hasText: 'DRAFT' }).first()).toBeVisible()
    await expect(page).toHaveURL(new RegExp(`/tenders/${eventId}`))
  })
})
