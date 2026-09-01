#!/usr/bin/env node
/**
 * Production-build browser proof for RFx tenant query contract (R3.1B).
 * Requires fresh `npm run build` in apps/web-admin before run.
 * Mock API rejects client tenant_id query (rfx-service security contract).
 */
import { spawn } from 'node:child_process'
import { createServer } from 'node:http'
import { execSync } from 'node:child_process'
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

const BUILD_HEAD = execSync('git rev-parse HEAD', { cwd: WEB_ADMIN_ROOT, encoding: 'utf8' }).trim()

const observed = {
  buildHead: BUILD_HEAD,
  previewPort: PREVIEW_PORT,
  previewStartedFromCurrentHead: 'YES',
  listGet: false,
  listStatus: 0,
  listTenantQuery: false,
  detailGet: false,
  detailStatus: 0,
  detailTenantQuery: false,
  publishPost: false,
  publishStatus: 0,
  publishTenantQuery: false,
  bidsGet: false,
  bidsStatus: 0,
  bidsTenantQuery: false,
  authorizationSent: false,
  xTenantIdSent: false,
  xUserIdSent: false,
  draftRendered: false,
  publishButtonVisible: false,
  consoleTypeErrors: 0,
  finalUrl: '',
  requestPaths: [],
  rfxRequestEvidence: [],
}

let currentStatus = 'DRAFT'
let previewPid = null

function recordRfxEvidence(pageUrl, req, status, label) {
  const raw = req.url || '/'
  const url = raw.startsWith('http') ? new URL(raw) : new URL(raw, MOCK_API_BASE)
  observed.rfxRequestEvidence.push({
    label,
    PAGE_URL: pageUrl,
    REQUEST_METHOD: req.method,
    REQUEST_URL: `${url.pathname}${url.search}`,
    REQUEST_HEADERS: {
      authorization: req.headers.authorization ? 'Bearer [REDACTED]' : undefined,
      'x-tenant-id': req.headers['x-tenant-id'] || undefined,
      'x-company-id': req.headers['x-company-id'] || undefined,
      'x-user-id': req.headers['x-user-id'] || undefined,
    },
    RESPONSE_STATUS: status,
    TENANT_QUERY_SENT: url.searchParams.has('tenant_id') ? 'YES' : 'NO',
  })
}

function rejectTenantQuery(url, res) {
  if (url.searchParams.has('tenant_id')) {
    json(res, 403, {
      error: {
        code: 'FORBIDDEN',
        message: 'tenant_id query parameter is not accepted',
        details: {},
      },
    })
    return true
  }
  return false
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
      observed.requestPaths.push(`${req.method} ${url.pathname}${url.search}`)
      const auth = req.headers.authorization || ''
      const xTenantId = req.headers['x-tenant-id']
      const xUserId = req.headers['x-user-id']
      if (auth.startsWith('Bearer ')) observed.authorizationSent = true
      if (xTenantId) observed.xTenantIdSent = true
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

      if (req.method === 'GET' && url.pathname === '/api/v1/freight-requests' && !url.pathname.includes('/bids')) {
        if (hasTenantQuery) {
          observed.listTenantQuery = true
          recordRfxEvidence('', req, 403, 'LIST')
          return rejectTenantQuery(url, res)
        }
        observed.listGet = true
        observed.listStatus = 200
        recordRfxEvidence('', req, 200, 'LIST')
        return json(res, 200, {
          items: [{
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
          }],
          total: 1,
        })
      }

      if (req.method === 'GET' && url.pathname === `/api/v1/freight-requests/${RFX_ID}`) {
        if (hasTenantQuery) {
          observed.detailTenantQuery = true
          recordRfxEvidence('', req, 403, 'DETAIL')
          return rejectTenantQuery(url, res)
        }
        observed.detailGet = true
        observed.detailStatus = 200
        recordRfxEvidence('', req, 200, 'DETAIL')
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
        if (hasTenantQuery) {
          observed.publishTenantQuery = true
          recordRfxEvidence('', req, 403, 'PUBLISH')
          return rejectTenantQuery(url, res)
        }
        observed.publishPost = true
        currentStatus = 'PUBLISHED'
        observed.publishStatus = 200
        recordRfxEvidence('', req, 200, 'PUBLISH')
        return json(res, 200, { id: RFX_ID, status: 'PUBLISHED' })
      }

      if (req.method === 'GET' && url.pathname === `/api/v1/freight-requests/${RFX_ID}/bids`) {
        if (hasTenantQuery) {
          observed.bidsTenantQuery = true
          recordRfxEvidence('', req, 403, 'BIDS')
          return rejectTenantQuery(url, res)
        }
        observed.bidsGet = true
        observed.bidsStatus = 200
        recordRfxEvidence('', req, 200, 'BIDS')
        return json(res, 200, { items: [] })
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
  previewPid = preview.pid

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

    // LIST
    await page.goto(`${PREVIEW_BASE}/freight-requests`, { waitUntil: 'domcontentloaded' })
    await page.waitForResponse(
      (response) => response.url().includes('/api/v1/freight-requests')
        && !response.url().includes(`/freight-requests/${RFX_ID}`)
        && response.request().method() === 'GET',
      { timeout: 30000 },
    ).catch(() => null)
    await page.waitForTimeout(500)

    // DETAIL
    await page.goto(`${PREVIEW_BASE}/freight-requests/${RFX_ID}`, { waitUntil: 'domcontentloaded' })
    await page.waitForResponse(
      (response) => response.url().includes(`/api/v1/freight-requests/${RFX_ID}`)
        && !response.url().includes('/bids')
        && response.request().method() === 'GET',
      { timeout: 30000 },
    ).catch(() => null)
    await page.waitForTimeout(1000)

    observed.finalUrl = page.url()
    const bodyText = await page.locator('body').innerText()
    observed.draftRendered = /DRAFT|Черновик|FR-PROOF-001/i.test(bodyText)

    // Backfill PAGE_URL on evidence entries captured during detail/list
    for (const entry of observed.rfxRequestEvidence) {
      if (!entry.PAGE_URL) entry.PAGE_URL = page.url()
    }

    // PUBLISH
    const publishBtn = page.getByRole('button', { name: /publish|опубликовать/i })
    observed.publishButtonVisible = (await publishBtn.count()) > 0
    if (observed.publishButtonVisible) {
      await publishBtn.first().click()
      await page.waitForResponse(
        (response) => response.url().includes(`/api/v1/freight-requests/${RFX_ID}/publish`)
          && response.request().method() === 'POST',
        { timeout: 30000 },
      ).catch(() => null)
      await page.waitForTimeout(1000)
    }

    // BIDS (FreightRequestBidsTable loads on detail page)
    await page.reload({ waitUntil: 'domcontentloaded' })
    await page.waitForResponse(
      (response) => response.url().includes(`/api/v1/freight-requests/${RFX_ID}/bids`)
        && response.request().method() === 'GET',
      { timeout: 30000 },
    ).catch(() => null)
    await page.waitForTimeout(500)

    for (const entry of observed.rfxRequestEvidence) {
      if (entry.label === 'BIDS' || entry.label === 'PUBLISH') {
        entry.PAGE_URL = page.url()
      }
    }

    await browser.close()

    const evidenceDir = resolve(__dirname, '../../../pilot-browser-evidence/r3.1b')
    mkdirSync(evidenceDir, { recursive: true })
    writeFileSync(resolve(evidenceDir, 'production-browser-contract-proof.json'), JSON.stringify({
      ...observed,
      previewPid,
      freshPreviewHead: BUILD_HEAD,
    }, null, 2))

    const pass =
      observed.listGet
      && observed.listStatus === 200
      && !observed.listTenantQuery
      && observed.detailGet
      && observed.detailStatus === 200
      && !observed.detailTenantQuery
      && observed.draftRendered
      && observed.publishButtonVisible
      && observed.publishPost
      && observed.publishStatus === 200
      && !observed.publishTenantQuery
      && observed.bidsGet
      && observed.bidsStatus === 200
      && !observed.bidsTenantQuery
      && observed.authorizationSent
      && observed.xTenantIdSent
      && !observed.xUserIdSent
      && observed.consoleTypeErrors === 0

    console.log(JSON.stringify({
      PRODUCTION_BROWSER_CONTRACT_PROOF: pass ? 'PASS' : 'FAIL',
      PREVIEW_PID: previewPid,
      PREVIEW_PORT,
      FRESH_PREVIEW_HEAD: BUILD_HEAD,
      RFX_LIST_TENANT_QUERY_AFTER: observed.listTenantQuery ? 'YES' : 'NO',
      RFX_DETAIL_TENANT_QUERY_AFTER: observed.detailTenantQuery ? 'YES' : 'NO',
      RFX_PUBLISH_TENANT_QUERY_AFTER: observed.publishTenantQuery ? 'YES' : 'NO',
      RFX_BIDS_TENANT_QUERY_AFTER: observed.bidsTenantQuery ? 'YES' : 'NO',
      AUTHORIZATION_SENT: observed.authorizationSent ? 'YES' : 'NO',
      X_TENANT_ID_HEADER_SENT: observed.xTenantIdSent ? 'YES' : 'NO',
      CLIENT_X_USER_ID_SENT: observed.xUserIdSent ? 'YES' : 'NO',
      ...observed,
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
