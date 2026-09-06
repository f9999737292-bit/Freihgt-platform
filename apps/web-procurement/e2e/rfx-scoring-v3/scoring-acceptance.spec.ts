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
  test.beforeEach(async ({ page }) => {
    await seedBuyerAdminSession(page)
    await seedBuyerProcurementSession(page)
    page.on('console', (msg) => {
      if (msg.type() === 'error') {
        console.log(`[browser-console] ${msg.text()}`)
      }
    })
  })

  test('studio publish + evaluation scores + legacy regression', async ({ page, request }) => {
    test.skip(!adminURL || !procurementURL, 'BROWSER_E2E URLs required')

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

    const scoreTotals: number[] = []
    for (const item of evalPayload.items) {
      const scoreResp = await request.get(
        `${gatewayURL}/api/v1/rfx-events/${eventId}/responses/${item.id}/score`,
        { headers: buyerHeaders },
      )
      expect(scoreResp.ok()).toBeTruthy()
      const scorePayload = await scoreResp.json()
      expect(scorePayload.qualification?.calculation_status).toBe('CALCULATED')
      scoreTotals.push(Number(scorePayload.qualification?.total_score))
    }
    expect(scoreTotals).toContain(70)
    expect(scoreTotals).toContain(60)

    await page.goto(`${procurementURL}/login`, { waitUntil: 'domcontentloaded' })
    const browserResponses = page.waitForResponse(
      (resp) =>
        resp.url().includes(`/rfx-events/${eventId}/responses`) &&
        resp.request().method() === 'GET' &&
        resp.ok(),
      { timeout: 120_000 },
    )
    await page.goto(`${procurementURL}/tenders/${eventId}/evaluation`, { waitUntil: 'domcontentloaded' })
    const browserResponsesResp = await browserResponses
    expect(browserResponsesResp.ok()).toBeTruthy()
    const browserResponsesBody = await browserResponsesResp.json()
    expect(Array.isArray(browserResponsesBody.items)).toBeTruthy()
    expect(browserResponsesBody.items.length).toBeGreaterThanOrEqual(2)

    await expect(page.getByTestId('evaluation-comparison-table')).toBeVisible({ timeout: 60_000 })
    await expect(page.getByTestId('legacy-commercial-score').first()).toBeVisible({ timeout: 60_000 })

    const v3Cells = page.getByTestId('v3-questionnaire-score')
    await expect(v3Cells).toHaveCount(2, { timeout: 120_000 })
    await expect.poll(async () => v3Cells.allTextContents(), { timeout: 120_000 }).toEqual(
      expect.arrayContaining([expect.stringContaining('70'), expect.stringContaining('60')]),
    )

    await expect(page.getByTestId('v3-knockout-badge')).toHaveCount(1, { timeout: 60_000 })

    await page.getByTestId('v3-explain-button').nth(1).click()
    await expect(page.getByTestId('v3-explanation-panel')).toBeVisible({ timeout: 30_000 })
    await expect(page.getByTestId('v3-explanation-row').first()).toBeVisible()

    await page.goto(`${procurementURL}/login`, { waitUntil: 'domcontentloaded' })
    await page.goto(`${procurementURL}/tenders/${legacyEventId}/evaluation`, { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('v3-questionnaire-score')).toHaveCount(0)
    await expect(page.getByTestId('legacy-commercial-score').first()).toBeVisible()
  })
})
