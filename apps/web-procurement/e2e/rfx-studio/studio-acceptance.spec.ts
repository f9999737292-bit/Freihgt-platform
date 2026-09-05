import { expect, test, type Page } from '@playwright/test'
import {
  configureRequireRule,
  clickAndWaitForMutation,
  clickQuestionCard,
  eventId,
  expectAutosaveInvalid,
  expectAutosaveSaved,
  fillQuestionLabel,
  jwt,
  openQuestionnaireStep,
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
  await Promise.all([
    page.waitForResponse(
      (resp) => resp.url().includes('/questions') && resp.request().method() === 'POST' && resp.status() === 201,
      { timeout: 60_000 },
    ),
    page.getByRole('button', { name: 'Добавить вопрос' }).first().click(),
  ])
  await expect(page.getByLabel('Текст вопроса')).toHaveValue('Новый вопрос', { timeout: 15_000 })
}

test.beforeEach(async ({ page }) => {
  test.setTimeout(300_000)
  if (!jwt || !tenantId || !eventId || !userId) {
    test.skip(true, 'browser E2E env not configured')
  }
  page.on('console', (msg) => {
    if (msg.type() === 'error') {
      console.error(`[browser-console] ${msg.text()}`)
    }
  })
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
  await page.goto('/login', { waitUntil: 'domcontentloaded' })
  await page.goto(studioPath(), { waitUntil: 'domcontentloaded' })
  await expect(page).not.toHaveURL(/\/login(?:\?|$)/)
  await expect(page.getByRole('heading', { name: new RegExp(rfxNumber) })).toBeVisible({ timeout: 120_000 })
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

  await openQuestionnaireStep(page)

  const sectionCreate = await clickAndWaitForMutation(
    page,
    '/sections',
    'POST',
    () => page.getByRole('button', { name: 'Добавить раздел' }).click(),
  )
  expect(sectionCreate.status()).toBe(201)

  const sectionTitlePatch = await clickAndWaitForMutation(
    page,
    '/sections/',
    'PATCH',
    async () => {
      await renameSectionTitle(page, SECTION_HSE)
    },
  )
  expect(sectionTitlePatch.status()).toBeLessThan(400)
  await expect(page.getByLabel('Название раздела').first()).toHaveValue(SECTION_HSE)
  await expectAutosaveSaved(page)

  await addQuestion(page)
  const q1Patch = await clickAndWaitForMutation(
    page,
    '/questions/',
    'PATCH',
    async () => {
      await fillQuestionLabel(page, LABEL_ADR_AVAILABLE)
      await selectQuestionType(page, 'Да/Нет')
      await toggleQuestionRequired(page)
    },
  )
  expect(q1Patch.status()).toBeLessThan(400)
  await expectAutosaveSaved(page)
  await expect(page.getByLabel('Текст вопроса')).toHaveValue(LABEL_ADR_AVAILABLE)
  await expect(page.locator('.question-card').filter({ hasText: LABEL_ADR_AVAILABLE })).toBeVisible()

  await addQuestion(page)
  const q2Patch = await clickAndWaitForMutation(
    page,
    '/questions/',
    'PATCH',
    async () => {
      await fillQuestionLabel(page, LABEL_ADR_NUMBER)
      await selectQuestionType(page, 'Текст')
    },
  )
  expect(q2Patch.status()).toBeLessThan(400)
  await expectAutosaveSaved(page)

  await addQuestion(page)
  const q3Patch = await clickAndWaitForMutation(
    page,
    '/questions/',
    'PATCH',
    async () => {
      await fillQuestionLabel(page, LABEL_ADR_EXPIRY)
      await selectQuestionType(page, 'Дата')
    },
  )
  expect(q3Patch.status()).toBeLessThan(400)
  await expectAutosaveSaved(page)

  await addQuestion(page)
  const q4Patch = await clickAndWaitForMutation(
    page,
    '/questions/',
    'PATCH',
    async () => {
      await fillQuestionLabel(page, LABEL_SELECT)
      await selectQuestionType(page, 'Один из списка')
    },
  )
  expect(q4Patch.status()).toBeLessThan(400)
  await expectAutosaveSaved(page)

  const opt1Create = await clickAndWaitForMutation(
    page,
    '/options',
    'POST',
    () => page.getByRole('button', { name: 'Добавить опцию' }).click(),
  )
  expect(opt1Create.status()).toBe(201)
  const opt2Create = await clickAndWaitForMutation(
    page,
    '/options',
    'POST',
    () => page.getByRole('button', { name: 'Добавить опцию' }).click(),
  )
  expect(opt2Create.status()).toBe(201)
  await expect(page.getByLabel('OPT_1')).toBeVisible()
  await expect(page.getByLabel('OPT_2')).toBeVisible()
  await expectAutosaveSaved(page)

  await clickQuestionCard(page, LABEL_ADR_NUMBER)
  const ruleCreate = await clickAndWaitForMutation(
    page,
    '/rules',
    'POST',
    () => page.getByRole('button', { name: 'Добавить правило' }).click(),
  )
  expect(ruleCreate.status()).toBe(201)
  const rulePatch = await clickAndWaitForMutation(
    page,
    '/rules/',
    'PATCH',
    async () => {
      await configureRequireRule(page, LABEL_ADR_AVAILABLE)
    },
  )
  expect(rulePatch.status()).toBeLessThan(400)
  await expectAutosaveSaved(page)

  await clickQuestionCard(page, LABEL_ADR_EXPIRY)
  await page.getByRole('button', { name: 'Добавить правило' }).click()
  const rule2Patch = await clickAndWaitForMutation(
    page,
    '/rules/',
    'PATCH',
    async () => {
      await configureRequireRule(page, LABEL_ADR_AVAILABLE)
    },
  )
  expect(rule2Patch.status()).toBeLessThan(400)
  await expectAutosaveSaved(page)

  const savePatch = await clickAndWaitForMutation(
    page,
    '/questions/',
    'PATCH',
    () => fillQuestionLabel(page, LABEL_ADR_AVAILABLE),
  )
  expect(savePatch.status()).toBeLessThan(400)
  await expect(page.locator('.save-status')).toHaveText(/Сохранение|Сохранено/i)
  await expectAutosaveSaved(page)

  await page.reload({ waitUntil: 'domcontentloaded' })
  await waitForStudioLoad(page)
  await openQuestionnaireStep(page)
  await expect(page.getByLabel('Название раздела').first()).toHaveValue(SECTION_HSE)
  await expect(page.locator('.question-card').filter({ hasText: LABEL_ADR_AVAILABLE })).toBeVisible()
  await expect(page.locator('.question-card').filter({ hasText: LABEL_ADR_NUMBER })).toBeVisible()
  await expect(page.locator('.question-card').filter({ hasText: LABEL_ADR_EXPIRY })).toBeVisible()
  await clickQuestionCard(page, LABEL_ADR_NUMBER)
  await expect(page.getByLabel('Действие')).toHaveValue('REQUIRE')

  const adrCard = page.getByRole('button', { name: new RegExp(`^${LABEL_ADR_AVAILABLE.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}\\s`) })
  page.once('dialog', (dialog) => dialog.accept())
  const deleteResp = await clickAndWaitForMutation(
    page,
    '/questions/',
    'DELETE',
    () => adrCard.getByRole('button', { name: 'Удалить' }).click(),
  )
  expect(deleteResp.status()).toBeLessThan(400)
  await expectAutosaveSaved(page)

  await clickQuestionCard(page, LABEL_ADR_NUMBER)
  const invalidPatch = await clickAndWaitForMutation(
    page,
    '/rules/',
    'PATCH',
    () => page.getByLabel('Значение').first().fill('trigger-invalid'),
  )
  expect(invalidPatch.status()).toBeGreaterThanOrEqual(400)
  await expectAutosaveInvalid(page)

  await page.reload({ waitUntil: 'domcontentloaded' })
  await waitForStudioLoad(page)
  await openQuestionnaireStep(page)
  await expect(page.locator('.question-card').filter({ hasText: LABEL_ADR_NUMBER })).toBeVisible()
  await expect(page.locator('.question-card').filter({ hasText: LABEL_ADR_AVAILABLE })).toHaveCount(0)

  const validateResp = await clickAndWaitForMutation(
    page,
    '/validate-publish',
    'POST',
    () => page.getByRole('button', { name: 'Проверить' }).click(),
  )
  expect(validateResp.status()).toBe(200)
  await expect(page).toHaveURL(/step=validation/)
  await expect(page.getByText('Проверка RFx')).toBeVisible()
  const readiness = await validateResp.json()
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
