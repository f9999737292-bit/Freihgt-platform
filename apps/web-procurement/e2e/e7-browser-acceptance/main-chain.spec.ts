import { expect, test, type Page } from '@playwright/test'
import {
  adminURL,
  assertGatewayHost,
  buyerCompanyId,
  buyerJwt,
  buyerUserId,
  carrierCompanyId,
  carrierJwt,
  carrierUserId,
  clickAndCapture,
  gatewayURL,
  logStage,
  procurementURL,
  seedAdminSession,
  seedProcurementSession,
  waitForApi,
} from './helpers'

const TITLE = 'E7 Main Chain Live'
const LOT_NAME = 'E7 Main Lot'
const ADR_LABEL = 'ADR available?'
const FLEET_LABEL = 'Fleet count'

test.describe.configure({ mode: 'serial' })

test('E7-BRW-01 main chain on one event ID', async ({ browser }) => {
  const buyer = await browser.newContext()
  const admin = await browser.newContext()
  const carrier = await browser.newContext()
  const buyerPage = await buyer.newPage()
  const adminPage = await admin.newPage()
  const carrierPage = await carrier.newPage()

  await seedProcurementSession(buyerPage, {
    token: buyerJwt,
    user: buyerUserId,
    company: buyerCompanyId,
    roles: ['PROCUREMENT_MANAGER'],
  })
  await seedAdminSession(adminPage)
  await seedProcurementSession(carrierPage, {
    token: carrierJwt,
    user: carrierUserId,
    company: carrierCompanyId,
    roles: ['CARRIER_DISPATCHER'],
  })

  const createWait = waitForApi(buyerPage, { method: 'POST', pathIncludes: '/api/v1/rfx-events' })
  const membershipsWait = waitForApi(buyerPage, {
    method: 'GET',
    pathIncludes: `/api/v1/users/${buyerUserId}/companies`,
  })
  await buyerPage.goto(`${procurementURL}/tenders/new`, { waitUntil: 'domcontentloaded' })
  const memberships = await membershipsWait
  expect(memberships.status(), `GET user companies -> ${memberships.status()}`).toBe(200)
  const titleInput = buyerPage.getByTestId('wizard-title')
  await expect(titleInput).toBeVisible({ timeout: 30_000 })
  await titleInput.fill(TITLE)
  const owner = buyerPage.getByTestId('wizard-owner-company')
  await expect(owner.locator('option')).not.toHaveCount(0, { timeout: 15_000 })
  if (!(await owner.inputValue())) {
    const first = await owner.locator('option').nth(0).getAttribute('value')
    if (first) await owner.selectOption(first)
  }
  await buyerPage.getByRole('button', { name: /Next|Далее|下一步/ }).click()
  const created = await createWait
  expect(created.status(), `POST create event -> ${created.status()}`).toBe(201)
  assertGatewayHost(created.url())
  const createdBody = await created.json() as { id: string; rfx_number: string; creation_channel?: string; status?: string }
  const eventId = createdBody.id
  expect(eventId).toBeTruthy()
  logStage({ stage: 'create-draft', method: 'POST', path: '/api/v1/rfx-events', status: created.status() })
  console.log(`E7-MAIN-EVENT-ID ${eventId}`)

  await buyerPage.getByTestId('wizard-lot-name').fill(LOT_NAME)
  const lotResp = await clickAndCapture(
    buyerPage,
    'add-lot',
    { method: 'POST', pathIncludes: `/api/v1/rfx-events/${eventId}/lots` },
    () => buyerPage.getByRole('button', { name: /Add lot|Добавить лот|添加标段/ }).click(),
  )
  expect(lotResp.status()).toBe(201)

  await buyerPage.getByRole('button', { name: /Next|Далее|下一步/ }).click()
  const participantSelect = buyerPage.getByTestId('wizard-participant-company')
  await expect(participantSelect.locator('option')).not.toHaveCount(0, { timeout: 15_000 })
  const carrierOption = participantSelect.locator('option').filter({ hasText: /Carrier A/ }).first()
  await expect(carrierOption).toHaveCount(1, { timeout: 15_000 })
  await participantSelect.selectOption({ label: (await carrierOption.textContent())?.trim() || '' })
  const participantResp = await clickAndCapture(
    buyerPage,
    'add-participant',
    { method: 'POST', pathIncludes: `/api/v1/rfx-events/${eventId}/participants` },
    () => buyerPage.getByRole('button', { name: /Add participant|Добавить участника|添加参与者/ }).click(),
  )
  expect(participantResp.status()).toBe(201)

  await buyerPage.getByRole('button', { name: /Next|Далее|下一步/ }).click()
  const deadlineResp = await clickAndCapture(
    buyerPage,
    'save-deadline',
    { method: 'PATCH', pathIncludes: `/api/v1/rfx-events/${eventId}` },
    () => buyerPage.getByRole('button', { name: /Next|Далее|下一步/ }).click(),
  )
  expect(deadlineResp.status()).toBeLessThan(400)
  await expect(buyerPage.getByRole('button', { name: /Publish|Опубликовать|发布/ })).toBeVisible()

  const humanGet = waitForApi(buyerPage, {
    method: 'GET',
    pathIncludes: `/api/v1/rfx-events/${eventId}`,
    pathExcludes: ['/lots', '/participants', '/xlsx', '/studio'],
  })
  await buyerPage.goto(`${procurementURL}/tenders/${eventId}`, { waitUntil: 'domcontentloaded' })
  const human = await humanGet
  expect(human.status()).toBe(200)
  assertGatewayHost(human.url())
  const humanBody = await human.json() as { creation_channel?: string; status?: string; id?: string }
  expect(humanBody.id || eventId).toBe(eventId)
  expect(humanBody.creation_channel).toBe('MANUAL')
  expect(humanBody.status).toBe('DRAFT')
  logStage({ stage: 'human-get', method: 'GET', path: `/api/v1/rfx-events/${eventId}`, status: human.status() })
  await expect(buyerPage.getByTestId('tender-creation-channel')).toHaveText(/Created manually|Создан вручную|手动创建/)
  await expect(buyerPage.getByRole('button', { name: /Publish|Опубликовать|发布/ })).toBeVisible()

  const studioLoad = waitForApi(adminPage, { method: 'GET', pathIncludes: `/api/v1/rfx-events/${eventId}/studio` })
  await adminPage.goto(`${adminURL}/rfx/${eventId}/studio?step=questionnaire`, { waitUntil: 'domcontentloaded' })
  const studio = await studioLoad
  expect(studio.status()).toBe(200)
  assertGatewayHost(studio.url())
  logStage({ stage: 'studio-open', method: 'GET', path: `/api/v1/rfx-events/${eventId}/studio`, status: studio.status() })
  await expect(adminPage.getByRole('button', { name: 'Добавить раздел' })).toBeVisible({ timeout: 60_000 })

  const sectionResp = await clickAndCapture(
    adminPage,
    'create-section',
    { method: 'POST', pathIncludes: '/sections' },
    () => adminPage.getByRole('button', { name: 'Добавить раздел' }).click(),
  )
  expect(sectionResp.status()).toBe(201)
  await renameSection(adminPage, 'HSE')
  await addStudioQuestion(adminPage, ADR_LABEL, 'Да/Нет', true)
  await addStudioQuestion(adminPage, FLEET_LABEL, 'Число', false)

  await adminPage.goto(`${adminURL}/rfx/${eventId}/studio?step=validation`, { waitUntil: 'domcontentloaded' })
  await expect(adminPage.getByText('Проверка RFx')).toBeVisible({ timeout: 30_000 })
  await adminPage.locator('textarea').first().fill('E7 questionnaire publish')
  const publishQ = await clickAndCapture(
    adminPage,
    'publish-questionnaire',
    { method: 'POST', pathIncludes: `/api/v1/rfx-events/${eventId}/questionnaire/publish` },
    () => adminPage.getByRole('button', { name: 'Опубликовать опросник' }).click(),
  )
  expect(publishQ.status()).toBe(200)

  await adminPage.goto(`${adminURL}/rfx/${eventId}/studio?step=scoring`, { waitUntil: 'domcontentloaded' })
  await expect(adminPage.getByTestId('rfx-scoring-workspace')).toBeVisible({ timeout: 120_000 })
  await expect(adminPage.getByTestId('scoring-add-criterion')).toBeEnabled({ timeout: 60_000 })
  await addCriterion(adminPage, 1)
  await addCriterion(adminPage, 2)
  await adminPage.getByTestId('scoring-criterion-code').nth(0).fill('HSE')
  await adminPage.getByTestId('scoring-criterion-name').nth(0).fill('HSE')
  await adminPage.getByTestId('scoring-criterion-weight').nth(0).fill('40')
  await adminPage.getByTestId('scoring-criterion-code').nth(1).fill('CAPACITY')
  await adminPage.getByTestId('scoring-criterion-name').nth(1).fill('Capacity')
  await adminPage.getByTestId('scoring-criterion-weight').nth(1).fill('60')
  const bindings = adminPage.getByTestId('scoring-question-binding')
  await expect(bindings.nth(0).locator('option')).toHaveCount(3, { timeout: 60_000 })
  await bindings.nth(0).selectOption({ index: 1 })
  await bindings.nth(1).selectOption({ index: 2 })
  await adminPage.getByTestId('scoring-criterion-card').nth(0).getByTestId('scoring-knockout-boolean-false').check()
  const saveScore = adminPage.waitForResponse((resp) =>
    resp.request().method() === 'PUT' && resp.url().includes('/score-model') && resp.status() < 500,
  )
  await adminPage.getByTestId('scoring-save-draft').click()
  await saveScore
  if (await adminPage.getByTestId('scoring-validate').isVisible()) {
    const validateScore = adminPage.waitForResponse((resp) =>
      resp.url().includes('/score-model/validate') && resp.request().method() === 'POST',
    )
    await adminPage.getByTestId('scoring-validate').click()
    await validateScore
  }
  await adminPage.getByTestId('scoring-publish').click()
  const publishScore = await clickAndCapture(
    adminPage,
    'publish-score-model',
    { method: 'POST', pathIncludes: `/api/v1/rfx-events/${eventId}/score-model/publish` },
    () => adminPage.getByTestId('scoring-publish-confirm').click(),
  )
  expect(publishScore.status()).toBe(200)
  await expect(adminPage.getByTestId('scoring-published-lock')).toBeVisible({ timeout: 60_000 })

  const publishEvent = await clickAndCapture(
    buyerPage,
    'publish-event',
    { method: 'POST', pathIncludes: `/api/v1/rfx-events/${eventId}/publish` },
    () => buyerPage.getByRole('button', { name: /Publish|Опубликовать|发布/ }).click(),
  )
  expect(publishEvent.status()).toBe(200)

  await carrierPage.goto(`${procurementURL}/carrier/tenders/${eventId}`, { waitUntil: 'domcontentloaded' })
  const startResp = await clickAndCapture(
    carrierPage,
    'carrier-start',
    { method: 'POST', pathIncludes: `/api/v1/rfx-events/${eventId}/responses` },
    () => carrierPage.getByRole('button', { name: /Start response|Начать ответ|开始响应/ }).click(),
  )
  expect(startResp.status()).toBeLessThan(400)
  const offerInput = carrierPage.locator('input[type="number"]').first()
  await expect(offerInput).toBeVisible({ timeout: 15_000 })
  await offerInput.fill('15000')
  const offerResp = await clickAndCapture(
    carrierPage,
    'save-offer',
    { method: 'PATCH', pathIncludes: '/api/v1/rfx-responses/' },
    () => carrierPage.getByRole('button', { name: /Save offer|Сохранить предложение|保存报价/ }).click(),
  )
  expect(offerResp.status()).toBeLessThan(400)

  await carrierPage.goto(`${procurementURL}/carrier/tenders/${eventId}/questionnaire`, { waitUntil: 'domcontentloaded' })
  await expect(carrierPage.getByTestId('carrier-response-workspace')).toBeVisible({ timeout: 60_000 })
  const questionRoots = carrierPage.locator('[data-testid^="question-"]')
  await expect(questionRoots.first()).toBeVisible({ timeout: 30_000 })
  const yesRadio = questionRoots.first().locator('input[type="radio"]').first()
  await yesRadio.check()
  const numberInput = carrierPage.locator('[data-testid^="question-"] input[type="number"]').first()
  await numberInput.fill('50')
  await numberInput.blur()
  await carrierPage.waitForTimeout(1000)
  const submitQ = await clickAndCapture(
    carrierPage,
    'submit-questionnaire',
    { method: 'POST', pathIncludes: `/api/v1/rfx-events/${eventId}/carrier-response/submit` },
    () => carrierPage.getByTestId('submit-questionnaire').click(),
  )
  expect(submitQ.status()).toBe(200)

  await buyerPage.goto(`${procurementURL}/tenders/${eventId}/evaluation`, { waitUntil: 'domcontentloaded' })
  await expect(buyerPage.getByTestId('evaluation-comparison-table')).toBeVisible({ timeout: 60_000 })
  await expect(buyerPage.getByTestId('v3-questionnaire-score').first()).toBeVisible({ timeout: 120_000 })
  await expect(buyerPage.getByTestId('evaluation-award')).toBeVisible()
  const awardResp = await clickAndCapture(
    buyerPage,
    'award-open',
    { method: 'POST', pathIncludes: `/api/v1/rfx-events/${eventId}/award-response` },
    async () => {
      await buyerPage.getByTestId('evaluation-award').click()
      await buyerPage.getByTestId('evaluation-award-confirm').click()
    },
  )
  expect(awardResp.status(), `POST award-response -> ${awardResp.status()}`).toBe(200)
  logStage({ stage: 'award', method: 'POST', path: `/api/v1/rfx-events/${eventId}/award-response`, status: awardResp.status() })

  await buyer.close()
  await admin.close()
  await carrier.close()
})

async function renameSection(page: Page, title: string) {
  const input = page.getByLabel('Название раздела').first()
  await input.fill(title)
  await input.blur()
  await expect(page.locator('.save-status')).toHaveText(/Сохранено/i, { timeout: 30_000 })
}

async function addStudioQuestion(page: Page, label: string, typeLabel: string, required: boolean) {
  await Promise.all([
    page.waitForResponse((resp) => resp.url().includes('/questions') && resp.request().method() === 'POST' && resp.status() === 201, { timeout: 60_000 }),
    page.getByRole('button', { name: 'Добавить вопрос' }).first().click(),
  ])
  const text = page.getByLabel('Текст вопроса')
  await expect(text).toBeVisible({ timeout: 15_000 })
  await text.fill(label)
  await page.getByLabel('Тип вопроса').selectOption({ label: typeLabel })
  if (required) {
    await page.getByLabel('Обязательный вопрос').check()
  }
  await text.blur()
  await expect(page.locator('.save-status')).toHaveText(/Сохранено/i, { timeout: 30_000 })
}

async function addCriterion(page: Page, expected: number) {
  const cards = page.getByTestId('scoring-criterion-card')
  let current = await cards.count()
  while (current < expected) {
    await page.getByTestId('scoring-add-criterion').click()
    await expect(cards).toHaveCount(current + 1, { timeout: 15_000 })
    current += 1
  }
}
