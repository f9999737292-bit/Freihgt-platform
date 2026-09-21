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
  attachLiveDiagnostics,
  clickAndCapture,
  dumpCarrierOfferEvidence,
  dumpTenderDetailEvidence,
  dumpWizardEvidence,
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

  const membershipsWait = waitForApi(buyerPage, {
    method: 'GET',
    pathIncludes: `/api/v1/users/${buyerUserId}/companies`,
  })
  const companiesWait = waitForApi(buyerPage, { method: 'GET', pathIncludes: '/api/v1/companies', status: 200 })
  await buyerPage.goto(`${procurementURL}/tenders/new`, { waitUntil: 'domcontentloaded' })
  const memberships = await membershipsWait
  expect(memberships.status(), `GET user companies -> ${memberships.status()}`).toBe(200)
  const membershipBody = await memberships.json() as { items?: Array<{ company_id?: string }> }
  expect(membershipBody.items?.length, `user companies ${JSON.stringify(membershipBody)}`).toBeGreaterThan(0)
  const titleInput = buyerPage.getByTestId('wizard-title')
  await expect(titleInput).toBeVisible({ timeout: 30_000 })
  await titleInput.fill(TITLE)
  const owner = buyerPage.getByTestId('wizard-owner-company')
  await expect(owner.locator('option')).not.toHaveCount(0, { timeout: 15_000 })
  if (!(await owner.inputValue())) {
    const first = await owner.locator('option').nth(0).getAttribute('value')
    if (first) await owner.selectOption(first)
  }
  await expect(titleInput).toHaveValue(TITLE)
  await expect(owner).toHaveValue(/.+/)
  attachLiveDiagnostics(buyerPage, 'MAIN')
  const forbiddenHits: string[] = []
  for (const page of [buyerPage, adminPage, carrierPage]) {
    page.on('request', (req) => {
      const url = req.url()
      if (url.includes('/integrations/erp/')) {
        forbiddenHits.push(`${req.method()} ${url}`)
      }
    })
  }
  const afterClick: string[] = []
  buyerPage.on('request', (req) => {
    afterClick.push(`${req.method()} ${req.url()}`)
  })
  const before = await dumpWizardEvidence(buyerPage)
  expect(before.modelTitle, `Vue title model before Next: ${JSON.stringify(before)}`).toBe(TITLE)
  expect(before.modelOwner, `Vue owner model before Next: ${JSON.stringify(before)}`).toBeTruthy()
  expect(before.nextDisabled).toBe(false)
  const createWait = waitForApi(buyerPage, { method: 'POST', pathIncludes: '/api/v1/rfx-events' })
  await buyerPage.getByTestId('wizard-next').click()
  await dumpWizardEvidence(buyerPage)
  console.log(`E7-MAIN-AFTER-CLICK-REQUESTS ${JSON.stringify(afterClick)}`)
  const created = await createWait
  expect(created.status(), `POST create event -> ${created.status()}`).toBe(201)
  assertGatewayHost(created.url())
  const createdBody = await created.json() as { id: string; rfx_number: string; creation_channel?: string; status?: string }
  const eventId = createdBody.id
  expect(eventId).toBeTruthy()
  logStage({ stage: 'create-draft', method: 'POST', path: '/api/v1/rfx-events', status: created.status() })
  console.log(`E7-MAIN-EVENT-ID ${eventId} stage=create-draft`)

  await buyerPage.getByTestId('wizard-lot-name').fill(LOT_NAME)
  const lotResp = await clickAndCapture(
    buyerPage,
    'add-lot',
    { method: 'POST', pathIncludes: `/api/v1/rfx-events/${eventId}/lots` },
    () => buyerPage.getByRole('button', { name: /Add lot|Добавить лот|添加标段/ }).click(),
  )
  expect(lotResp.status()).toBe(201)
  const lotBody = await lotResp.json() as { id?: string; name?: string }
  expect(lotBody.id, `add-lot 201 missing id: ${JSON.stringify(lotBody)}`).toBeTruthy()
  const lotId = lotBody.id as string
  console.log(`E7-MAIN-EVENT-ID ${eventId} stage=add-lot lotId=${lotId}`)

  const companies = await companiesWait
  const companiesURL = new URL(companies.url())
  console.log(`E7-COMPANIES ${companies.request().method()} ${companiesURL.pathname}${companiesURL.search} -> ${companies.status()}`)
  expect(companies.status(), `GET /api/v1/companies -> ${companies.status()}`).toBe(200)
  expect(companiesURL.searchParams.get('company_type')).toBe('CARRIER')
  expect(companiesURL.searchParams.get('status')).toBe('ACTIVE')
  const companiesBody = await companies.json() as { items?: Array<{ legal_name?: string; company_type?: string; status?: string }> }
  const carrierA = (companiesBody.items ?? []).find((item) => item.legal_name === 'Carrier A' && item.company_type === 'CARRIER')
  expect(carrierA, `Carrier A missing in ${JSON.stringify({ path: companiesURL.pathname, query: companiesURL.search, count: companiesBody.items?.length })}`).toBeTruthy()

  await buyerPage.getByRole('button', { name: /Next|Далее|下一步/ }).click()
  const participantSelect = buyerPage.getByTestId('wizard-participant-company')
  await expect(participantSelect.locator('option')).not.toHaveCount(0, { timeout: 15_000 })
  const optionLabels = await participantSelect.locator('option').allTextContents()
  console.log(`E7-PARTICIPANT-OPTIONS ${JSON.stringify(optionLabels)}`)
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
  console.log(`E7-MAIN-EVENT-ID ${eventId} stage=add-participant`)

  await buyerPage.getByRole('button', { name: /Next|Далее|下一步/ }).click()
  const deadlineResp = await clickAndCapture(
    buyerPage,
    'save-deadline',
    { method: 'PATCH', pathIncludes: `/api/v1/rfx-events/${eventId}` },
    () => buyerPage.getByRole('button', { name: /Next|Далее|下一步/ }).click(),
  )
  expect(deadlineResp.status()).toBeLessThan(400)
  console.log(`E7-MAIN-EVENT-ID ${eventId} stage=save-deadline`)
  await expect(buyerPage.getByRole('button', { name: /Publish|Опубликовать|发布/ })).toBeVisible()

  const sessionBeforeDetail = await buyerPage.evaluate(() => {
    const raw = localStorage.getItem('freight_procurement_session')
    const parsed = raw ? JSON.parse(raw) as { user?: { id?: string; roles?: string[] } } : null
    return { userId: parsed?.user?.id ?? null, roles: parsed?.user?.roles ?? null }
  })
  console.log(`E7-SESSION-BEFORE-DETAIL ${JSON.stringify(sessionBeforeDetail)}`)
  expect(sessionBeforeDetail.roles).toEqual(['PROCUREMENT_MANAGER'])

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
  expect(humanBody.id, 'human GET event id drifted').toBe(eventId)
  expect(humanBody.creation_channel).toBe('MANUAL')
  expect(humanBody.status).toBe('DRAFT')
  logStage({ stage: 'human-get', method: 'GET', path: `/api/v1/rfx-events/${eventId}`, status: human.status() })
  console.log(`E7-MAIN-EVENT-ID ${eventId} stage=human-get`)
  await expect(buyerPage.getByTestId('tender-creation-channel')).toHaveText(/Created manually|Создан вручную|手动创建/)
  const humanDetail = await dumpTenderDetailEvidence(buyerPage)
  expect(humanDetail.roles).toEqual(['PROCUREMENT_MANAGER'])
  expect(humanDetail.headerTag, `PageHeader unresolved: ${JSON.stringify(humanDetail)}`).toBe('DIV')
  expect(humanDetail.hasBack, `unconditional Back missing: ${JSON.stringify(humanDetail)}`).toBe(true)

  const studioLoad = waitForApi(adminPage, { method: 'GET', pathIncludes: `/api/v1/rfx-events/${eventId}/studio` })
  await adminPage.goto(`${adminURL}/rfx/${eventId}/studio?step=questionnaire`, { waitUntil: 'domcontentloaded' })
  const studio = await studioLoad
  expect(studio.status()).toBe(200)
  assertGatewayHost(studio.url())
  logStage({ stage: 'studio-open', method: 'GET', path: `/api/v1/rfx-events/${eventId}/studio`, status: studio.status() })
  console.log(`E7-MAIN-EVENT-ID ${eventId} stage=studio-open`)
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
  await expect(adminPage.getByTestId('studio-validation-title')).toBeVisible({ timeout: 30_000 })
  await adminPage.locator('textarea').first().fill('E7 questionnaire publish')
  const publishQ = await clickAndCapture(
    adminPage,
    'publish-questionnaire',
    { method: 'POST', pathIncludes: `/api/v1/rfx-events/${eventId}/questionnaire/publish` },
    () => adminPage.getByTestId('studio-publish-questionnaire').click(),
  )
  expect(publishQ.status()).toBe(200)
  console.log(`E7-MAIN-EVENT-ID ${eventId} stage=publish-questionnaire`)

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
  console.log(`E7-MAIN-EVENT-ID ${eventId} stage=publish-score-model`)

  const publishPageGet = waitForApi(buyerPage, {
    method: 'GET',
    pathIncludes: `/api/v1/rfx-events/${eventId}`,
    pathExcludes: ['/lots', '/participants', '/xlsx', '/studio', '/publish', '/questionnaire', '/score-model'],
  })
  await buyerPage.goto(`${procurementURL}/tenders/${eventId}`, { waitUntil: 'domcontentloaded' })
  const publishPage = await publishPageGet
  expect(publishPage.status()).toBe(200)
  const publishPageBody = await publishPage.json() as { id?: string; status?: string }
  expect(publishPageBody.id, 'pre-publish GET event id drifted').toBe(eventId)
  expect(publishPageBody.status, 'event must stay DRAFT until UI PublishEvent').toBe('DRAFT')
  const publishDetail = await dumpTenderDetailEvidence(buyerPage)
  expect(publishDetail.roles).toEqual(['PROCUREMENT_MANAGER'])
  expect(publishDetail.hasBack).toBe(true)
  await expect(buyerPage.getByTestId('tender-publish')).toBeVisible({ timeout: 15_000 })
  const publishEvent = await clickAndCapture(
    buyerPage,
    'publish-event',
    { method: 'POST', pathIncludes: `/api/v1/rfx-events/${eventId}/publish` },
    () => buyerPage.getByTestId('tender-publish').click(),
  )
  expect(publishEvent.status()).toBe(200)
  const publishBody = await publishEvent.json().catch(() => ({})) as { id?: string; status?: string }
  if (publishBody.id) expect(publishBody.id, 'publish event id drifted').toBe(eventId)
  console.log(`E7-MAIN-EVENT-ID ${eventId} stage=publish-event status=${publishBody.status ?? 'unknown'}`)

  attachLiveDiagnostics(carrierPage, 'CARRIER')
  await carrierPage.goto(`${procurementURL}/carrier/tenders/${eventId}`, { waitUntil: 'domcontentloaded' })
  const startResp = await clickAndCapture(
    carrierPage,
    'carrier-start',
    { method: 'POST', pathIncludes: `/api/v1/rfx-events/${eventId}/responses` },
    () => carrierPage.getByRole('button', { name: /Start response|Начать ответ|开始响应/ }).click(),
  )
  expect(startResp.status(), `POST start response -> ${startResp.status()}`).toBe(201)
  const startBody = await startResp.json().catch(() => ({})) as { id?: string; rfx_event_id?: string; status?: string }
  expect(startBody.id, `start response 201 missing id: ${JSON.stringify(startBody)}`).toBeTruthy()
  const responseId = startBody.id as string
  if (startBody.rfx_event_id) expect(startBody.rfx_event_id, 'carrier start event id drifted').toBe(eventId)
  expect(startBody.status ?? 'DRAFT').toBe('DRAFT')
  console.log(`E7-MAIN-EVENT-ID ${eventId} stage=carrier-start responseId=${responseId}`)

  const offerInput = carrierPage.getByTestId(`carrier-offer-lot-${lotId}`)
  const saveOffer = carrierPage.getByTestId('carrier-save-offer')
  await dumpCarrierOfferEvidence(carrierPage, lotId)
  await expect(offerInput).toBeVisible({ timeout: 15_000 })
  await expect(saveOffer).toBeVisible()
  await expect(saveOffer).toBeEnabled({ timeout: 15_000 })
  const beforeFill = await dumpCarrierOfferEvidence(carrierPage, lotId)
  expect(beforeFill.lotInput?.lotId ?? beforeFill.lotInput?.testid, `lot offer input mismatch: ${JSON.stringify(beforeFill)}`).toContain(lotId)
  await offerInput.fill('15000')
  await expect(offerInput).toHaveValue('15000')
  const afterFill = await dumpCarrierOfferEvidence(carrierPage, lotId)
  expect(afterFill.lotInput?.value, `DOM value after fill: ${JSON.stringify(afterFill)}`).toBe('15000')
  await offerInput.blur()
  await expect(offerInput).toHaveValue('15000')
  const afterBlur = await dumpCarrierOfferEvidence(carrierPage, lotId)
  expect(afterBlur.lotInput?.value, `DOM value after blur: ${JSON.stringify(afterBlur)}`).toBe('15000')
  expect(afterBlur.lotInput?.modelAmount, `Vue model after blur: ${JSON.stringify(afterBlur)}`).toBe('15000')
  expect(afterBlur.toasts.some((text) => /amount for every lot|сумм|每个批次/i.test(text))).toBe(false)
  await expect(saveOffer).toBeEnabled()

  const prematureSubmit: string[] = []
  carrierPage.on('request', (req) => {
    if (req.method() === 'POST' && /\/rfx-responses\/[^/]+\/submit|\/carrier-response\/submit/.test(req.url())) {
      prematureSubmit.push(`${req.method()} ${req.url()}`)
    }
  })
  const offerResp = await clickAndCapture(
    carrierPage,
    'save-offer',
    { method: 'PATCH', pathIncludes: `/api/v1/rfx-responses/${responseId}` },
    () => saveOffer.click(),
  )
  expect(offerResp.status(), `PATCH save offer -> ${offerResp.status()}`).toBeLessThan(400)
  expect(offerResp.url()).toContain(`/api/v1/rfx-responses/${responseId}`)
  const offerPayload = offerResp.request().postDataJSON() as {
    offer_lines?: Array<{ rfx_lot_id?: string; amount?: number; currency_code?: string }>
  }
  console.log(`E7-SAVE-OFFER-BODY ${JSON.stringify(offerPayload)}`)
  expect(offerPayload.offer_lines, `save-offer body: ${JSON.stringify(offerPayload)}`).toEqual([
    { rfx_lot_id: lotId, amount: 15000, currency_code: 'RUB' },
  ])
  const offerSaved = await offerResp.json().catch(() => ({})) as {
    id?: string
    rfx_event_id?: string
    status?: string
    offer_lines?: Array<{ rfx_lot_id?: string; amount?: number; currency_code?: string }>
  }
  expect(offerSaved.id ?? responseId).toBe(responseId)
  if (offerSaved.rfx_event_id) expect(offerSaved.rfx_event_id).toBe(eventId)
  expect(offerSaved.status ?? 'DRAFT', `response must stay DRAFT after commercial save: ${JSON.stringify(offerSaved)}`).toBe('DRAFT')
  const patchLine = (offerSaved.offer_lines ?? []).find((line) => line.rfx_lot_id === lotId)
  expect(patchLine, `PATCH response missing offer line: ${JSON.stringify(offerSaved)}`).toBeTruthy()
  expect(Number(patchLine?.amount)).toBe(15000)
  expect(prematureSubmit, `premature submit during save-offer: ${JSON.stringify(prematureSubmit)}`).toEqual([])
  const afterSaveToasts = await dumpCarrierOfferEvidence(carrierPage, lotId)
  expect(afterSaveToasts.toasts.some((text) => /amount for every lot|сумм|每个批次/i.test(text)), `offerLotRequired after save: ${JSON.stringify(afterSaveToasts)}`).toBe(false)
  console.log(`E7-MAIN-EVENT-ID ${eventId} stage=save-offer responseId=${responseId}`)

  const refreshGet = carrierPage.waitForResponse(async (resp) => {
    if (resp.request().method() !== 'GET' || !resp.url().includes(`/api/v1/rfx-events/${eventId}/own-response`) || resp.status() !== 200) {
      return false
    }
    const body = await resp.json().catch(() => null) as {
      offer_lines?: Array<{ rfx_lot_id?: string; amount?: number | string }>
    } | null
    return Boolean((body?.offer_lines ?? []).some((line) => line.rfx_lot_id === lotId && Number(line.amount) === 15000))
  }, { timeout: 60_000 })
  await carrierPage.goto(`${procurementURL}/carrier/tenders/${eventId}`, { waitUntil: 'domcontentloaded' })
  const refreshed = await refreshGet
  expect(refreshed.status()).toBe(200)
  const refreshedBody = await refreshed.json() as {
    id?: string
    rfx_event_id?: string
    status?: string
    offer_lines?: Array<{ rfx_lot_id?: string; amount?: number | string; currency_code?: string }>
  }
  expect(refreshedBody.id, 'own-response id drifted after save-offer').toBe(responseId)
  if (refreshedBody.rfx_event_id) expect(refreshedBody.rfx_event_id).toBe(eventId)
  expect(refreshedBody.status, `own-response status after save-offer: ${JSON.stringify(refreshedBody)}`).toBe('DRAFT')
  const savedLine = (refreshedBody.offer_lines ?? []).find((line) => line.rfx_lot_id === lotId)
  expect(savedLine, `saved offer line missing: ${JSON.stringify(refreshedBody)}`).toBeTruthy()
  expect(Number(savedLine?.amount)).toBe(15000)
  expect(prematureSubmit, `premature submit after save-offer refresh: ${JSON.stringify(prematureSubmit)}`).toEqual([])
  console.log(`E7-MAIN-EVENT-ID ${eventId} stage=save-offer-refresh responseId=${responseId}`)

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
  expect(submitQ.url()).toContain(eventId)
  console.log(`E7-MAIN-EVENT-ID ${eventId} stage=submit-questionnaire responseId=${responseId}`)

  await buyerPage.goto(`${procurementURL}/tenders/${eventId}/evaluation`, { waitUntil: 'domcontentloaded' })
  await expect(buyerPage.getByTestId('evaluation-comparison-table')).toBeVisible({ timeout: 60_000 })
  await expect(buyerPage.getByTestId('v3-questionnaire-score').first()).toBeVisible({ timeout: 120_000 })
  const awardButton = buyerPage.locator(`[data-testid="evaluation-award"][data-award-response-id="${responseId}"]`)
  await expect(awardButton).toBeVisible()
  const awardResp = await clickAndCapture(
    buyerPage,
    'award-open',
    { method: 'POST', pathIncludes: `/api/v1/rfx-events/${eventId}/award-response` },
    async () => {
      await awardButton.click()
      await buyerPage.getByTestId('evaluation-award-confirm').click()
    },
  )
  expect(awardResp.status(), `POST award-response -> ${awardResp.status()}`).toBe(200)
  expect(awardResp.url()).toContain(`/api/v1/rfx-events/${eventId}/award-response`)
  expect(awardResp.url()).not.toContain('/transport-orders')
  expect(awardResp.url()).not.toContain('/integrations/erp/')
  const awardPayload = awardResp.request().postDataJSON() as { response_id?: string }
  expect(awardPayload.response_id, `award body: ${JSON.stringify(awardPayload)}`).toBe(responseId)
  logStage({ stage: 'award', method: 'POST', path: `/api/v1/rfx-events/${eventId}/award-response`, status: awardResp.status() })
  console.log(`E7-MAIN-EVENT-ID ${eventId} stage=award eventId=${eventId} responseId=${responseId}`)

  expect(forbiddenHits, `forbidden ERP requests: ${JSON.stringify(forbiddenHits)}`).toEqual([])

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
