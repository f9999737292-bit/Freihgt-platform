import { expect, test } from '@playwright/test'
import {
  assertGatewayHost,
  carrierCompanyId,
  eventId,
  eventTitle,
  expectAutosaveInvalid,
  expectAutosaveSaved,
  fillDateQuestion,
  fillNumberQuestion,
  fillTextQuestion,
  flushAutosave,
  gatewayURL,
  jwt,
  questionnairePath,
  questionRoot,
  rfxNumber,
  seedCarrierSession,
  setYesNo,
  stubCarrierEventMetadata,
  stubCarrierCompanyContext,
  tenantId,
  userId,
  waitForAnswersPatch,
  waitForCarrierWorkspaceLoad,
  waitForCarrierWorkspace,
} from './helpers'

test.beforeEach(async ({ page }) => {
  test.setTimeout(300_000)
  if (!jwt || !tenantId || !eventId || !userId || !carrierCompanyId) {
    test.skip(true, 'browser E2E env not configured')
  }
  page.on('console', (msg) => {
    if (msg.type() === 'error') {
      console.error(`[browser-console] ${msg.text()}`)
    }
  })
  await seedCarrierSession(page)
  await stubCarrierEventMetadata(page)
  await stubCarrierCompanyContext(page)
})

test('F103-002 RFx carrier response save_version conflict', async ({ page }) => {
  await page.goto('/login', { waitUntil: 'domcontentloaded' })
  await page.goto(questionnairePath(), { waitUntil: 'domcontentloaded' })
  await waitForCarrierWorkspace(page)

  await setYesNo(page, 'ADR_AVAILABLE', true)
  await fillTextQuestion(page, 'ADR_NUMBER', 'ADR-CONFLICT')
  await fillDateQuestion(page, 'ADR_EXPIRY', '2030-06-15')
  await fillNumberQuestion(page, 'FLEET_COUNT', '5')
  await flushAutosave(page)

  const workspace = await page.evaluate(async ({ gw, ev, company, token, tenant, user }) => {
    const url = `${gw}/api/v1/rfx-events/${ev}/carrier-response?carrier_company_id=${company}`
    const resp = await fetch(url, {
      headers: {
        Authorization: `Bearer ${token}`,
        'X-Company-ID': company,
        'X-Tenant-ID': tenant,
        'X-User-ID': user,
      },
    })
    return resp.json()
  }, {
    gw: gatewayURL,
    ev: eventId,
    company: carrierCompanyId,
    token: jwt,
    tenant: tenantId,
    user: userId,
  })

  const fleetMeta = await page.evaluate((ws) => {
    for (const swq of ws.questionnaire?.sections ?? []) {
      for (const q of swq.questions ?? []) {
        if (q.question_code === 'FLEET_COUNT') {
          return { questionId: q.id, sectionId: swq.section.id }
        }
      }
    }
    return null
  }, workspace)
  expect(fleetMeta).not.toBeNull()

  const currentVersion = Number(workspace.save_version ?? 0)
  const staleVersion = Math.max(0, currentVersion - 1)
  const conflictStatus = await page.evaluate(async ({ gw, ev, company, token, tenant, user, version }) => {
    const url = `${gw}/api/v1/rfx-events/${ev}/carrier-response/answers?carrier_company_id=${company}`
    const resp = await fetch(url, {
      method: 'PATCH',
      headers: {
        Authorization: `Bearer ${token}`,
        'Content-Type': 'application/json',
        'X-Company-ID': company,
        'X-Tenant-ID': tenant,
        'X-User-ID': user,
      },
      body: JSON.stringify({ save_version: version, answers: [] }),
    })
    return resp.status
  }, {
    gw: gatewayURL,
    ev: eventId,
    company: carrierCompanyId,
    token: jwt,
    tenant: tenantId,
    user: userId,
    version: staleVersion,
  })
  expect(conflictStatus).toBe(409)

  const bumpStatus = await page.evaluate(async ({
    gw, ev, company, token, tenant, user, version, questionId, sectionId,
  }) => {
    const url = `${gw}/api/v1/rfx-events/${ev}/carrier-response/answers?carrier_company_id=${company}`
    const resp = await fetch(url, {
      method: 'PATCH',
      headers: {
        Authorization: `Bearer ${token}`,
        'Content-Type': 'application/json',
        'X-Company-ID': company,
        'X-Tenant-ID': tenant,
        'X-User-ID': user,
      },
      body: JSON.stringify({
        save_version: version,
        answers: [{ question_id: questionId, section_id: sectionId, value: 5 }],
      }),
    })
    return resp.status
  }, {
    gw: gatewayURL,
    ev: eventId,
    company: carrierCompanyId,
    token: jwt,
    tenant: tenantId,
    user: userId,
    version: currentVersion,
    questionId: fleetMeta!.questionId,
    sectionId: fleetMeta!.sectionId,
  })
  expect(bumpStatus).toBe(200)

  const conflictPromise = page.waitForResponse(
    (resp) =>
      resp.url().includes('/carrier-response/answers')
      && resp.status() === 409,
    { timeout: 60_000 },
  )
  await fillNumberQuestion(page, 'FLEET_COUNT', '6')
  await conflictPromise
  await expect(page.getByTestId('conflict-banner')).toBeVisible()
})

test('F103-001 RFx carrier response live browser acceptance', async ({ page }) => {
  const gatewayFailures: string[] = []
  page.on('response', (resp) => {
    const url = resp.url()
    if (!url.includes('/api/v1/rfx-events/') || url.includes('/api/v1/rfx-events/' + eventId + '"')) {
      return
    }
    if (!url.includes(`/api/v1/rfx-events/${eventId}/`)) {
      return
    }
    if (resp.status() === 404 || resp.status() >= 500) {
      gatewayFailures.push(`${resp.status()} ${resp.request().method()} ${url}`)
    }
  })

  const loadPromise = waitForCarrierWorkspaceLoad(page)
  await page.goto('/login', { waitUntil: 'domcontentloaded' })
  await page.goto(questionnairePath(), { waitUntil: 'domcontentloaded' })
  await expect(page).not.toHaveURL(/\/login(?:\?|$)/)
  const loadResponse = await loadPromise
  assertGatewayHost(loadResponse.url())
  expect(loadResponse.status()).toBe(200)

  await waitForCarrierWorkspace(page)
  await expect(page.getByText(rfxNumber)).toBeVisible()
  await expect(page.getByText(eventTitle)).toBeVisible()
  await expect(page.getByTestId('section-nav-HSE')).toBeVisible()
  await expect(page.getByTestId('section-HSE')).toBeVisible()

  await setYesNo(page, 'ADR_AVAILABLE', true)
  await expect(questionRoot(page, 'ADR_NUMBER')).toBeVisible()
  await expect(questionRoot(page, 'ADR_EXPIRY')).toBeVisible()

  await fillTextQuestion(page, 'ADR_NUMBER', 'ADR-12345')
  const saveTen = waitForAnswersPatch(page)
  await fillNumberQuestion(page, 'FLEET_COUNT', '10')
  const patchTen = await saveTen
  expect(patchTen.status()).toBe(200)
  assertGatewayHost(patchTen.url())
  await flushAutosave(page)

  const invalidPatch = waitForAnswersPatch(page)
  await fillNumberQuestion(page, 'FLEET_COUNT', '-1')
  const patchInvalid = await invalidPatch
  expect(patchInvalid.status()).toBe(422)
  assertGatewayHost(patchInvalid.url())
  await expectAutosaveInvalid(page)

  await page.reload({ waitUntil: 'domcontentloaded' })
  await waitForCarrierWorkspace(page)
  await expect(questionRoot(page, 'FLEET_COUNT').locator('input[type="number"]')).toHaveValue('10')

  const saveTwenty = waitForAnswersPatch(page)
  await fillNumberQuestion(page, 'FLEET_COUNT', '20')
  const patchTwenty = await saveTwenty
  expect(patchTwenty.status()).toBe(200)
  await flushAutosave(page)

  await page.goto(`/carrier/tenders/${eventId}`, { waitUntil: 'domcontentloaded' })
  await page.goto(questionnairePath(), { waitUntil: 'domcontentloaded' })
  await waitForCarrierWorkspace(page)
  await expect(questionRoot(page, 'FLEET_COUNT').locator('input[type="number"]')).toHaveValue('20')

  page.once('dialog', (dialog) => dialog.accept())
  await page.getByTestId('submit-questionnaire').click()
  await expect(page.getByTestId('submit-blocked')).toBeVisible({ timeout: 30_000 })

  await fillDateQuestion(page, 'ADR_EXPIRY', '2030-12-31')
  await flushAutosave(page)

  page.once('dialog', (dialog) => dialog.accept())
  const submitPromise = page.waitForResponse(
    (resp) =>
      resp.url().includes(`/api/v1/rfx-events/${eventId}/carrier-response/submit`)
      && resp.request().method() === 'POST',
    { timeout: 60_000 },
  )
  await page.getByTestId('submit-questionnaire').click()
  const submitResp = await submitPromise
  expect(submitResp.status()).toBe(200)
  assertGatewayHost(submitResp.url())

  await expect(page.getByTestId('post-submit-lock')).toBeVisible()
  await expect(page.getByTestId('submit-questionnaire')).toHaveCount(0)

  const patchDenied = await page.evaluate(async ({ gw, ev, company, token, tenant, user }) => {
    const url = `${gw}/api/v1/rfx-events/${ev}/carrier-response/answers?carrier_company_id=${company}`
    const resp = await fetch(url, {
      method: 'PATCH',
      headers: {
        Authorization: `Bearer ${token}`,
        'Content-Type': 'application/json',
        'X-Company-ID': company,
        'X-Tenant-ID': tenant,
        'X-User-ID': user,
      },
      body: JSON.stringify({ save_version: 99, answers: [] }),
    })
    return resp.status
  }, {
    gw: gatewayURL,
    ev: eventId,
    company: carrierCompanyId,
    token: jwt,
    tenant: tenantId,
    user: userId,
  })
  expect(patchDenied).toBeGreaterThanOrEqual(400)

  expect(gatewayFailures).toEqual([])
})
