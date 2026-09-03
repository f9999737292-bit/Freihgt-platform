/**
 * Global AppShell layout diagnostic — fake local session only, no credentials.
 */
import { createHash } from 'node:crypto'
import { mkdirSync } from 'node:fs'
import { join } from 'node:path'
import { chromium } from 'playwright'

const BASE = process.env.PILOT_UI_BASE_URL || 'http://localhost:13000'
const TENANT = process.env.TENANT_W2 || '285f9447-faf7-423e-96dd-e4c5e2b3fc6c'
const OUT_DIR = process.env.DIAGNOSTIC_OUT_DIR || join(process.cwd(), 'artifacts', 'appshell-diagnostic')

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
      position: cs.position,
      top: rect.top,
      left: rect.left,
      width: rect.width,
      height: rect.height,
      minHeight: cs.minHeight,
      visibility: cs.visibility,
      opacity: cs.opacity,
    }
  }, selector)
}

async function auditPageCss(path) {
  const res = await fetch(`${BASE}${path}`, { redirect: 'manual' })
  const html = await res.text()
  const assets = [...html.matchAll(/\/_nuxt\/[^"'\s>]+\.css/g)].map((m) => m[0])
  const unique = [...new Set(assets)]
  const report = []
  for (const asset of unique) {
    const url = `${BASE}${asset}`
    const cssRes = await fetch(url)
    const body = cssRes.ok ? await cssRes.text() : ''
    const hash = createHash('sha256').update(body).digest('hex').slice(0, 16)
    report.push({
      url: asset,
      status: cssRes.status,
      contentType: cssRes.headers.get('content-type') || '',
      cacheControl: cssRes.headers.get('cache-control') || '',
      size: body.length,
      hash,
      hasAppShell: /\.app-shell\b/.test(body),
      hasAppShellMain: /\.app-shell__main\b/.test(body),
      hasHeader: /\.header\b/.test(body),
      hasSidebar: /\.sidebar\b/.test(body),
    })
  }
  return { status: res.status, assets: report }
}

async function inspectRoute(page, route, screenshotName) {
  await page.goto(`${BASE}${route}`, { waitUntil: 'networkidle', timeout: 60000 })
  mkdirSync(OUT_DIR, { recursive: true })
  await page.screenshot({ path: join(OUT_DIR, screenshotName), fullPage: true })

  const prefix = route === '/dashboard' ? 'DASHBOARD' : 'RFX'
  const metrics = {
    appShell: await cssText(page, '.app-shell'),
    sidebar: await cssText(page, '.sidebar'),
    main: await cssText(page, '.app-shell__main'),
    header: await cssText(page, '.header'),
    content: await cssText(page, '.app-shell__content'),
    pageStack: await cssText(page, '.page-stack'),
  }

  record(`${prefix}_APP_SHELL_EXISTS`, metrics.appShell ? 'YES' : 'NO')
  if (metrics.appShell) {
    record(`${prefix}_APP_SHELL_DISPLAY`, metrics.appShell.display)
    record(`${prefix}_APP_SHELL_WIDTH`, String(Math.round(metrics.appShell.width)))
    record(`${prefix}_APP_SHELL_HEIGHT`, String(Math.round(metrics.appShell.height)))
  }

  record(`${prefix}_SIDEBAR_EXISTS`, metrics.sidebar ? 'YES' : 'NO')
  if (metrics.sidebar) {
    record(`${prefix}_SIDEBAR_POSITION`, metrics.sidebar.position)
    record(`${prefix}_SIDEBAR_WIDTH`, String(Math.round(metrics.sidebar.width)))
    record(`${prefix}_SIDEBAR_MIN_HEIGHT`, metrics.sidebar.minHeight)
  }

  record(`${prefix}_MAIN_EXISTS`, metrics.main ? 'YES' : 'NO')
  if (metrics.main) {
    record(`${prefix}_MAIN_DISPLAY`, metrics.main.display)
    record(`${prefix}_MAIN_TOP`, String(Math.round(metrics.main.top)))
    record(`${prefix}_MAIN_LEFT`, String(Math.round(metrics.main.left)))
    record(`${prefix}_MAIN_WIDTH`, String(Math.round(metrics.main.width)))
    record(`${prefix}_MAIN_HEIGHT`, String(Math.round(metrics.main.height)))
  }

  record(`${prefix}_HEADER_EXISTS`, metrics.header ? 'YES' : 'NO')
  if (metrics.header) {
    record(`${prefix}_HEADER_DISPLAY`, metrics.header.display)
    record(`${prefix}_HEADER_TOP`, String(Math.round(metrics.header.top)))
    record(`${prefix}_HEADER_LEFT`, String(Math.round(metrics.header.left)))
    record(`${prefix}_HEADER_WIDTH`, String(Math.round(metrics.header.width)))
  }

  record(`${prefix}_CONTENT_EXISTS`, metrics.content ? 'YES' : 'NO')
  if (metrics.content) {
    record(`${prefix}_CONTENT_DISPLAY`, metrics.content.display)
    record(`${prefix}_CONTENT_TOP`, String(Math.round(metrics.content.top)))
    record(`${prefix}_CONTENT_LEFT`, String(Math.round(metrics.content.left)))
    record(`${prefix}_CONTENT_WIDTH`, String(Math.round(metrics.content.width)))
  }

  const createBtn = page.getByRole('button', { name: /Создать RFx|Create RFx/i })
  record(`${prefix}_CREATE_BUTTON_COUNT`, String(await createBtn.count()))
  record(`${prefix}_PAGE_STACK_EXISTS`, metrics.pageStack ? 'YES' : 'NO')

  return metrics
}

async function main() {
  record('DIAGNOSTIC_SCRIPT_VALID', 'YES')

  const loginCss = await auditPageCss('/login')
  record('LOGIN_HTTP_STATUS', String(loginCss.status))
  let css404 = 0
  let css5xx = 0
  let appShellCss = false
  let appShellMainCss = false
  let headerCss = false
  let sidebarCss = false

  for (const item of loginCss.assets) {
    record('CSS_URL', item.url)
    record('CSS_HTTP_STATUS', String(item.status))
    record('CSS_CONTENT_TYPE', item.contentType)
    record('CSS_CACHE_CONTROL', item.cacheControl)
    record('CSS_SIZE', String(item.size))
    record('CSS_HASH', item.hash)
    if (item.status === 404) css404++
    if (item.status >= 500) css5xx++
    appShellCss ||= item.hasAppShell
    appShellMainCss ||= item.hasAppShellMain
    headerCss ||= item.hasHeader
    sidebarCss ||= item.hasSidebar
  }

  const browser = await chromium.launch({ channel: 'chrome', headless: true })
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } })
  await context.addInitScript((session) => {
    localStorage.setItem('freight_admin_session', JSON.stringify(session))
    localStorage.setItem('freight_admin_tenant', JSON.stringify({ tenantId: session.user.tenant_id }))
  }, fakeSession)

  const page = await context.newPage()
  let firstConsole = ''
  let firstPage = ''
  const failedJs = []
  const failedCss = []

  page.on('console', (msg) => {
    if (msg.type() === 'error' && !firstConsole) firstConsole = msg.text()
  })
  page.on('pageerror', (err) => {
    if (!firstPage) firstPage = String(err.message || err)
  })
  page.on('response', (resp) => {
    const url = resp.url()
    if (resp.status() >= 400 && url.includes('/_nuxt/')) {
      if (url.endsWith('.css')) failedCss.push(`${resp.status()} ${url}`)
      else failedJs.push(`${resp.status()} ${url}`)
    }
  })

  const dash = await inspectRoute(page, '/dashboard', 'dashboard-appshell.png')
  const dashCss = await auditPageCss('/dashboard')
  for (const item of dashCss.assets) {
    appShellCss ||= item.hasAppShell
    appShellMainCss ||= item.hasAppShellMain
    headerCss ||= item.hasHeader
    sidebarCss ||= item.hasSidebar
    if (item.status === 404) css404++
    if (item.status >= 500) css5xx++
  }

  const rfx = await inspectRoute(page, '/rfx', 'rfx-appshell.png')
  const rfxCss = await auditPageCss('/rfx')
  for (const item of rfxCss.assets) {
    appShellCss ||= item.hasAppShell
    appShellMainCss ||= item.hasAppShellMain
    headerCss ||= item.hasHeader
    sidebarCss ||= item.hasSidebar
    if (item.status === 404) css404++
    if (item.status >= 500) css5xx++
  }

  record('APP_SHELL_CSS_PRESENT', appShellCss ? 'YES' : 'NO')
  record('APP_SHELL_MAIN_CSS_PRESENT', appShellMainCss ? 'YES' : 'NO')
  record('HEADER_CSS_PRESENT', headerCss ? 'YES' : 'NO')
  record('SIDEBAR_CSS_PRESENT', sidebarCss ? 'YES' : 'NO')
  record('CSS_404_COUNT', String(css404))
  record('CSS_5XX_COUNT', String(css5xx))
  record('JS_404_COUNT', String(failedJs.length))
  record('FIRST_CONSOLE_ERROR', firstConsole || 'NONE')
  record('FIRST_PAGE_ERROR', firstPage || 'NONE')
  record('FAILED_JS_ASSETS', failedJs.length ? failedJs.join(' | ') : 'NONE')
  record('FAILED_CSS_ASSETS', failedCss.length ? failedCss.join(' | ') : 'NONE')
  record('SCREENSHOT_DIR', OUT_DIR)

  const pass = {
    dashboard:
      dash.appShell?.display === 'flex' &&
      dash.main?.width > 200 &&
      dash.header?.width > 100 &&
      dash.sidebar?.left < 400,
    rfx:
      rfx.appShell?.display === 'flex' &&
      rfx.main?.width > 200 &&
      rfx.header?.width > 100 &&
      (await page.getByRole('button', { name: /Создать RFx|Create RFx/i }).count()) > 0,
  }

  record('POST_FIX_DASHBOARD_RENDER', pass.dashboard ? 'PASS' : 'FAIL')
  record('POST_FIX_RFX_RENDER', pass.rfx ? 'PASS' : 'FAIL')
  record('POST_FIX_HEADER_VISIBLE', dash.header?.height > 0 && rfx.header?.height > 0 ? 'PASS' : 'FAIL')
  record('POST_FIX_MAIN_VISIBLE', dash.main?.height > 0 && rfx.main?.height > 0 ? 'PASS' : 'FAIL')
  record('POST_FIX_CREATE_BUTTON_VISIBLE', pass.rfx ? 'PASS' : 'FAIL')

  await browser.close()
  process.exit(pass.dashboard && pass.rfx ? 0 : 1)
}

main().catch((err) => {
  record('DIAGNOSTIC', 'FAIL')
  console.error(String(err.message || err))
  process.exit(1)
})
