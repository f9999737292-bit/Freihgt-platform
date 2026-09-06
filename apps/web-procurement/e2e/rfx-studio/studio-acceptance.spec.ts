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
  patchQuestionValidation,
  renameSectionTitle,
  questionCard,
  rfxNumber,
  seedBuyerSession,
  selectQuestionType,
  studioPath,
  tenantId,
  toggleQuestionRequired,
  userId,
  waitForStudioLoad,
  assertGatewayHost,
} from './helpers'

const LABEL_ADR_AVAILABLE = 'ADR available?'
const LABEL_ADR_NUMBER = 'ADR number'
const LABEL_ADR_EXPIRY = 'ADR expiry date'
const LABEL_FLEET_COUNT = 'Fleet count'
const LABEL_SELECT = 'Transport mode'
const SECTION_HSE = 'HSE'

const CARRIER_WRITE_FRAGMENTS = [
  '/carrier-response/start',
  '/carrier-response/answers',
  '/carrier-response/validate',
  '/carrier-response/submit',
]

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
  assertGatewayHost(studioResponse.url())
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
  assertGatewayHost(sectionCreate.url())

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
  assertGatewayHost(q1Patch.url())
  await expectAutosaveSaved(page)
  await expect(page.getByLabel('Текст вопроса')).toHaveValue(LABEL_ADR_AVAILABLE)
  await expect(questionCard(page, LABEL_ADR_AVAILABLE)).toHaveCount(1)
  await expect(questionCard(page, LABEL_ADR_AVAILABLE).first()).toBeVisible()

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

  await addQuestion(page)
  const q5Patch = await clickAndWaitForMutation(
    page,
    '/questions/',
    'PATCH',
    async () => {
      await fillQuestionLabel(page, LABEL_FLEET_COUNT)
      await selectQuestionType(page, 'Число')
    },
  )
  expect(q5Patch.status()).toBeLessThan(400)
  await patchQuestionValidation(page, 'Q_5', { min_value: 0 })
  await expectAutosaveSaved(page)

  await clickQuestionCard(page, LABEL_ADR_AVAILABLE)
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
  await expect(questionCard(page, LABEL_ADR_AVAILABLE)).toHaveCount(1)
  await expect(questionCard(page, LABEL_ADR_AVAILABLE).first()).toBeVisible()
  await expect(questionCard(page, LABEL_ADR_NUMBER)).toBeVisible()
  await expect(questionCard(page, LABEL_ADR_EXPIRY)).toBeVisible()
  await clickQuestionCard(page, LABEL_ADR_NUMBER)
  await expect(page.getByLabel('Действие')).toHaveValue('REQUIRE')

  const carrierWriteRequests: string[] = []
  page.on('request', (req) => {
    const url = req.url()
    if (CARRIER_WRITE_FRAGMENTS.some((frag) => url.includes(frag))) {
      carrierWriteRequests.push(`${req.method()} ${url}`)
    }
  })

  await page.getByRole('button', { name: 'Предпросмотр' }).click()
  await expect(page).toHaveURL(new RegExp(`/rfx/${eventId}/studio/preview`))
  await expect(page.getByText('Предпросмотр для перевозчика')).toBeVisible()
  await expect(page.getByText('Только просмотр — ответы не сохраняются')).toBeVisible()
  await page.getByTestId('enter-carrier-preview-sandbox').click()
  await expect(page.getByTestId('carrier-preview-sandbox')).toBeVisible()
  await expect(page.getByText('Режим предпросмотра')).toBeVisible()

  const adrSection = page.getByTestId('preview-section-SEC_1')
  await adrSection.getByTestId('preview-question-Q_1').getByText('Да').click()
  await expect(adrSection.getByTestId('preview-question-Q_2')).toBeVisible()
  await expect(adrSection.getByTestId('preview-question-Q_3')).toBeVisible()

  const fleetInput = adrSection.getByTestId('preview-question-Q_5').locator('input[type="number"]')
  await fleetInput.fill('-1')
  await fleetInput.blur()
  await page.getByTestId('preview-check-submit').click()
  await expect(page.getByTestId('preview-submit-blocked')).toBeVisible()
  await expect(adrSection.getByTestId('preview-question-Q_5').getByTestId('preview-inline-errors')).toBeVisible()

  await fleetInput.fill('10')
  await page.getByTestId('preview-check-submit').click()
  await expect(page.getByTestId('preview-submit-blocked')).toBeVisible()
  await page.getByTestId('preview-global-summary').getByRole('button', { name: 'Перейти' }).first().click()
  await adrSection.getByTestId('preview-question-Q_3').locator('input[type="date"]').fill('2026-12-31')
  await adrSection.getByTestId('preview-question-Q_2').locator('input[type="text"]').fill('ADR-001')
  await page.getByTestId('preview-check-submit').click()
  await expect(page.getByTestId('preview-submit-success')).toBeVisible()
  await page.getByTestId('preview-close').click()
  expect(carrierWriteRequests).toEqual([])

  await page.goto(studioPath('?step=questionnaire'), { waitUntil: 'domcontentloaded' })
  await waitForStudioLoad(page)

  const adrCard = questionCard(page, LABEL_ADR_AVAILABLE)
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
    () => page.getByLabel('Значение').first().fill('false'),
  )
  expect(invalidPatch.status()).toBeGreaterThanOrEqual(400)
  await expectAutosaveInvalid(page)

  await page.reload({ waitUntil: 'domcontentloaded' })
  await waitForStudioLoad(page)
  await openQuestionnaireStep(page)
  await clickQuestionCard(page, LABEL_ADR_NUMBER)
  await expect(page.getByLabel('Значение').first()).toHaveValue('true')
  await expect(questionCard(page, LABEL_ADR_AVAILABLE)).toHaveCount(0)

  const validateResp = await clickAndWaitForMutation(
    page,
    '/validate-publish',
    'POST',
    () => page.getByRole('button', { name: 'Проверить' }).click(),
  )
  expect(validateResp.status()).toBe(200)
  assertGatewayHost(validateResp.url())
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
