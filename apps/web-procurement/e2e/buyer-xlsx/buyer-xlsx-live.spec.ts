import { expect, test, type Page } from '@playwright/test'
import { expectPanelVisible, requireBuyerXlsxEnv } from './helpers'

function liveFixture() {
  return {
    webURL: requireBuyerXlsxEnv('BROWSER_E2E_WEB_URL'),
    jwt: requireBuyerXlsxEnv('BROWSER_E2E_JWT'),
    tenantId: requireBuyerXlsxEnv('BROWSER_E2E_TENANT_ID'),
    companyId: requireBuyerXlsxEnv('BROWSER_E2E_BUYER_COMPANY_ID'),
    eventId: requireBuyerXlsxEnv('BROWSER_E2E_EVENT_ID'),
    userId: requireBuyerXlsxEnv('BROWSER_E2E_USER_ID'),
    gatewayURL: requireBuyerXlsxEnv('BROWSER_E2E_GATEWAY_URL'),
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
    document.cookie = 'freight_procurement_locale=en-US; path=/'
  }, {
    token: fix.jwt,
    tenant: fix.tenantId,
    company: fix.companyId,
    user: fix.userId,
  })
}

async function openDraft(page: Page) {
  const { eventId } = liveFixture()
  await seedLiveBuyerSession(page)
  await page.goto(`/tenders/${eventId}`, { waitUntil: 'domcontentloaded' })
  await expect(page.getByText('Tender not found')).toHaveCount(0)
  await expectPanelVisible(page)
}

async function exportWorkbook(page: Page) {
  const [download] = await Promise.all([
    page.waitForEvent('download'),
    page.getByTestId('buyer-xlsx-export').click(),
  ])
  const path = await download.path()
  if (!path) {
    throw new Error('export download path is empty')
  }
  return path
}

async function uploadWorkbook(page: Page, filePath: string) {
  await page.getByTestId('buyer-xlsx-file').setInputFiles(filePath)
}

test.describe('buyer XLSX live stack', () => {
  test('422 preview keeps Commit disabled', async ({ page }) => {
    await openDraft(page)
    await page.getByTestId('buyer-xlsx-file').setInputFiles({
      name: 'bad.xlsx',
      mimeType: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
      buffer: Buffer.from('not-a-xlsx'),
    })
    await expect(page.getByTestId('buyer-xlsx-error')).toBeVisible()
    await expect(page.getByTestId('buyer-xlsx-commit')).toBeDisabled()
    await expect(page.getByTestId('buyer-xlsx-committed')).toHaveCount(0)
    await expect(page.getByTestId('buyer-xlsx-issue-text')).not.toContainText('rfx.buyer_xlsx')
  })

  test('409 stale disables Commit until a new Preview', async ({ page }) => {
    await openDraft(page)
    const workbook = await exportWorkbook(page)
    await uploadWorkbook(page, workbook)
    await expect(page.getByTestId('buyer-xlsx-preview')).toBeVisible()
    await expect(page.getByTestId('buyer-xlsx-commit')).toBeEnabled()

    await page.getByLabel('Lot number').fill('L-STALE')
    await page.getByLabel('Lot name').fill('Stale lot')
    await page.getByRole('button', { name: 'Add lot' }).click()
    await expect(page.getByText('L-STALE')).toBeVisible()

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
    await openDraft(page)
    const workbook = await exportWorkbook(page)
    await uploadWorkbook(page, workbook)
    await expect(page.getByTestId('buyer-xlsx-preview')).toBeVisible()
    await expect(page.getByTestId('buyer-xlsx-commit')).toBeEnabled()
    await page.getByTestId('buyer-xlsx-commit').click()
    await expect(page.getByTestId('buyer-xlsx-committed')).toBeVisible()
    await expect(page.getByTestId('buyer-xlsx-commit')).toBeDisabled()
    await expect(page.locator('.badge, [data-status], dd').filter({ hasText: 'DRAFT' }).first()).toBeVisible()
    await expect(page).toHaveURL(new RegExp(`/tenders/${eventId}`))
  })
})
