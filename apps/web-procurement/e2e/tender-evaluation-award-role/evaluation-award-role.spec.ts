import { test, expect } from '@playwright/test'
import {
  eventId,
  responseId,
  seedBuyerSession,
  seedCarrierSession,
  stubEvaluationWorkspace,
} from './helpers'

test.describe('tender evaluation award button role gate', () => {
  test('SHIPPER_LOGIST sees evaluation but not award actions', async ({ page }) => {
    await seedBuyerSession(page, { roles: ['SHIPPER_LOGIST'] })
    await stubEvaluationWorkspace(page)
    await page.goto(`/tenders/${eventId}/evaluation`, { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('evaluation-comparison-table')).toBeVisible({ timeout: 30_000 })
    await expect(page.getByTestId('evaluation-award')).toHaveCount(0)
  })

  test('PROCUREMENT_MANAGER sees award for complete offers', async ({ page }) => {
    await seedBuyerSession(page, { roles: ['PROCUREMENT_MANAGER'] })
    await stubEvaluationWorkspace(page)
    await page.goto(`/tenders/${eventId}/evaluation`, { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('evaluation-comparison-table')).toBeVisible({ timeout: 30_000 })
    await expect(page.locator(`[data-testid="evaluation-award"][data-award-response-id="${responseId}"]`)).toBeVisible()
  })

  test('CARRIER_DISPATCHER does not see buyer award', async ({ page }) => {
    await seedCarrierSession(page)
    await stubEvaluationWorkspace(page)
    await page.goto(`/tenders/${eventId}/evaluation`, { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('evaluation-award')).toHaveCount(0)
  })
})
