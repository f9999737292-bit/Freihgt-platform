import { test, expect } from '@playwright/test'
import {
  adminURL,
  eventId,
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
    await bindings.nth(0).selectOption({ value: 'ADR_AVAILABLE' })
    await bindings.nth(1).selectOption({ value: 'FLEET_COUNT' })

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

    await seedBuyerProcurementSession(page)
    await page.goto(`${procurementURL}/tenders/${eventId}/evaluation`, { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('legacy-commercial-score').first()).toBeVisible({ timeout: 120_000 })

    const v3Cells = page.getByTestId('v3-questionnaire-score')
    await expect(v3Cells).toHaveCount(2, { timeout: 60_000 })
    await expect(v3Cells.first()).toContainText('70')
    await expect(v3Cells.nth(1)).toContainText('60')
    await expect(page.getByTestId('v3-knockout-badge').first()).toBeVisible()

    await page.getByTestId('v3-explain-button').first().click()
    await expect(page.getByTestId('v3-explanation-panel')).toBeVisible({ timeout: 30_000 })
    await expect(page.getByTestId('v3-explanation-row').first()).toBeVisible()

    await page.goto(`${procurementURL}/tenders/${legacyEventId}/evaluation`, { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('v3-questionnaire-score')).toHaveCount(0)
    await expect(page.getByTestId('legacy-commercial-score').first()).toBeVisible()
  })
})
