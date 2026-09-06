import { test, expect } from '@playwright/test'
import {
  adminURL,
  companyId,
  eventId,
  gatewayURL,
  jwt,
  legacyEventId,
  procurementURL,
  seedBuyerAdminSession,
  seedBuyerProcurementSession,
  submitCarrierA,
  submitCarrierB,
} from './helpers'

test.describe('RFx v3.0D scoring browser acceptance', () => {
  test('studio publish + evaluation scores + legacy regression', async ({ page, request }) => {
    test.skip(!adminURL || !procurementURL, 'BROWSER_E2E URLs required')

    await seedBuyerAdminSession(page)
    await page.goto(`${adminURL}/rfx/${eventId}/studio?step=scoring`, { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('rfx-scoring-workspace')).toBeVisible({ timeout: 120_000 })

    await page.getByTestId('scoring-add-criterion').click()
    const cards = page.getByTestId('scoring-criterion-card')
    await expect(cards).toHaveCount(1)

    await page.getByTestId('scoring-validate').click()
    await expect(page.getByTestId('scoring-readiness-panel')).toBeVisible({ timeout: 30_000 })

    await page.getByTestId('scoring-add-criterion').click()
    await expect(cards).toHaveCount(2)

    const codeInputs = page.getByTestId('scoring-criterion-code')
    await codeInputs.nth(0).fill('HSE')
    await page.getByTestId('scoring-criterion-name').nth(0).fill('HSE')
    await page.getByTestId('scoring-criterion-weight').nth(0).fill('40')
    await codeInputs.nth(1).fill('CAPACITY')
    await page.getByTestId('scoring-criterion-name').nth(1).fill('Capacity')
    await page.getByTestId('scoring-criterion-weight').nth(1).fill('60')

    const bindings = page.getByTestId('scoring-question-binding')
    await expect(bindings.nth(0).locator('option')).toHaveCount(3, { timeout: 120_000 })
    await expect(bindings.nth(1).locator('option')).toHaveCount(3, { timeout: 120_000 })
    await bindings.nth(0).selectOption('ADR_AVAILABLE')
    await bindings.nth(1).selectOption('FLEET_COUNT')

    await page.getByTestId('scoring-knockout-boolean-false').check()
    await page.getByTestId('scoring-save-draft').click()
    await page.getByTestId('scoring-validate').click()
    await expect(page.getByTestId('scoring-readiness-ready')).toBeVisible({ timeout: 60_000 })

    await page.getByTestId('scoring-publish').click()
    await page.getByTestId('scoring-publish-confirm').click()
    await expect(page.getByTestId('scoring-published-lock')).toBeVisible({ timeout: 60_000 })
    await expect(page.getByTestId('scoring-model-status')).toContainText(/Published|Опубликована|已发布/i)

    await submitCarrierA(request)
    await submitCarrierB(request)

    const buyerHeaders = {
      Authorization: `Bearer ${jwt}`,
      'X-Company-ID': companyId,
    }
    const evalApi = await request.get(`${gatewayURL}/api/v1/rfx-events/${eventId}/responses`, { headers: buyerHeaders })
    expect(evalApi.ok()).toBeTruthy()
    const evalPayload = await evalApi.json()
    expect(Array.isArray(evalPayload.items)).toBeTruthy()
    expect(evalPayload.items.length).toBeGreaterThanOrEqual(2)

    await page.goto('about:blank')
    await seedBuyerProcurementSession(page)
    const responsesPromise = page.waitForResponse(
      (resp) => resp.url().includes(`/rfx-events/${eventId}/responses`) && resp.ok(),
      { timeout: 120_000 },
    )
    await page.goto(`${procurementURL}/tenders/${eventId}/evaluation`, { waitUntil: 'domcontentloaded' })
    const responsesResp = await responsesPromise
    const responsesBody = await responsesResp.json()
    expect(Array.isArray(responsesBody.items)).toBeTruthy()
    expect(responsesBody.items.length).toBeGreaterThanOrEqual(2)

    await expect(page.getByTestId('legacy-commercial-score').first()).toBeVisible({ timeout: 120_000 })

    const v3Cells = page.getByTestId('v3-questionnaire-score')
    await expect(v3Cells).toHaveCount(2, { timeout: 120_000 })
    await expect(v3Cells.filter({ hasText: '70' })).toHaveCount(1, { timeout: 120_000 })
    await expect(v3Cells.filter({ hasText: '60' })).toHaveCount(1, { timeout: 120_000 })

    const knockoutRow = page.locator('tr').filter({
      has: page.getByTestId('v3-questionnaire-score').filter({ hasText: '60' }),
    })
    await expect(knockoutRow.getByTestId('v3-knockout-badge')).toBeVisible({ timeout: 60_000 })

    await knockoutRow.getByTestId('v3-explain-button').click()
    await expect(page.getByTestId('v3-explanation-panel')).toBeVisible({ timeout: 30_000 })
    await expect(page.getByTestId('v3-explanation-row').first()).toBeVisible()

    await page.goto(`${procurementURL}/tenders/${legacyEventId}/evaluation`, { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('v3-questionnaire-score')).toHaveCount(0)
    await expect(page.getByTestId('legacy-commercial-score').first()).toBeVisible()
  })
})
