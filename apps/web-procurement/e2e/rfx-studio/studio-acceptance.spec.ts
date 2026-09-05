import { expect, test, type Page } from '@playwright/test'
import {
  clickQuestionCard,
  eventId,
  expectAutosaveInvalid,
  expectAutosaveSaved,
  fillQuestionLabel,
  jwt,
  renameSectionTitle,
  rfxNumber,
  seedBuyerSession,
  selectQuestionType,
  studioPath,
  tenantId,
  toggleQuestionRequired,
  userId,
  waitForStudioLoad,
} from './helpers'

const LABEL_ADR_AVAILABLE = 'ADR available?'
const LABEL_ADR_NUMBER = 'ADR number'
const LABEL_ADR_EXPIRY = 'ADR expiry date'
const LABEL_SELECT = 'Transport mode'
const SECTION_HSE = 'HSE'

async function addQuestion(page: Page) {
  await page.getByRole('button', { name: 'Добавить вопрос' }).first().click()
}

async function waitForMutationResponse(page: Page, fragment: string, method: string) {
  return page.waitForResponse(
    (resp) => resp.url().includes(fragment) && resp.request().method() === method,
    { timeout: 60_000 },
  )
}

test.beforeEach(async ({ page }) => {
  test.setTimeout(180_000)
  if (!jwt || !tenantId || !eventId || !userId) {
    test.skip(true, 'browser E2E env not configured')
  }
  await seedBuyerSession(page)
})

test('F102-001 RFx Studio live browser acceptance', async ({ page }) => {
  const gatewayFailures: string[] = []
  page.on('response', (resp) => {
    const url = resp.url()
    if (!url.includes('/api/v1/rfx-events/')) {
      return
    }
    if (resp.status() === 404 || resp.status() >= 500) {
      gatewayFailures.push(`${resp.status()} ${resp.request().method()} ${url}`)
    }
  })

  const studioLoad = waitForStudioLoad(page)
  await page.goto(studioPath(), { waitUntil: 'domcontentloaded' })
  await expect(page).not.toHaveURL(/\/login(?:\?|$)/)
  const studioResponse = await studioLoad
  expect(studioResponse.status()).toBe(200)
  const studioBody = await studioResponse.json()
  expect(studioBody.event?.status).toBe('DRAFT')
  expect(studioBody.event?.rfx_number).toBe(rfxNumber)

  await expect(page.getByRole('link', { name: 'Основное' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Анкета' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Проверка' })).toBeVisible()
  await expect(page.getByText('BINTRANS RFx Studio')).toBeVisible()
  await expect(page.getByText('DRAFT')).toBeVisible()

  await page.getByRole('link', { name: 'Анкета' }).click()
  await expect(page).toHaveURL(/step=questionnaire/)

  const sectionCreate = waitForMutationResponse(page, '/sections', 'POST')
  await page.getByRole('button', { name: 'Добавить раздел' }).click()
  expect((await sectionCreate).status()).toBe(201)

  const sectionTitlePatch = waitForMutationResponse(page, '/sections/', 'PATCH')
  await renameSectionTitle(page, SECTION_HSE)
  expect((await sectionTitlePatch).status()).toBeLessThan(400)
  await expect(page.getByLabel('Название раздела').first()).toHaveValue(SECTION_HSE)
  await expectAutosaveSaved(page)

  await addQuestion(page)
  await clickQuestionCard(page, 'Новый вопрос')
  const q1Patch = waitForMutationResponse(page, '/questions/', 'PATCH')
  await fillQuestionLabel(page, LABEL_ADR_AVAILABLE)
  await selectQuestionType(page, 'Да/Нет')
  await toggleQuestionRequired(page)
  expect((await q1Patch).status()).toBeLessThan(400)
  await expectAutosaveSaved(page)
  await expect(page.locator('.question-card').filter({ hasText: LABEL_ADR_AVAILABLE })).toBeVisible()

  await addQuestion(page)
  await clickQuestionCard(page, 'Новый вопрос')
  const q2Patch = waitForMutationResponse(page, '/questions/', 'PATCH')
  await fillQuestionLabel(page, LABEL_ADR_NUMBER)
  await selectQuestionType(page, 'Текст')
  expect((await q2Patch).status()).toBeLessThan(400)
  await expectAutosaveSaved(page)

  await addQuestion(page)
  await clickQuestionCard(page, 'Новый вопрос')
  const q3Patch = waitForMutationResponse(page, '/questions/', 'PATCH')
  await fillQuestionLabel(page, LABEL_ADR_EXPIRY)
  await selectQuestionType(page, 'Дата')
  expect((await q3Patch).status()).toBeLessThan(400)
  await expectAutosaveSaved(page)

  await addQuestion(page)
  await clickQuestionCard(page, 'Новый вопрос')
  const q4Patch = waitForMutationResponse(page, '/questions/', 'PATCH')
  await fillQuestionLabel(page, LABEL_SELECT)
  await selectQuestionType(page, 'Один из списка')
  expect((await q4Patch).status()).toBeLessThan(400)
  await expectAutosaveSaved(page)

  const opt1Create = waitForMutationResponse(page, '/options', 'POST')
  await page.getByRole('button', { name: 'Добавить опцию' }).click()
  expect((await opt1Create).status()).toBe(201)
  const opt2Create = waitForMutationResponse(page, '/options', 'POST')
  await page.getByRole('button', { name: 'Добавить опцию' }).click()
  expect((await opt2Create).status()).toBe(201)
  await expect(page.getByLabel('OPT_1')).toBeVisible()
  await expect(page.getByLabel('OPT_2')).toBeVisible()
  await expectAutosaveSaved(page)

  await clickQuestionCard(page, LABEL_ADR_NUMBER)
  const ruleCreate = waitForMutationResponse(page, '/rules', 'POST')
  await page.getByRole('button', { name: 'Добавить правило' }).click()
  expect((await ruleCreate).status()).toBe(201)
  await page.getByLabel('Действие').selectOption({ label: 'Сделать обязательным' })
  await page.getByLabel('Исходный вопрос').selectOption({ label: LABEL_ADR_AVAILABLE })
  await page.getByLabel('Оператор').selectOption({ label: 'Равно' })
  const rulePatch = waitForMutationResponse(page, '/rules/', 'PATCH')
  await page.getByLabel('Значение').fill('true')
  expect((await rulePatch).status()).toBeLessThan(400)
  await expectAutosaveSaved(page)

  await clickQuestionCard(page, LABEL_ADR_EXPIRY)
  await page.getByRole('button', { name: 'Добавить правило' }).click()
  await page.getByLabel('Действие').selectOption({ label: 'Сделать обязательным' })
  await page.getByLabel('Исходный вопрос').selectOption({ label: LABEL_ADR_AVAILABLE })
  await page.getByLabel('Оператор').selectOption({ label: 'Равно' })
  const rule2Patch = waitForMutationResponse(page, '/rules/', 'PATCH')
  await page.getByLabel('Значение').fill('true')
  expect((await rule2Patch).status()).toBeLessThan(400)
  await expectAutosaveSaved(page)

  const savePatch = waitForMutationResponse(page, '/questions/', 'PATCH')
  await fillQuestionLabel(page, LABEL_ADR_AVAILABLE)
  expect((await savePatch).status()).toBeLessThan(400)
  await expect(page.locator('.save-status')).toHaveText(/Сохранение|Сохранено/i)
  await expectAutosaveSaved(page)

  await page.reload({ waitUntil: 'domcontentloaded' })
  await waitForStudioLoad(page)
  await page.getByRole('link', { name: 'Анкета' }).click()
  await expect(page.getByLabel('Название раздела').first()).toHaveValue(SECTION_HSE)
  await expect(page.locator('.question-card').filter({ hasText: LABEL_ADR_AVAILABLE })).toBeVisible()
  await expect(page.locator('.question-card').filter({ hasText: LABEL_ADR_NUMBER })).toBeVisible()
  await expect(page.locator('.question-card').filter({ hasText: LABEL_ADR_EXPIRY })).toBeVisible()
  await clickQuestionCard(page, LABEL_ADR_NUMBER)
  await expect(page.getByLabel('Действие')).toHaveValue('REQUIRE')

  const adrCard = page.locator('.question-card').filter({ hasText: LABEL_ADR_AVAILABLE })
  page.once('dialog', (dialog) => dialog.accept())
  const deleteResp = waitForMutationResponse(page, '/questions/', 'DELETE')
  await adrCard.getByRole('button', { name: 'Удалить' }).click()
  expect((await deleteResp).status()).toBeLessThan(400)
  await expectAutosaveSaved(page)

  await clickQuestionCard(page, LABEL_ADR_NUMBER)
  const invalidPatch = waitForMutationResponse(page, '/rules/', 'PATCH')
  await page.getByLabel('Значение').first().fill('trigger-invalid')
  expect((await invalidPatch).status()).toBeGreaterThanOrEqual(400)
  await expectAutosaveInvalid(page)

  await page.reload({ waitUntil: 'domcontentloaded' })
  await waitForStudioLoad(page)
  await page.getByRole('link', { name: 'Анкета' }).click()
  await expect(page.locator('.question-card').filter({ hasText: LABEL_ADR_NUMBER })).toBeVisible()
  await expect(page.locator('.question-card').filter({ hasText: LABEL_ADR_AVAILABLE })).toHaveCount(0)

  const validateResp = waitForMutationResponse(page, '/validate-publish', 'POST')
  await page.getByRole('button', { name: 'Проверить' }).click()
  expect((await validateResp).status()).toBe(200)
  await expect(page).toHaveURL(/step=validation/)
  await expect(page.getByText('Проверка RFx')).toBeVisible()
  const readiness = await (await validateResp).json()
  expect(Array.isArray(readiness.items)).toBe(true)

  await page.getByRole('button', { name: 'Предпросмотр' }).click()
  await expect(page).toHaveURL(new RegExp(`/rfx/${eventId}/studio/preview`))
  await expect(page.getByText('Предпросмотр для перевозчика')).toBeVisible()
  await expect(page.getByText(SECTION_HSE)).toBeVisible()
  await expect(page.getByText(LABEL_ADR_NUMBER)).toBeVisible()
  await expect(page.getByText(LABEL_ADR_EXPIRY)).toBeVisible()
  await expect(page.getByText('Только просмотр — ответы не сохраняются')).toBeVisible()

  expect(gatewayFailures).toEqual([])
})
