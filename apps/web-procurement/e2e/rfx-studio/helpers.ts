import { expect, type Page } from '@playwright/test'

export const jwt = process.env.BROWSER_E2E_JWT || ''
export const tenantId = process.env.BROWSER_E2E_TENANT_ID || ''
export const companyId = process.env.BROWSER_E2E_BUYER_COMPANY_ID || ''
export const eventId = process.env.BROWSER_E2E_EVENT_ID || ''
export const rfxNumber = process.env.BROWSER_E2E_RFX_NUMBER || ''
export const userId = process.env.BROWSER_E2E_USER_ID || ''

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
  await expect(status).toHaveText(/не прошли проверку|Данные не прошли/i, { timeout: 30_000 })
}

export function studioPath(suffix = '') {
  return `/rfx/${eventId}/studio${suffix}`
}

export async function waitForStudioLoad(page: Page) {
  return page.waitForResponse(
    (resp) => resp.url().includes(`/api/v1/rfx-events/${eventId}/studio`) && resp.status() === 200,
    { timeout: 60_000 },
  )
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

export async function clickQuestionCard(page: Page, label: string) {
  await page.locator('.question-card').filter({ hasText: label }).click()
}

export async function renameSectionTitle(page: Page, title: string) {
  const input = page.getByLabel('Название раздела').first()
  await input.fill(title)
  await input.blur()
}
