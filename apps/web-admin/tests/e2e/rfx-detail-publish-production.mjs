#!/usr/bin/env node
/**
 * Production-build browser proof for RFx detail + publish (R3.1A).
 * Runs against `nuxt preview` with a local mock API matching gateway contracts.
 */
import { spawn } from 'node:child_process'
import { createServer } from 'node:http'
import { mkdirSync, writeFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { chromium } from 'playwright'

const __dirname = dirname(fileURLToPath(import.meta.url))
const WEB_ADMIN_ROOT = resolve(__dirname, '..')
const RFX_ID = '6aa74939-c406-480b-a38f-d5e349d57899'
const TENANT_ID = '285f9447-faf7-423e-96dd-e4c5e2b3fc6c'
const COMPANY_ID = '83cb2447-75e9-41f2-8e0d-93c70f8506be'
const PREVIEW_PORT = 3300
const MOCK_API_PORT = 3301
const PREVIEW_BASE = `http://127.0.0.1:${PREVIEW_PORT}`
const MOCK_API_BASE = `http://127.0.0.1:${MOCK_API_PORT}`

const observed = {
  detailGet: false,
  detailStatus: 0,
  publishPost: false,
  publishStatus: 0,
  authorizationSent: false,
  xUserIdSent: false,
  draftRendered: false,
  publishButtonVisible: false,
  consoleTypeErrors: 0,
  finalUrl: '',
  requestPaths: [],
}

let currentStatus = 'DRAFT'

function json(res, status, body) {
  res.writeHead(status, {
    'Content-Type': 'application/json',
    'Access-Control-Allow-Origin': '*',
    'Access-Control-Allow-Headers': 'authorization,content-type,x-tenant-id,x-company-id,x-request-id,x-locale',
    'Access-Control-Allow-Methods': 'GET,POST,PUT,PATCH,DELETE,OPTIONS',
  })
  res.end(JSON.stringify(body))
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
      observed.requestPaths.push(`${req.method} ${url.pathname}`)
      const auth = req.headers.authorization || ''
      const xUserId = req.headers['x-user-id']
      if (auth.startsWith('Bearer ')) observed.authorizationSent = true
      if (xUserId) observed.xUserIdSent = true

      if (req.method === 'POST' && url.pathname === '/api/v1/auth/login') {
        return json(res, 200, {
          access_token: 'proof-token',
          user: {
            id: '8541a3a3-bde7-4fed-9501-37b9953bf904',
            tenant_id: TENANT_ID,
            email: 'buyer@test.local',
            full_name: 'Pilot Buyer',
            preferred_locale: 'ru-RU',
            status: 'ACTIVE',
            roles: ['SHIPPER_ADMIN'],
          },
        })
      }

      if (req.method === 'GET' && url.pathname === '/health') {
        return json(res, 200, { status: 'ok' })
      }

      if (req.method === 'GET' && url.pathname === `/api/v1/freight-requests/${RFX_ID}`) {
        observed.detailGet = true
        observed.detailStatus = 200
        return json(res, 200, {
          id: RFX_ID,
          tenant_id: TENANT_ID,
          freight_request_number: 'FR-PROOF-001',
          request_type: 'MINI_TENDER',
          status: currentStatus,
          shipper_company_id: COMPANY_ID,
          currency_code: 'RUB',
          response_deadline: '2026-12-31T00:00:00Z',
          created_at: '2026-08-29T00:00:00Z',
          updated_at: '2026-08-29T00:00:00Z',
        })
      }

      if (req.method === 'POST' && url.pathname === `/api/v1/freight-requests/${RFX_ID}/publish`) {
        observed.publishPost = true
        currentStatus = 'PUBLISHED'
        observed.publishStatus = 200
        return json(res, 200, { id: RFX_ID, status: 'PUBLISHED' })
      }

      if (req.method === 'GET' && url.pathname === '/api/v1/companies') {
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

    const browser = await chromium.launch({ headless: true })
    const context = await browser.newContext()
    const page = await context.newPage()

    page.on('console', (msg) => {
      if (msg.type() === 'error' && /TypeError: .* is not a function/.test(msg.text())) {
        observed.consoleTypeErrors += 1
      }
    })

    await page.goto(`${PREVIEW_BASE}/login`)
    const inputs = page.locator('form.login-form input')
    await inputs.nth(0).fill(TENANT_ID)
    await inputs.nth(1).fill('buyer@test.local')
    await inputs.nth(2).fill('password')
    await page.getByRole('button', { name: /login|войти/i }).click()
    await page.waitForURL(/\/dashboard|\/freight-requests|\/shipments|\/billing-registers/, { timeout: 30000 })

    await page.goto(`${PREVIEW_BASE}/freight-requests/${RFX_ID}`, { waitUntil: 'domcontentloaded' })
    if (!page.url().includes('/freight-requests/')) {
      await page.locator('a[href="/freight-requests"]').first().click()
      await page.waitForURL(new RegExp(`/freight-requests/${RFX_ID}`), { timeout: 30000 })
    }
    await page.waitForResponse(
      (response) => response.url().includes(`/api/v1/freight-requests/${RFX_ID}`) && response.request().method() === 'GET',
      { timeout: 30000 },
    ).catch(() => null)
    await page.waitForTimeout(1000)

    observed.finalUrl = page.url()
    const bodyText = await page.locator('body').innerText()
    observed.draftRendered = /DRAFT|Черновик|FR-PROOF-001/i.test(bodyText)

    const publishBtn = page.getByRole('button', { name: /publish|опубликовать/i })
    observed.publishButtonVisible = (await publishBtn.count()) > 0
    if (observed.publishButtonVisible) {
      await publishBtn.first().click()
      await page.waitForTimeout(1000)
    }

    await browser.close()

    const evidenceDir = resolve(__dirname, '../../../../pilot-browser-evidence/r3.1a')
    mkdirSync(evidenceDir, { recursive: true })
    writeFileSync(resolve(evidenceDir, 'production-browser-proof.json'), JSON.stringify(observed, null, 2))

    const pass =
      observed.detailGet
      && observed.detailStatus === 200
      && observed.draftRendered
      && observed.publishButtonVisible
      && observed.publishPost
      && observed.publishStatus === 200
      && observed.authorizationSent
      && !observed.xUserIdSent
      && observed.consoleTypeErrors === 0

    console.log(JSON.stringify({ PRODUCTION_BUILD_BROWSER_PROOF: pass ? 'PASS' : 'FAIL', ...observed }, null, 2))
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
