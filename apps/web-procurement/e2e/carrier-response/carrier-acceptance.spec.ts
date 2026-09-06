import { expect, test } from '@playwright/test'
import {
  assertGatewayHost,
  eventId,
  gatewayURL,
  jwt,
  questionnairePath,
  seedCarrierSession,
  tenantId,
  userId,
  waitForInvalid,
  waitForSaved,
  carrierCompanyId,
} from './helpers'

test.beforeEach(async ({ page }) => {
  test.setTimeout(300_000)
  if (!jwt || !tenantId || !eventId || !userId || !carrierCompanyId) {
    test.skip(true, 'browser E2E env not configured')
  }
  page.on('console', (msg) => {
    if (msg.type() === 'error') console.error(`[browser-console] ${msg.text()}`)
  })
  await seedCarrierSession(page)
})

test('F103-001 carrier response live browser acceptance', async ({ page }) => {
  const patchResponses: { status: number; body?: unknown }[] = []
  page.on('response', async (resp) => {
    if (resp.url().includes('/carrier-response/answers') && resp.request().method() === 'PATCH') {
      let body: unknown
      try {
        body = await resp.json()
      } catch {
        body = null
      }
      patchResponses.push({ status: resp.status(), body })
    }
  })

  await page.goto('/login', { waitUntil: 'domcontentloaded' })
  const workspaceLoad = page.waitForResponse(
    (resp) => resp.url().includes(`/carrier-response`) && resp.status() < 500,
    { timeout: 120_000 },
  )
  await page.goto(questionnairePath(), { waitUntil: 'domcontentloaded' })
  await expect(page.getByTestId('carrier-response-workspace')).toBeVisible({ timeout: 120_000 })
  const loadResp = await workspaceLoad
  assertGatewayHost(loadResp.url())

  await expect(page.getByTestId('autosave-status')).toBeVisible()

  const adrYes = page.getByTestId('question-ADR_AVAILABLE').getByText('Да', { exact: true })
  await adrYes.click()

  const adrNumber = page.getByTestId('question-ADR_NUMBER').locator('input')
  await expect(adrNumber).toBeVisible()
  await adrNumber.fill('ADR-12345')

  const fleet = page.getByTestId('question-FLEET_COUNT').locator('input')
  await fleet.fill('10')
  await waitForSaved(page)

  await fleet.fill('-1')
  await waitForInvalid(page)
  await expect(page.getByTestId('inline-errors')).toBeVisible()
  await expect(fleet).toHaveValue('-1')

  await page.reload({ waitUntil: 'domcontentloaded' })
  await expect(page.getByTestId('question-FLEET_COUNT').locator('input')).toHaveValue('10', { timeout: 60_000 })

  await page.getByTestId('question-FLEET_COUNT').locator('input').fill('20')
  await waitForSaved(page)

  await page.goto('/carrier/tenders', { waitUntil: 'domcontentloaded' })
  await page.goto(questionnairePath(), { waitUntil: 'domcontentloaded' })
  await expect(page.getByTestId('question-ADR_NUMBER').locator('input')).toHaveValue('ADR-12345')
  await expect(page.getByTestId('question-FLEET_COUNT').locator('input')).toHaveValue('20')

  await page.getByTestId('submit-questionnaire').click()
  await expect(page.getByTestId('submit-blocked')).toBeVisible()
  await expect(page.getByTestId('global-error-summary')).toBeVisible()

  await page.getByRole('button', { name: 'Исправить' }).first().click()
  const expiry = page.getByTestId('question-ADR_EXPIRY').locator('input')
  await expect(expiry).toBeFocused({ timeout: 15_000 })
  await expiry.fill('2030-12-31')
  await waitForSaved(page)

  page.once('dialog', (dialog) => dialog.accept())
  await page.getByTestId('submit-questionnaire').click()
  await expect(page.getByTestId('post-submit-lock')).toBeVisible({ timeout: 60_000 })

  await expect(page.getByTestId('question-FLEET_COUNT').locator('input')).toBeDisabled()

  const denied = await page.evaluate(async ({ gw, ev, co }) => {
    const session = JSON.parse(localStorage.getItem('freight_procurement_session') || '{}')
    const tenant = localStorage.getItem('freight_procurement_tenant_id')
    const resp = await fetch(`${gw}/api/v1/rfx-events/${ev}/carrier-response/answers?carrier_company_id=${co}`, {
      method: 'PATCH',
      headers: {
        Authorization: `Bearer ${session.token}`,
        'Content-Type': 'application/json',
        'X-Tenant-ID': tenant || '',
        'X-Company-ID': co,
        'X-User-ID': session.user?.id || '',
      },
      body: JSON.stringify({ save_version: 999, answers: [] }),
    })
    return resp.status
  }, { gw: gatewayURL, ev: eventId, co: carrierCompanyId })
  expect(denied).toBeGreaterThanOrEqual(400)

  const invalidPatch = patchResponses.find((r) => r.status === 422)
  expect(invalidPatch).toBeTruthy()
})
