import { test, expect } from '@playwright/test'
import { buyerCompanyId, fulfillJSON } from '../late-submission/helpers'
import {
  eventId,
  responseId,
  seedBuyerSession,
  seedCarrierSession,
  stubEvaluationWorkspace,
} from './helpers'

const otherCompanyId = 'dddddddd-dddd-4ddd-8ddd-dddddddddddd'

async function openEvaluation(page: import('@playwright/test').Page, roles: string[]) {
  await seedBuyerSession(page, { roles })
  await stubEvaluationWorkspace(page)
  await page.goto(`/tenders/${eventId}/evaluation`, { waitUntil: 'domcontentloaded' })
  await expect(page.getByTestId('evaluation-comparison-table')).toBeVisible({ timeout: 30_000 })
}

test.describe('tender evaluation award button role gate', () => {
  test('SHIPPER_LOGIST sees evaluation but not award, shortlist, or recalculate', async ({ page }) => {
    await openEvaluation(page, ['SHIPPER_LOGIST'])
    await expect(page.getByTestId('evaluation-award')).toHaveCount(0)
    await expect(page.getByTestId('evaluation-shortlist')).toHaveCount(0)
    await expect(page.getByTestId('evaluation-recalculate')).toHaveCount(0)
  })

  test('PROCUREMENT_MANAGER sees award for complete offers', async ({ page }) => {
    await openEvaluation(page, ['PROCUREMENT_MANAGER'])
    await expect(page.locator(`[data-testid="evaluation-award"][data-award-response-id="${responseId}"]`)).toBeVisible()
    await expect(page.getByTestId('evaluation-shortlist')).toBeVisible()
    await expect(page.getByTestId('evaluation-recalculate')).toBeVisible()
  })

  test('SHIPPER_ADMIN sees award, shortlist, and recalculate', async ({ page }) => {
    await openEvaluation(page, ['SHIPPER_ADMIN'])
    await expect(page.getByTestId('evaluation-award')).toBeVisible()
    await expect(page.getByTestId('evaluation-shortlist')).toBeVisible()
    await expect(page.getByTestId('evaluation-recalculate')).toBeVisible()
  })

  test('FORWARDER_MANAGER and PLATFORM_ADMIN see award', async ({ page }) => {
    await openEvaluation(page, ['FORWARDER_MANAGER'])
    await expect(page.getByTestId('evaluation-award')).toBeVisible()
    await page.context().clearCookies()
    await page.evaluate(() => localStorage.clear())
    await openEvaluation(page, ['PLATFORM_ADMIN'])
    await expect(page.getByTestId('evaluation-award')).toBeVisible()
  })

  test('CARRIER_DISPATCHER does not see buyer award', async ({ page }) => {
    await seedCarrierSession(page)
    await stubEvaluationWorkspace(page)
    await page.goto(`/tenders/${eventId}/evaluation`, { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('evaluation-award')).toHaveCount(0)
    await expect(page.getByTestId('evaluation-shortlist')).toHaveCount(0)
    await expect(page.getByTestId('evaluation-recalculate')).toHaveCount(0)
  })

  test('award confirmation does not send when the selected company changes', async ({ page }) => {
    const posts: string[] = []
    await page.route('**/award-response', async (route) => {
      posts.push(route.request().method())
      await fulfillJSON(route, 200, { response_id: responseId })
    })
    await openEvaluation(page, ['PROCUREMENT_MANAGER'])
    await page.getByTestId('evaluation-award').click()
    await expect(page.getByTestId('evaluation-award-confirm')).toBeVisible()
    await page.evaluate((companyId) => {
      const root = document.querySelector('#__nuxt')
      const pinia = root && root.__vue_app__ && root.__vue_app__.config.globalProperties.$pinia
      const tenant = pinia && pinia._s.get('tenant')
      if (tenant) tenant.setCompany(companyId)
    }, otherCompanyId)
    await page.getByTestId('evaluation-award-confirm').click()
    await expect(page.getByText('Award is not permitted for the current role or company')).toBeVisible()
    expect(posts).toEqual([])
    expect(otherCompanyId).not.toBe(buyerCompanyId)
  })

  test('award confirmation does not send after the role is withdrawn', async ({ page }) => {
    const posts: string[] = []
    await page.route('**/award-response', async (route) => {
      posts.push(route.request().method())
      await fulfillJSON(route, 200, { response_id: responseId })
    })
    await openEvaluation(page, ['SHIPPER_ADMIN'])
    await page.getByTestId('evaluation-award').click()
    await page.evaluate(() => {
      const root = document.querySelector('#__nuxt')
      const pinia = root && root.__vue_app__ && root.__vue_app__.config.globalProperties.$pinia
      const auth = pinia && pinia._s.get('auth')
      if (auth && auth.user) auth.user = Object.assign({}, auth.user, { roles: ['SHIPPER_LOGIST'] })
    })
    await expect(page.getByTestId('evaluation-award-confirm')).toHaveCount(0)
    expect(posts).toEqual([])
  })

  test('award confirmation shows denial on 403 and does not repeat the request', async ({ page }) => {
    const posts: string[] = []
    await page.route('**/award-response', async (route) => {
      posts.push(route.request().method())
      await new Promise((resolve) => setTimeout(resolve, 300))
      await fulfillJSON(route, 403, { error: { code: 'FORBIDDEN', message: 'denied', details: {} } })
    })
    await openEvaluation(page, ['PROCUREMENT_MANAGER'])
    await page.getByTestId('evaluation-award').click()
    const confirm = page.getByTestId('evaluation-award-confirm')
    await confirm.click()
    await expect(confirm).toBeDisabled()
    await confirm.click({ force: true })
    await expect(page.getByText('Award is not permitted for the current role or company')).toBeVisible()
    expect(posts).toEqual(['POST'])
  })
})
