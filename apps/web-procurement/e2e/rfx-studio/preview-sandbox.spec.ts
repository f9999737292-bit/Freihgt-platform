import { expect, test, type Page } from '@playwright/test'
import {
  configureRequireRule,
  clickAndWaitForMutation,
  clickQuestionCard,
  companyId,
  eventId,
  fillQuestionLabel,
  gatewayURL,
  jwt,
  openQuestionnaireStep,
  questionCard,
  renameSectionTitle,
  seedBuyerSession,
  selectQuestionType,
  studioPath,
  tenantId,
  toggleQuestionRequired,
  userId,
  waitForStudioLoad,
} from './helpers'

const SECTION_HSE = 'HSE'
const LABEL_ADR_AVAILABLE = 'ADR available?'
const LABEL_ADR_NUMBER = 'ADR number'
const LABEL_ADR_EXPIRY = 'ADR expiry date'
const LABEL_FLEET_COUNT = 'Fleet count'

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

async function patchQuestionValidation(page: Page, questionCode: string, validation: Record<string, unknown>) {
  const studioResp = await page.request.get(`${gatewayURL}/api/v1/rfx-events/${eventId}/studio`, {
    headers: {
      Authorization: `Bearer ${jwt}`,
      'X-Company-ID': companyId,
    },
  })
  expect(studioResp.ok()).toBeTruthy()
  const studio = await studioResp.json()
  const question = studio.sections
    ?.flatMap((s: { questions: Array<{ id: string; question_code: string }> }) => s.questions)
    ?.find((q: { question_code: string }) => q.question_code === questionCode)
  if (!question?.id) throw new Error(`question ${questionCode} not found`)
  const patch = await page.request.patch(
    `${gatewayURL}/api/v1/rfx-events/${eventId}/questions/${question.id}`,
    {
      headers: {
        Authorization: `Bearer ${jwt}`,
        'X-Company-ID': companyId,
      },
      data: { validation_rule_json: validation },
    },
  )
  expect(patch.ok()).toBeTruthy()
}

async function buildHsePreviewFixture(page: Page) {
  await openQuestionnaireStep(page)
  await clickAndWaitForMutation(page, '/sections', 'POST', () =>
    page.getByRole('button', { name: 'Добавить раздел' }).click(),
  )
  await clickAndWaitForMutation(page, '/sections/', 'PATCH', () => renameSectionTitle(page, SECTION_HSE))

  await addQuestion(page)
  await clickAndWaitForMutation(page, '/questions/', 'PATCH', async () => {
    await fillQuestionLabel(page, LABEL_ADR_AVAILABLE)
    await selectQuestionType(page, 'Да/Нет')
    await toggleQuestionRequired(page)
  })

  await addQuestion(page)
  await clickAndWaitForMutation(page, '/questions/', 'PATCH', async () => {
    await fillQuestionLabel(page, LABEL_ADR_NUMBER)
    await selectQuestionType(page, 'Текст')
  })

  await addQuestion(page)
  await clickAndWaitForMutation(page, '/questions/', 'PATCH', async () => {
    await fillQuestionLabel(page, LABEL_ADR_EXPIRY)
    await selectQuestionType(page, 'Дата')
  })

  await addQuestion(page)
  await clickAndWaitForMutation(page, '/questions/', 'PATCH', async () => {
    await fillQuestionLabel(page, LABEL_FLEET_COUNT)
    await selectQuestionType(page, 'Число')
  })
  await patchQuestionValidation(page, 'Q_4', { min_value: 0 })

  await clickQuestionCard(page, LABEL_ADR_NUMBER)
  await clickAndWaitForMutation(page, '/rules', 'POST', () =>
    page.getByRole('button', { name: 'Добавить правило' }).click(),
  )
  await clickAndWaitForMutation(page, '/rules/', 'PATCH', () => configureRequireRule(page, LABEL_ADR_AVAILABLE))

  await clickQuestionCard(page, LABEL_ADR_EXPIRY)
  await clickAndWaitForMutation(page, '/rules', 'POST', () =>
    page.getByRole('button', { name: 'Добавить правило' }).click(),
  )
  await clickAndWaitForMutation(page, '/rules/', 'PATCH', () => configureRequireRule(page, LABEL_ADR_AVAILABLE))
}

test.beforeEach(async ({ page }) => {
  test.setTimeout(300_000)
  if (!jwt || !tenantId || !eventId || !userId || !gatewayURL) {
    test.skip(true, 'browser E2E env not configured')
  }
  await seedBuyerSession(page)
})

test('F102-002 buyer preview-as-carrier sandbox is data-only', async ({ page }) => {
  const carrierWriteRequests: string[] = []
  page.on('request', (req) => {
    const url = req.url()
    if (CARRIER_WRITE_FRAGMENTS.some((frag) => url.includes(frag))) {
      carrierWriteRequests.push(`${req.method()} ${url}`)
    }
  })

  await page.goto('/login', { waitUntil: 'domcontentloaded' })
  await page.goto(studioPath(), { waitUntil: 'domcontentloaded' })
  await waitForStudioLoad(page)
  await buildHsePreviewFixture(page)

  await page.goto(studioPath('/preview'), { waitUntil: 'domcontentloaded' })
  await expect(page.getByTestId('enter-carrier-preview-sandbox')).toBeVisible({ timeout: 60_000 })
  await page.getByTestId('enter-carrier-preview-sandbox').click()
  await expect(page.getByTestId('carrier-preview-sandbox')).toBeVisible()
  await expect(page.getByText('Режим предпросмотра')).toBeVisible()

  const adrSection = page.getByTestId('preview-section-SEC_1')
  await expect(adrSection.getByTestId('preview-question-Q_1')).toBeVisible()
  await adrSection.getByTestId('preview-question-Q_1').getByText('Да').click()

  await expect(adrSection.getByTestId('preview-question-Q_2')).toBeVisible()
  await expect(adrSection.getByTestId('preview-question-Q_3')).toBeVisible()

  const fleetInput = adrSection.getByTestId('preview-question-Q_4').locator('input[type="number"]')
  await fleetInput.fill('-1')
  await fleetInput.blur()
  await page.getByTestId('preview-check-submit').click()
  await expect(page.getByTestId('preview-submit-blocked')).toBeVisible()
  await expect(page.getByTestId('preview-inline-errors')).toBeVisible()

  await fleetInput.fill('10')
  await page.getByTestId('preview-check-submit').click()
  await expect(page.getByTestId('preview-submit-blocked')).toBeVisible()

  const globalSummary = page.getByTestId('preview-global-summary')
  await expect(globalSummary.getByRole('button', { name: 'Перейти' }).first()).toBeVisible()
  await globalSummary.getByRole('button', { name: 'Перейти' }).first().click()

  await adrSection.getByTestId('preview-question-Q_3').locator('input[type="date"]').fill('2026-12-31')
  await adrSection.getByTestId('preview-question-Q_2').locator('input[type="text"]').fill('ADR-001')

  await page.getByTestId('preview-check-submit').click()
  await expect(page.getByTestId('preview-submit-success')).toBeVisible()
  await expect(page.getByTestId('preview-submit-blocked')).toHaveCount(0)

  await page.getByTestId('preview-close').click()
  await expect(page.getByTestId('enter-carrier-preview-sandbox')).toBeVisible()

  expect(carrierWriteRequests).toEqual([])
})
