import { expect, test, type Page } from '@playwright/test'
import {
  buyerCompanyId,
  buyerJwt,
  buyerUserId,
  carrierCompanyId,
  carrierJwt,
  carrierUserId,
  isolationEventId,
  isolationRfxNumber,
  otherBuyerCompanyId,
  otherBuyerJwt,
  otherBuyerUserId,
  procurementURL,
  seedProcurementSession,
  waitForApi,
} from './helpers'

const CHANNEL = {
  'en-US': 'Created manually',
  'ru-RU': 'Создан вручную',
  'zh-CN': '手动创建',
} as const

async function openAs(
  page: Page,
  actor: { token: string; user: string; company: string; roles: string[] },
  locale: keyof typeof CHANNEL,
) {
  await seedProcurementSession(page, { ...actor, locale })
  const getWait = waitForApi(page, {
    method: 'GET',
    pathIncludes: `/api/v1/rfx-events/${isolationEventId}`,
    pathExcludes: ['/lots', '/participants', '/xlsx'],
  })
  await page.goto(`${procurementURL}/tenders/${isolationEventId}`, { waitUntil: 'domcontentloaded' })
  return getWait
}

test.describe('E7 isolation and locales', () => {
  test('tender detail channel is localized on RU/EN/ZH without raw codes', async ({ page }) => {
    for (const locale of ['en-US', 'ru-RU', 'zh-CN'] as const) {
      const loaded = await openAs(page, {
        token: buyerJwt,
        user: buyerUserId,
        company: buyerCompanyId,
        roles: ['PROCUREMENT_MANAGER'],
      }, locale)
      expect(loaded.status()).toBe(200)
      const body = await loaded.json() as { creation_channel?: string }
      expect(body.creation_channel).toBe('MANUAL')
      await expect(page.getByTestId('tender-rfx-number')).toHaveText(isolationRfxNumber)
      await expect(page.getByTestId('tender-creation-channel')).toHaveText(CHANNEL[locale])
      await expect(page.locator('body')).not.toContainText('creation_channel')
      if (locale !== 'en-US') {
        await expect(page.getByTestId('tender-creation-channel')).not.toHaveText('MANUAL')
      }
    }
  })

  test('carrier receives 403 and foreign buyer receives 404 without channel leakage', async ({ page }) => {
    const carrierGet = await openAs(page, {
      token: carrierJwt,
      user: carrierUserId,
      company: carrierCompanyId,
      roles: ['CARRIER_DISPATCHER'],
    }, 'en-US')
    expect(carrierGet.status()).toBe(403)
    await expect(page.getByTestId('tender-not-found')).toBeVisible()
    await expect(page.getByTestId('tender-creation-channel')).toHaveCount(0)
    await expect(page.locator('body')).not.toContainText('Created manually')
    await expect(page.locator('body')).not.toContainText('MANUAL')

    const foreignGet = await openAs(page, {
      token: otherBuyerJwt,
      user: otherBuyerUserId,
      company: otherBuyerCompanyId,
      roles: ['PROCUREMENT_MANAGER'],
    }, 'en-US')
    expect(foreignGet.status()).toBe(404)
    await expect(page.getByTestId('tender-not-found')).toBeVisible()
    await expect(page.getByTestId('tender-creation-channel')).toHaveCount(0)
    await expect(page.locator('body')).not.toContainText('Created manually')
    await expect(page.locator('body')).not.toContainText('MANUAL')
  })
})
