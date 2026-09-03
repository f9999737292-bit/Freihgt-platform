/**
 * RFx blank content diagnostic — no auth, no secrets.
 */
import { chromium } from 'playwright'

const BASE = process.env.PILOT_UI_BASE_URL || 'http://localhost:13000'

function record(key, value) {
  console.log(`${key}=${value}`)
}

async function fetchText(url) {
  const res = await fetch(url, { redirect: 'manual' })
  return { status: res.status, headers: Object.fromEntries(res.headers.entries()), body: await res.text() }
}

async function main() {
  const login = await fetchText(`${BASE}/login`)
  const rfx = await fetchText(`${BASE}/rfx`)
  const dash = await fetchText(`${BASE}/dashboard`)

  const loginBuild = login.body.match(/buildId:"([^"]+)"/)?.[1] || 'NONE'
  const rfxBuild = rfx.body.match(/buildId:"([^"]+)"/)?.[1] || 'NONE'

  record('LOGIN_HTTP_STATUS', String(login.status))
  record('RFX_HTTP_STATUS', String(rfx.status))
  record('DASHBOARD_HTTP_STATUS', String(dash.status))
  record('HTML_BUILD_ID_LOGIN', loginBuild)
  record('HTML_BUILD_ID_RFX', rfxBuild)
  record('RFX_SERVER_HTML_HAS_SIDEBAR', /app-sidebar|LayoutAppSidebar|sidebar/i.test(rfx.body) ? 'YES' : 'NO')
  record('RFX_SERVER_HTML_HAS_HEADER', /app-header|LayoutAppHeader|ui-page-header/i.test(rfx.body) ? 'YES' : 'NO')
  record('RFX_SERVER_HTML_HAS_PAGE_CONTENT', /page-stack|rfx\.title|Создать RFx|RFx/i.test(rfx.body) ? 'YES' : 'NO')
  record('RFX_SERVER_HTML_HAS_ERROR', /nuxt-error|__NUXT_ERROR__|error-page|Invalid/i.test(rfx.body) ? 'YES' : 'NO')
  record('RFX_SERVER_ERROR_TEXT', rfx.body.match(/Invalid|error/i)?.[0] || 'NONE')
  record('CACHE_CONTROL_HTML', login.headers['cache-control'] || 'NONE')

  const assetUrls = [...login.body.matchAll(/\/_nuxt\/[^"'\s>]+\.(js|css)/g)].map((m) => `${BASE}${m[0]}`)
  let asset404 = 0
  let asset5xx = 0
  const buildIds = new Set()

  for (const url of [...new Set(assetUrls)].slice(0, 40)) {
    const res = await fetch(url)
    if (res.status === 404) asset404++
    if (res.status >= 500) asset5xx++
    const etag = res.headers.get('etag') || 'NONE'
    record(`ASSET_${res.status}`, url.replace(BASE, ''))
    if (url.endsWith('.js')) record('CACHE_CONTROL_JS', res.headers.get('cache-control') || 'NONE')
    if (url.endsWith('.css')) record('CACHE_CONTROL_CSS', res.headers.get('cache-control') || 'NONE')
  }

  record('ASSET_404_COUNT', String(asset404))
  record('ASSET_5XX_COUNT', String(asset5xx))
  record('MIXED_BUILD_ASSETS', loginBuild !== rfxBuild && rfxBuild !== 'NONE' ? 'YES' : 'NO')

  const browser = await chromium.launch({ channel: 'chrome', headless: true })
  const context = await browser.newContext()
  const page = await context.newPage()

  let firstConsole = ''
  let firstPage = ''
  let failedResource = ''
  let failedStatus = ''

  page.on('console', (msg) => {
    if (msg.type() === 'error' && !firstConsole) firstConsole = msg.text()
  })
  page.on('pageerror', (err) => {
    if (!firstPage) firstPage = String(err.message || err)
  })
  page.on('requestfailed', (req) => {
    if (!failedResource) {
      failedResource = req.url()
      failedStatus = req.failure()?.errorText || 'FAILED'
    }
  })

  await page.goto(`${BASE}/login`, { waitUntil: 'networkidle', timeout: 60000 })
  record('LOGIN_PAGE_RENDER', (await page.locator('form.login-form').count()) > 0 ? 'PASS' : 'FAIL')

  await page.goto(`${BASE}/rfx`, { waitUntil: 'networkidle', timeout: 60000 })
  const bodyText = await page.locator('body').innerText()
  record('RFX_UNAUTH_FINAL_URL', page.url())
  record('APP_SHELL_DOM_PRESENT', (await page.locator('.app-shell').count()) > 0 ? 'YES' : 'NO')
  record('SIDEBAR_DOM_PRESENT', (await page.locator('.app-sidebar, nav.sidebar, [class*=sidebar]').count()) > 0 ? 'YES' : 'NO')
  record('MAIN_DOM_PRESENT', (await page.locator('.app-shell__main, main.app-shell__content').count()) > 0 ? 'YES' : 'NO')
  record('HEADER_DOM_PRESENT', (await page.locator('.app-header, header').count()) > 0 ? 'YES' : 'NO')
  record('CONTENT_DOM_PRESENT', (await page.locator('.page-stack, .ui-page-header').count()) > 0 ? 'YES' : 'NO')
  record('BODY_HAS_INVALID', /Invalid/i.test(bodyText) ? 'YES' : 'NO')
  record('FIRST_BROWSER_ERROR', firstConsole || 'NONE')
  record('FIRST_PAGE_ERROR', firstPage || 'NONE')
  record('FAILED_RESOURCE', failedResource || 'NONE')
  record('FAILED_RESOURCE_STATUS', failedStatus || 'NONE')

  await browser.close()
}

main().catch((err) => {
  record('DIAGNOSTIC', 'FAIL')
  console.error(String(err.message || err))
  process.exit(1)
})
