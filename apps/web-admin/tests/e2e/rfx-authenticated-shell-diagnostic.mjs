/**
 * Authenticated RFx shell diagnostic using injected local session (no real credentials).
 */
import { chromium } from 'playwright'

const BASE = process.env.PILOT_UI_BASE_URL || 'http://localhost:13000'
const TENANT = process.env.TENANT_W2 || '285f9447-faf7-423e-96dd-e4c5e2b3fc6c'

function record(key, value) {
  console.log(`${key}=${value}`)
}

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

async function main() {
  const browser = await chromium.launch({ channel: 'chrome', headless: true })
  const context = await browser.newContext()
  await context.addInitScript((session) => {
    localStorage.setItem('freight_admin_session', JSON.stringify(session))
    localStorage.setItem('freight_admin_tenant', JSON.stringify({ tenantId: session.user.tenant_id }))
  }, fakeSession)

  const page = await context.newPage()
  let firstConsole = ''
  let firstPage = ''
  let failedResource = ''
  let failedStatus = ''
  const failedRequests = []

  page.on('console', (msg) => {
    if (msg.type() === 'error' && !firstConsole) firstConsole = msg.text()
  })
  page.on('pageerror', (err) => {
    if (!firstPage) firstPage = String(err.message || err)
  })
  page.on('requestfailed', (req) => {
    const url = req.url()
    if (url.includes('/_nuxt/')) {
      failedRequests.push(`${req.failure()?.errorText || 'FAILED'} ${url}`)
      if (!failedResource) {
        failedResource = url
        failedStatus = req.failure()?.errorText || 'FAILED'
      }
    }
  })
  page.on('response', (resp) => {
    const url = resp.url()
    if (url.includes('/_nuxt/') && resp.status() >= 400) {
      failedRequests.push(`${resp.status()} ${url}`)
    }
  })

  await page.goto(`${BASE}/dashboard`, { waitUntil: 'networkidle', timeout: 60000 })
  record('DASHBOARD_SHELL_RENDER', (await page.locator('.app-shell').count()) > 0 ? 'PASS' : 'FAIL')
  record('DASHBOARD_HEADER_VISIBLE', (await page.locator('.app-header').count()) > 0 ? 'PASS' : 'FAIL')

  await page.goto(`${BASE}/rfx`, { waitUntil: 'networkidle', timeout: 60000 })
  const bodyText = await page.locator('body').innerText()

  record('APP_SHELL_DOM_PRESENT', (await page.locator('.app-shell').count()) > 0 ? 'YES' : 'NO')
  record('SIDEBAR_DOM_PRESENT', (await page.locator('.sidebar').count()) > 0 ? 'YES' : 'NO')
  record('MAIN_DOM_PRESENT', (await page.locator('.app-shell__main').count()) > 0 ? 'YES' : 'NO')
  record('HEADER_DOM_PRESENT', (await page.locator('.app-header').count()) > 0 ? 'YES' : 'NO')
  record('CONTENT_DOM_PRESENT', (await page.locator('.page-stack, .ui-page-header').count()) > 0 ? 'YES' : 'NO')
  record('CREATE_BUTTON_VISIBLE', (await page.getByRole('button', { name: /Создать RFx|Create RFx/i }).count()) > 0 ? 'PASS' : 'FAIL')
  record('RFX_SHELL_RENDER', (await page.locator('.page-stack').count()) > 0 ? 'PASS' : 'FAIL')
  record('RFX_CONTENT_VISIBLE', /RFx|Тендер|Создать RFx|Поиск/i.test(bodyText) ? 'PASS' : 'FAIL')
  record('BODY_HAS_INVALID', /(^|\s)Invalid(\s|$)/i.test(bodyText) ? 'YES' : 'NO')
  record('FIRST_BROWSER_ERROR', firstConsole || 'NONE')
  record('FIRST_PAGE_ERROR', firstPage || 'NONE')
  record('FAILED_RESOURCE', failedResource || 'NONE')
  record('FAILED_RESOURCE_STATUS', failedStatus || 'NONE')
  record('NUXT_ASSET_FAILURES', failedRequests.length ? failedRequests.slice(0, 5).join(' | ') : 'NONE')

  await browser.close()

  const pass =
    (await page.locator('.app-shell__main').count()) > 0 &&
    (await page.locator('.page-stack').count()) > 0 &&
    failedRequests.length === 0 &&
    !firstPage
  process.exit(pass ? 0 : 1)
}

main().catch((err) => {
  record('DIAGNOSTIC', 'FAIL')
  console.error(String(err.message || err))
  process.exit(1)
})
