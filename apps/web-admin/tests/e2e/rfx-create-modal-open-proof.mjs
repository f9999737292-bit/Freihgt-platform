/**
 * RFx create modal open proof — no RFx mutation (close without save).
 * Credentials via env only; never logged.
 */
import { chromium } from 'playwright'

const BASE = process.env.PILOT_UI_BASE_URL || 'http://localhost:13000'
const tenant = process.env.TENANT_W2 || process.env.PILOT_TENANT_ID || ''
const email = process.env.BUYER_EMAIL || process.env.PILOT_TEST_EMAIL || ''
const password = process.env.BUYER_PASSWORD || process.env.PILOT_TEST_PASSWORD || ''

const out = {}
function record(key, value) {
  out[key] = value
  console.log(`${key}=${value}`)
}

let consoleErrors = []
let pageErrors = []
let vueWarnings = []

async function main() {
  if (!tenant || !email || !password) {
    record('BROWSER_PROOF', 'BLOCKED_MISSING_ENV')
    process.exit(2)
  }

  const browser = await chromium.launch({ channel: 'chrome', headless: true })
  const page = await browser.newPage()

  page.on('console', (msg) => {
    const text = msg.text()
    if (msg.type() === 'error') consoleErrors.push(text)
    if (/Vue warn|\[Vue warn\]/i.test(text)) vueWarnings.push(text)
  })
  page.on('pageerror', (err) => pageErrors.push(String(err.message || err)))

  await page.goto(`${BASE}/login`, { waitUntil: 'networkidle', timeout: 60000 })
  await page.locator('#login-tenant-id').fill(tenant)
  await page.locator('#login-email').fill(email)
  await page.locator('#login-password').fill(password)
  await page.locator('form.login-form button[type="submit"]').click()
  await page.waitForURL((url) => !url.pathname.includes('/login'), { timeout: 30000 })

  await page.goto(`${BASE}/rfx`, { waitUntil: 'networkidle', timeout: 60000 })

  const createButton = page.getByRole('button', { name: /Создать RFx|Create RFx/i })
  const dialogBefore = await page.locator('[role="dialog"]').count()

  record('BUTTON_VISIBLE', (await createButton.isVisible()) ? 'PASS' : 'FAIL')
  record('BUTTON_ENABLED', (await createButton.isEnabled()) ? 'PASS' : 'FAIL')

  await createButton.click()
  await page.waitForTimeout(1500)

  const dialogAfter = await page.locator('[role="dialog"]').count()
  const modalVisible = dialogAfter > dialogBefore

  record('BUTTON_CLICK_DISPATCHED', 'PASS')
  record('DIALOG_COUNT_BEFORE', String(dialogBefore))
  record('DIALOG_COUNT_AFTER', String(dialogAfter))
  record('CREATE_MODAL_VISIBLE', modalVisible ? 'PASS' : 'FAIL')
  record('BROWSER_CONSOLE_ERRORS', consoleErrors.length ? consoleErrors.slice(0, 5).join(' | ') : 'NONE')
  record('BROWSER_PAGE_ERRORS', pageErrors.length ? pageErrors.slice(0, 5).join(' | ') : 'NONE')
  record('VUE_WARNINGS', vueWarnings.length ? vueWarnings.slice(0, 5).join(' | ') : 'NONE')

  if (modalVisible) {
    const cancel = page.getByRole('button', { name: /Отмена|Cancel/i }).first()
    if (await cancel.isVisible().catch(() => false)) {
      await cancel.click()
      await page.waitForTimeout(500)
    }
  }

  record('REAL_RFX_CREATED', 'NO')
  await browser.close()

  const pass =
    (await createButton.isVisible()) &&
    (await createButton.isEnabled()) &&
    modalVisible &&
    consoleErrors.length === 0 &&
    pageErrors.length === 0
  process.exit(pass ? 0 : 1)
}

main().catch((err) => {
  record('BROWSER_PROOF', 'FAIL')
  console.error(String(err.message || err))
  process.exit(1)
})
