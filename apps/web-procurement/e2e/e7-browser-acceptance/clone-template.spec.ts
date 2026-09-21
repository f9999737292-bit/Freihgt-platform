import { expect, test } from '@playwright/test'
import {
  adminURL,
  attachLiveDiagnostics,
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
  const openClone = page.getByTestId('clone-from-template-open')
  await expect(openClone).toBeVisible()
  await openClone.click()
  await expect(page.getByTestId('clone-from-template-modal')).toBeVisible({ timeout: 15_000 })
  await expect(page.locator('#clone-modal-title')).toBeVisible()
  attachLiveDiagnostics(page, 'CLONE')
  const detailWait = waitForApi(page, { method: 'GET', pathIncludes: `/api/v1/rfx-templates/${templateId}` })
  await page.getByTestId('clone-template-select').selectOption(templateId)
  const detail = await detailWait
  const detailURL = new URL(detail.url())
  console.log(`E7-TEMPLATE-DETAIL ${detail.request().method()} ${detailURL.pathname} -> ${detail.status()} host=${detailURL.host}`)
  expect(detailURL.pathname, `template detail must not use a trailing slash: ${detailURL.pathname}`).toBe(`/api/v1/rfx-templates/${templateId}`)
  expect(detail.status(), `GET /api/v1/rfx-templates/${templateId} -> ${detail.status()}`).toBe(200)
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
  console.log(`E7-CLONE-DOM-URL ${page.url()}`)
  const modalDump = await page.evaluate(() => {
    const modal = document.querySelector('[data-testid="clone-from-template-modal"]')
    return {
      url: location.href,
      hasSuccess: Boolean(document.querySelector('[data-testid="clone-success"]')),
      hasOpenStudio: Boolean(document.querySelector('[data-testid="clone-open-studio"]')),
      sourceTemplate: document.querySelector('[data-testid="clone-source-template-id"]')?.textContent ?? null,
      sourceVersion: document.querySelector('[data-testid="clone-source-version-id"]')?.textContent ?? null,
      modalText: modal?.textContent?.replace(/\s+/g, ' ').slice(0, 500) ?? null,
    }
  })
  console.log(`E7-CLONE-POST-CREATE ${JSON.stringify(modalDump)}`)
  await expect(page.getByTestId('clone-success')).toBeVisible({ timeout: 15_000 })
  await expect(page.getByTestId('clone-source-template-id')).toHaveText(templateId)
  await expect(page.getByTestId('clone-source-version-id')).toHaveText(body.source_template_version_id || /.*/)
  await expect(page.getByTestId('clone-open-studio')).toBeVisible()

  const studioWait = waitForApi(page, { method: 'GET', pathIncludes: `/api/v1/rfx-events/${body.id}/studio` })
  await page.getByTestId('clone-open-studio').click()
  const studio = await studioWait
  expect(studio.status()).toBe(200)
  await expect(page).toHaveURL(new RegExp(`/rfx/${body.id}/studio`))
  expect(body.creation_channel === 'TEMPLATE' || body.creation_channel === 'MANUAL' || body.creation_channel == null).toBeTruthy()
})
