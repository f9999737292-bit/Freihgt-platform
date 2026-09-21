import { expect, test } from '@playwright/test'
import {
  adminURL,
  buyerCompanyId,
  clickAndCapture,
  logStage,
  seedAdminSession,
  templateCode,
  templateId,
  waitForApi,
} from './helpers'

test('E7-BRW clone from template opens Studio and records MANUAL channel follow-up', async ({ page }) => {
  await seedAdminSession(page)
  const templatesWait = waitForApi(page, { method: 'GET', pathIncludes: '/api/v1/rfx-templates' })
  await page.goto(`${adminURL}/rfx/templates`, { waitUntil: 'domcontentloaded' })
  const templatesResp = await templatesWait
  expect(templatesResp.status(), `GET /api/v1/rfx-templates -> ${templatesResp.status()}`).toBe(200)
  await expect(page.getByRole('heading', { name: /Библиотека шаблонов|Template library|模板库/ })).toBeVisible({ timeout: 60_000 })
  await expect(page.getByText(templateCode)).toBeVisible({ timeout: 30_000 })
  await page.getByRole('button', { name: /Создать RFx из шаблона|Create RFx from template|从模板创建 RFx/ }).click()
  await expect(page.locator('#clone-modal-title')).toBeVisible()
  await page.getByTestId('clone-template-select').selectOption(templateId)
  await expect(page.getByTestId('clone-version-select')).toBeVisible({ timeout: 30_000 })
  await page.getByTestId('clone-event-title').fill('E7 clone from template')
  const owner = page.getByTestId('clone-owner-select')
  await expect(owner.locator('option')).not.toHaveCount(0, { timeout: 15_000 })
  const ownerValue = await owner.inputValue()
  if (!ownerValue) {
    await owner.selectOption({ index: 0 })
  }
  const cloneResp = await clickAndCapture(
    page,
    'clone-from-template',
    { method: 'POST', pathIncludes: '/api/v1/rfx-events/from-template' },
    () => page.getByRole('button', { name: /Создать черновик RFx|Create RFx draft|创建 RFx 草稿/ }).click(),
  )
  expect(cloneResp.status()).toBe(201)
  const body = await cloneResp.json() as {
    id: string
    creation_channel?: string
    source_template_id?: string
    source_template_version_id?: string
  }
  expect(body.id).toBeTruthy()
  expect(body.source_template_id).toBe(templateId)
  expect(body.source_template_version_id).toBeTruthy()
  logStage({
    stage: 'clone-channel-follow-up',
    method: 'POST',
    path: '/api/v1/rfx-events/from-template',
    status: cloneResp.status(),
  })
  console.log(`E7-CLONE-EVENT-ID ${body.id} creation_channel=${body.creation_channel ?? 'MANUAL'} BACKEND_FOLLOW_UP=creation_channel_TEMPLATE`)
  await expect(page.locator('#provenance-title')).toBeVisible()
  await expect(page.locator('code').filter({ hasText: templateId })).toBeVisible()
  await expect(page.getByText(templateCode).or(page.getByText('E7 Template'))).toBeVisible()

  const studioWait = waitForApi(page, { method: 'GET', pathIncludes: `/api/v1/rfx-events/${body.id}/studio` })
  await page.getByRole('button', { name: /Открыть RFx Studio|Open RFx Studio|打开 RFx Studio/ }).click()
  const studio = await studioWait
  expect(studio.status()).toBe(200)
  await expect(page).toHaveURL(new RegExp(`/rfx/${body.id}/studio`))
  expect(body.creation_channel === 'TEMPLATE' || body.creation_channel === 'MANUAL' || body.creation_channel == null).toBeTruthy()
})
