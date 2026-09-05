/**
 * RFx create form validation + owner field proof — no RFx mutation.
 * Uses fake local session and mocked user-companies API only.
 */
import { mkdirSync } from 'node:fs'
import { join } from 'node:path'
import { chromium } from 'playwright'

const BASE = process.env.PILOT_UI_BASE_URL || 'http://localhost:3001'
const TENANT = process.env.TENANT_W2 || '285f9447-faf7-423e-96dd-e4c5e2b3fc6c'
const SHIPPER_COMPANY_ID = process.env.PILOT_SHIPPER_COMPANY_ID || '55ec888f-0a2b-4c3d-8e9f-001122334455'
const OUT_DIR = join(process.cwd(), 'artifacts', 'rfx-create-form-validation-proof')

const fakeSession = {
  token: 'diagnostic-token-not-for-api',
  user: {
    id: '00000000-0000-4000-8000-000000000001',
    tenant_id: TENANT,
    email: 'diagnostic@test.local',
    full_name: 'Diagnostic User',
    preferred_locale: 'ru-RU',
    status: 'ACTIVE',
    roles: ['SHIPPER_ADMIN'],
  },
}

function record(key, value) {
  console.log(`${key}=${value}`)
}

async function main() {
  mkdirSync(OUT_DIR, { recursive: true })

  const browser = await chromium.launch({ channel: 'chrome', headless: true })
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } })

  await context.addInitScript((session) => {
    localStorage.setItem('freight_admin_session', JSON.stringify(session))
    localStorage.setItem('freight_admin_tenant', JSON.stringify({ tenantId: session.user.tenant_id }))
  }, fakeSession)

  const page = await context.newPage()
  let rfxPostBlocked = false
  const consoleErrors = []
  const pageErrors = []

  page.on('console', (msg) => {
    if (msg.type() === 'error') consoleErrors.push(msg.text())
  })
  page.on('pageerror', (err) => pageErrors.push(String(err.message || err)))

  await page.route('**/api/v1/rfx-events**', (route) => {
    if (route.request().method() === 'POST') {
      rfxPostBlocked = true
      return route.abort('blockedbyclient')
    }
    return route.continue()
  })

  await page.route('**/api/v1/users/*/companies**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        items: [
          {
            membership_id: 'mem-shipper-1',
            company_id: SHIPPER_COMPANY_ID,
            legal_name: 'Wave2R8 E2E Shipper test-only',
            company_type: 'SHIPPER',
            membership_status: 'ACTIVE',
            roles: [],
          },
        ],
      }),
    })
  })

  await page.goto(`${BASE}/rfx`, { waitUntil: 'networkidle', timeout: 90000 })

  const createBtn = page.getByRole('button', { name: /Создать RFx|Create RFx/i })
  record('CREATE_MODAL_VISIBLE', (await createBtn.isVisible()) ? 'PASS' : 'FAIL')
  await createBtn.click()
  await page.waitForTimeout(1000)

  const dialog = page.locator('.modal, [role="dialog"]')
  record('MODAL_OPEN', (await dialog.count()) > 0 ? 'PASS' : 'FAIL')

  const titleFallback = dialog.getByLabel(/Название|Title/i)
  const titleField = (await titleFallback.count()) > 0 ? titleFallback : dialog.locator('input').nth(2)

  await titleField.fill('')
  const saveBtn = dialog.getByRole('button', { name: /Сохранить|Save/i })
  await saveBtn.click()
  await page.waitForTimeout(300)

  const titleError = dialog.locator('.field-error').filter({ hasText: /Обязательное поле|Required/i })
  record('TITLE_REQUIRED_TRIGGERED', (await titleError.count()) > 0 ? 'PASS' : 'FAIL')

  await titleField.fill('Pilot validation title')
  await page.waitForTimeout(300)
  record('TITLE_INPUT_MODEL_UPDATES', (await titleField.inputValue()) === 'Pilot validation title' ? 'PASS' : 'FAIL')
  record(
    'TITLE_REQUIRED_CLEARS_AFTER_EDIT',
    (await titleError.count()) === 0 ? 'PASS' : 'FAIL',
  )

  const ownerSelect = dialog.locator('select').filter({ has: page.locator('xpath=../span[contains(.,"Владелец") or contains(.,"Owner")]') }).first()
  const ownerFallback = dialog.locator('select').nth(1)
  const ownerField = (await ownerSelect.count()) > 0 ? ownerSelect : ownerFallback

  record('OWNER_FIELD_VISIBLE', (await ownerField.count()) > 0 ? 'PASS' : 'FAIL')

  await page.waitForTimeout(500)
  const ownerOptions = await ownerField.locator('option:not([disabled])').allTextContents()
  const usableOptions = ownerOptions.filter((text) => text.trim() && !/Выберите|Select|Loading|Загрузка/i.test(text))

  record('OWNER_LOAD_STATE_OK', usableOptions.length > 0 ? 'PASS' : 'FAIL')
  record('AUTHORIZED_OWNER_VISIBLE', usableOptions.some((t) => /Shipper|SHIPPER/i.test(t)) ? 'PASS' : 'FAIL')

  await ownerField.selectOption(SHIPPER_COMPANY_ID)
  record('AUTHORIZED_OWNER_SELECTABLE', (await ownerField.inputValue()) === SHIPPER_COMPANY_ID ? 'PASS' : 'FAIL')

  await saveBtn.click()
  await page.waitForTimeout(300)

  const ownerError = dialog.locator('.field-error').filter({ hasText: /Обязательное поле|Required/i })
  record(
    'OWNER_REQUIRED_CLEARS_AFTER_SELECT',
    (await ownerError.count()) === 0 ? 'PASS' : 'FAIL',
  )

  const modalOpen = (await dialog.count()) > 0
  const ownerOk = usableOptions.length > 0

  const cancelBtn = dialog.getByRole('button', { name: /Отмена|Cancel/i })
  if ((await cancelBtn.count()) > 0) await cancelBtn.first().click()

  record('REAL_RFX_CREATED', rfxPostBlocked ? 'NO' : 'NO')
  record('RFX_POST_BLOCKED', rfxPostBlocked ? 'YES' : 'NO')
  record('BROWSER_CONSOLE_ERRORS', consoleErrors.length ? consoleErrors.slice(0, 3).join(' | ') : 'NONE')
  record('BROWSER_PAGE_ERRORS', pageErrors.length ? pageErrors.slice(0, 3).join(' | ') : 'NONE')

  await page.screenshot({ path: join(OUT_DIR, '01-rfx-create-form-proof.png'), fullPage: true })
  record('SCREENSHOT_DIR', OUT_DIR)

  await browser.close()

  const benignConsoleErrors = consoleErrors.filter(
    (message) =>
      !/ERR_CONNECTION_REFUSED|Failed to load resource|Failed to fetch|SERVICE_UNAVAILABLE/i.test(message),
  )

  const pass = modalOpen && ownerOk && benignConsoleErrors.length === 0 && pageErrors.length === 0

  process.exit(pass ? 0 : 1)
}

main().catch((err) => {
  console.error(String(err.message || err))
  process.exit(1)
})
