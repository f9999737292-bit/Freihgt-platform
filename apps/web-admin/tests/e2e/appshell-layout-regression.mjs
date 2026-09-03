/**
 * AppShell layout regression — fake local session only, no credentials.
 */
import { mkdirSync } from 'node:fs'
import { join } from 'node:path'
import { chromium } from 'playwright'

const BASE = process.env.PILOT_UI_BASE_URL || 'http://localhost:13000'
const TENANT = process.env.TENANT_W2 || '285f9447-faf7-423e-96dd-e4c5e2b3fc6c'
const OUT_DIR = process.env.DIAGNOSTIC_OUT_DIR || join(process.cwd(), 'artifacts', 'appshell-regression')

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

async function cssText(page, selector) {
  return page.evaluate((sel) => {
    const el = document.querySelector(sel)
    if (!el) return null
    const cs = getComputedStyle(el)
    const rect = el.getBoundingClientRect()
    return {
      display: cs.display,
      top: rect.top,
      left: rect.left,
      width: rect.width,
      height: rect.height,
    }
  }, selector)
}

async function inspectRoute(page, route, screenshotName) {
  await page.goto(`${BASE}${route}`, { waitUntil: 'networkidle', timeout: 60000 })
  mkdirSync(OUT_DIR, { recursive: true })
  await page.screenshot({ path: join(OUT_DIR, screenshotName), fullPage: true })

  const prefix = route === '/dashboard' ? 'DASHBOARD' : 'RFX'
  const appShell = await cssText(page, '.app-shell')
  const sidebar = await cssText(page, '.sidebar')
  const main = await cssText(page, '.app-shell__main')
  const header = await cssText(page, '.header')
  const content = await cssText(page, '.app-shell__content')

  record(`${prefix}_APP_SHELL_VISIBLE`, appShell?.display === 'flex' ? 'PASS' : 'FAIL')
  record(`${prefix}_SIDEBAR_LEFT_ALIGNED`, sidebar && sidebar.left < 100 ? 'PASS' : 'FAIL')
  record(`${prefix}_HEADER_VISIBLE`, header && header.height > 0 && header.top < 200 ? 'PASS' : 'FAIL')
  record(`${prefix}_MAIN_VISIBLE`, main && main.height > 0 && main.top < 200 ? 'PASS' : 'FAIL')

  if (route === '/rfx') {
    const createBtn = page.getByRole('button', { name: /Создать RFx|Create RFx/i })
    record('RFX_CONTENT_VISIBLE', content && content.height > 0 ? 'PASS' : 'FAIL')
    record('CREATE_RFX_BUTTON_VISIBLE', (await createBtn.count()) > 0 ? 'PASS' : 'FAIL')
  }

  return { appShell, sidebar, main, header, content }
}

async function main() {
  const browser = await chromium.launch({ channel: 'chrome', headless: true })
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } })
  await context.addInitScript((session) => {
    localStorage.setItem('freight_admin_session', JSON.stringify(session))
    localStorage.setItem('freight_admin_tenant', JSON.stringify({ tenantId: session.user.tenant_id }))
  }, fakeSession)

  const page = await context.newPage()
  const dash = await inspectRoute(page, '/dashboard', 'dashboard-regression.png')
  const rfx = await inspectRoute(page, '/rfx', 'rfx-regression.png')
  const createBtnCount = await page.getByRole('button', { name: /Создать RFx|Create RFx/i }).count()

  await browser.close()

  const pass =
    dash.appShell?.display === 'flex' &&
    dash.sidebar && dash.sidebar.left < 100 &&
    dash.header && dash.header.top < 200 &&
    dash.main && dash.main.top < 200 &&
    rfx.appShell?.display === 'flex' &&
    rfx.header && rfx.header.top < 200 &&
    rfx.content && rfx.content.height > 0 &&
    createBtnCount > 0

  process.exit(pass ? 0 : 1)
}

main().catch((err) => {
  console.error(String(err.message || err))
  process.exit(1)
})
