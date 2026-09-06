import { expect, type Page } from '@playwright/test'

export const jwt = process.env.BROWSER_E2E_JWT || ''
export const tenantId = process.env.BROWSER_E2E_TENANT_ID || ''
export const companyId = process.env.BROWSER_E2E_BUYER_COMPANY_ID || ''
export const eventId = process.env.BROWSER_E2E_EVENT_ID || ''
export const rfxNumber = process.env.BROWSER_E2E_RFX_NUMBER || ''
export const userId = process.env.BROWSER_E2E_USER_ID || ''
export const gatewayURL = process.env.BROWSER_E2E_GATEWAY_URL || ''

export function assertGatewayHost(url: string) {
  if (!gatewayURL) {
    throw new Error('BROWSER_E2E_GATEWAY_URL is required')
  }
  const expected = new URL(gatewayURL)
  const actual = new URL(url)
  if (actual.host !== expected.host) {
    throw new Error(`expected gateway host ${expected.host}, got ${actual.host} for ${url}`)
  }
}

export async function seedBuyerSession(page: Page) {
  await page.addInitScript(({ token, tenant, company, user }) => {
    localStorage.setItem(
      'freight_admin_session',
      JSON.stringify({
        token,
        user: {
          id: user,
          tenant_id: tenant,
          email: 'buyer-studio-e2e@freight.test',
          full_name: 'Buyer Studio E2E',
          preferred_locale: 'ru-RU',
          status: 'ACTIVE',
          roles: ['PROCUREMENT_MANAGER'],
        },
      }),
    )
    localStorage.setItem('freight_admin_tenant_id', tenant)
    localStorage.setItem('freight_admin_company_id', company)
    document.cookie = 'freight_admin_locale=ru-RU; path=/'
  }, { token: jwt, tenant: tenantId, company: companyId, user: userId })
}

export async function expectAutosaveSaved(page: Page) {
  const status = page.locator('.save-status')
  await expect(status).not.toHaveText(/Есть изменения|Сохранение/i, { timeout: 30_000 })
  await expect(status).toHaveText(/Сохранено/i, { timeout: 30_000 })
}

export async function expectAutosaveInvalid(page: Page) {
  const status = page.locator('.save-status')
  await expect(status).toHaveClass(/save-status--error/, { timeout: 30_000 })
  await expect(status).not.toHaveText(/Сохранено/i)
}

export function studioPath(suffix = '') {
  return `/rfx/${eventId}/studio${suffix}`
}

export async function waitForStudioLoad(page: Page) {
  return page.waitForResponse(
    (resp) => {
      const url = resp.url()
      return (
        url.includes(`/api/v1/rfx-events/${eventId}/studio`)
        && !url.includes('tenant_id=')
        && resp.status() < 500
      )
    },
    { timeout: 120_000 },
  )
}

export async function openQuestionnaireStep(page: Page) {
  await page.goto(studioPath('?step=questionnaire'), { waitUntil: 'domcontentloaded' })
  await expect(page.getByRole('button', { name: 'Добавить раздел' })).toBeVisible({ timeout: 60_000 })
}

export async function clickAndWaitForMutation(
  page: Page,
  fragment: string,
  method: string,
  click: () => Promise<void>,
) {
  const [response] = await Promise.all([
    page.waitForResponse(
      (resp) => resp.url().includes(fragment) && resp.request().method() === method,
      { timeout: 60_000 },
    ),
    click(),
  ])
  return response
}

export async function selectQuestionType(page: Page, label: string) {
  await page.getByLabel('Тип вопроса').selectOption({ label })
}

export async function fillQuestionLabel(page: Page, label: string) {
  const input = page.getByLabel('Текст вопроса')
  await input.fill(label)
  await input.blur()
}

export async function toggleQuestionRequired(page: Page) {
  await page.getByLabel('Обязательный вопрос').check()
}

export function questionCard(page: Page, label: string) {
  const escaped = label.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  return page.locator('.question-card').filter({
    has: page.locator('strong', { hasText: new RegExp(`^${escaped}$`) }),
  })
}

export async function clickQuestionCard(page: Page, label: string) {
  const card = questionCard(page, label).first()
  await card.scrollIntoViewIfNeeded()
  await card.locator('strong').click()
  await expect(card).toHaveClass(/question-card--selected/, { timeout: 15_000 })
  await expect(page.locator('.property-panel').getByLabel('Текст вопроса')).toHaveValue(label, { timeout: 15_000 })
}

export async function selectRuleSourceQuestion(page: Page, questionLabel: string) {
  const select = page.getByLabel('Исходный вопрос')
  const option = select.locator('option').filter({ hasText: questionLabel }).first()
  await expect(option).toHaveCount(1, { timeout: 15_000 })
  const optionLabel = (await option.textContent())?.trim()
  if (!optionLabel) {
    throw new Error(`No rule source option found for ${questionLabel}`)
  }
  await select.selectOption({ label: optionLabel })
}

export async function configureRequireRule(page: Page, sourceQuestionLabel: string, value = 'true') {
  await page.getByLabel('Действие').selectOption({ label: 'Сделать обязательным' })
  await selectRuleSourceQuestion(page, sourceQuestionLabel)
  await page.getByLabel('Оператор').selectOption({ label: 'Равно' })
  await page.getByLabel('Значение').fill(value)
}

export async function renameSectionTitle(page: Page, title: string) {
  const input = page.getByLabel('Название раздела').first()
  await input.fill(title)
  await input.blur()
}

export async function patchQuestionValidation(
  page: Page,
  questionCode: string,
  validation: Record<string, unknown>,
) {
  if (!gatewayURL || !jwt || !companyId || !eventId) {
    throw new Error('browser E2E env not configured for patchQuestionValidation')
  }
  const studioResp = await page.request.get(`${gatewayURL}/api/v1/rfx-events/${eventId}/studio`, {
    headers: {
      Authorization: `Bearer ${jwt}`,
      'X-Company-ID': companyId,
    },
  })
  if (!studioResp.ok()) {
    throw new Error(`studio load failed: ${studioResp.status()}`)
  }
  const studio = await studioResp.json()
  const question = studio.sections
    ?.flatMap((s: { questions: Array<{ id: string; question_code: string; version: number }> }) => s.questions)
    ?.find((q: { question_code: string }) => q.question_code === questionCode)
  if (!question?.id) throw new Error(`question ${questionCode} not found`)
  const patch = await page.request.patch(
    `${gatewayURL}/api/v1/rfx-events/${eventId}/questions/${question.id}`,
    {
      headers: {
        Authorization: `Bearer ${jwt}`,
        'X-Company-ID': companyId,
      },
      data: { validation_rule_json: validation, expected_version: question.version },
    },
  )
  if (!patch.ok()) {
    throw new Error(`question patch failed: ${patch.status()}`)
  }
}
