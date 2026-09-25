#!/usr/bin/env node
/**
 * Production-build browser proof that /rfx does not send tenant_id on
 * GET /api/v1/rfx-events. Freight-request coverage stays in
 * rfx-tenant-query-contract-production.mjs.
 * Requires a fresh `npm run build` in apps/web-admin before run.
 */
import { spawn, execSync } from 'node:child_process'
import { createServer } from 'node:http'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { chromium } from 'playwright'

const __dirname = dirname(fileURLToPath(import.meta.url))
const WEB_ADMIN_ROOT = resolve(__dirname, '../..')
const TENANT_ID = '873b3fbc-3cb4-413f-81cd-6fa2c94e785e'
const PREVIEW_PORT = 3310
const MOCK_API_PORT = 3311
const PREVIEW_BASE = `http://127.0.0.1:${PREVIEW_PORT}`
const MOCK_API_BASE = `http://127.0.0.1:${MOCK_API_PORT}`
const BUILD_HEAD = execSync('git rev-parse HEAD', { cwd: WEB_ADMIN_ROOT, encoding: 'utf8' }).trim()

const observed = {
  buildHead: BUILD_HEAD,
  listGet: false,
  listStatus: 0,
  listTenantQuery: false,
  dashboardListGet: false,
  dashboardListStatus: 0,
  dashboardListTenantQuery: false,
  authorizationSent: false,
  xUserIdSent: false,
  finalUrl: '',
}

function json(res, status, body) {
  res.writeHead(status, {
    'Content-Type': 'application/json',
    'Access-Control-Allow-Origin': '*',
    'Access-Control-Allow-Headers': 'authorization,content-type,x-tenant-id,x-company-id,x-request-id,x-locale',
    'Access-Control-Allow-Methods': 'GET,POST,PUT,PATCH,DELETE,OPTIONS',
  })
  res.end(JSON.stringify(body))
}

function rejectTenantQuery(res) {
  json(res, 403, {
    error: {
      code: 'FORBIDDEN',
      message: 'tenant_id query parameter is not accepted',
      details: {},
    },
  })
}

function startMockApi() {
  return new Promise((resolveServer) => {
    const server = createServer((req, res) => {
      if (req.method === 'OPTIONS') {
        res.writeHead(204, {
          'Access-Control-Allow-Origin': '*',
          'Access-Control-Allow-Headers': 'authorization,content-type,x-tenant-id,x-company-id,x-request-id,x-locale',
          'Access-Control-Allow-Methods': 'GET,POST,PUT,PATCH,DELETE,OPTIONS',
        })
        res.end()
        return
      }

      const url = new URL(req.url || '/', MOCK_API_BASE)
      const hasTenantQuery = url.searchParams.has('tenant_id')
      const auth = req.headers.authorization || ''
      if (auth.startsWith('Bearer ')) observed.authorizationSent = true
      if (req.headers['x-user-id']) observed.xUserIdSent = true

      if (req.method === 'POST' && url.pathname === '/api/v1/auth/login') {
        return json(res, 200, {
          access_token: 'proof-token',
          user: {
            id: '29776166-c811-49bd-bb7f-d8539f05c8bb',
            tenant_id: TENANT_ID,
            email: 'pilot-r3-shipper@bintrans.local',
            full_name: 'BINTRANS R3.1 Disposable Shipper',
            preferred_locale: 'ru-RU',
            status: 'ACTIVE',
            roles: ['SHIPPER_ADMIN'],
          },
        })
      }

      if (req.method === 'GET' && url.pathname === '/health') {
        return json(res, 200, { status: 'ok' })
      }

      if (req.method === 'GET' && url.pathname === '/api/v1/rfx-events') {
        const fromListPage = url.searchParams.get('limit') === '20'
        if (hasTenantQuery) {
          if (fromListPage) observed.listTenantQuery = true
          else observed.dashboardListTenantQuery = true
          return rejectTenantQuery(res)
        }
        if (fromListPage) {
          observed.listGet = true
          observed.listStatus = 200
        } else {
          observed.dashboardListGet = true
          observed.dashboardListStatus = 200
        }
        return json(res, 200, { items: [], total: 0 })
      }

      if (req.method === 'GET' && url.pathname.startsWith('/api/v1/')) {
        return json(res, 200, { items: [], total: 0 })
      }

      json(res, 404, { error: { code: 'NOT_FOUND', message: 'not found', details: {} } })
    })
    server.listen(MOCK_API_PORT, '127.0.0.1', () => resolveServer(server))
  })
}

async function waitForUrl(url, attempts = 60) {
  for (let i = 0; i < attempts; i += 1) {
    try {
      const res = await fetch(url)
      if (res.ok) return
    } catch {
      // retry
    }
    await new Promise((r) => setTimeout(r, 500))
  }
  throw new Error(`Timeout waiting for ${url}`)
}

async function launchBrowser() {
  try {
    return await chromium.launch({ headless: true })
  } catch {
    return await chromium.launch({
      headless: true,
      executablePath: process.env.CHROME_PATH || 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe',
    })
  }
}

async function main() {
  const mockServer = await startMockApi()
  const preview = spawn('npm', ['run', 'preview', '--', '--port', String(PREVIEW_PORT)], {
    cwd: WEB_ADMIN_ROOT,
    shell: true,
    env: {
      ...process.env,
      NUXT_PUBLIC_API_BASE_URL: MOCK_API_BASE,
    },
    stdio: 'inherit',
  })

  try {
    await waitForUrl(`${PREVIEW_BASE}/login`)
    const browser = await launchBrowser()
    const page = await browser.newPage()
    await page.goto(`${PREVIEW_BASE}/login`)
    const inputs = page.locator('form.login-form input')
    await inputs.nth(0).fill(TENANT_ID)
    await inputs.nth(1).fill('pilot-r3-shipper@bintrans.local')
    await inputs.nth(2).fill('password')
    await page.getByRole('button', { name: /login|войти/i }).click()
    await page.waitForURL(/\/dashboard/, { timeout: 30000 })
    await page.waitForResponse(
      (response) => response.url().includes('/api/v1/rfx-events') && response.request().method() === 'GET',
      { timeout: 30000 },
    )
    await page.goto(`${PREVIEW_BASE}/rfx`, { waitUntil: 'domcontentloaded' })
    await page.waitForResponse(
      (response) => response.url().includes('/api/v1/rfx-events')
        && response.request().method() === 'GET'
        && new URL(response.url()).searchParams.get('limit') === '20',
      { timeout: 30000 },
    )
    observed.finalUrl = page.url()
    await browser.close()

    const pass =
      observed.dashboardListGet
      && observed.dashboardListStatus === 200
      && !observed.dashboardListTenantQuery
      && observed.listGet
      && observed.listStatus === 200
      && !observed.listTenantQuery
      && observed.authorizationSent
      && !observed.xUserIdSent
      && observed.finalUrl.includes('/rfx')

    console.log(JSON.stringify({
      RFX_EVENTS_PRODUCTION_BROWSER_PROOF: pass ? 'PASS' : 'FAIL',
      BUILD_HEAD,
      LIST_STATUS: observed.listStatus,
      LIST_TENANT_QUERY_SENT: observed.listTenantQuery ? 'YES' : 'NO',
      DASHBOARD_LIST_STATUS: observed.dashboardListStatus,
      DASHBOARD_LIST_TENANT_QUERY_SENT: observed.dashboardListTenantQuery ? 'YES' : 'NO',
      AUTHORIZATION_SENT: observed.authorizationSent ? 'YES' : 'NO',
      X_USER_ID_SENT: observed.xUserIdSent ? 'YES' : 'NO',
      FINAL_URL: observed.finalUrl,
    }, null, 2))
    if (!pass) process.exit(1)
  } finally {
    preview.kill('SIGTERM')
    mockServer.close()
  }
}

main().catch((error) => {
  console.error(error)
  process.exit(1)
})
