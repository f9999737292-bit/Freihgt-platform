/** @typedef {import('@playwright/test').Page} Page */

export const E2E_CONFIG = {
  tenantId: process.env.E2E_TENANT_ID || '74519f22-ff9b-4a8b-8fff-a958c689682f',
  adminEmail: process.env.E2E_ADMIN_EMAIL || 'admin@7rights.local',
  adminPassword: process.env.E2E_ADMIN_PASSWORD || 'Admin123456!',
  frontendUrl: process.env.E2E_FRONTEND_URL || 'http://127.0.0.1:3000',
  backendUrl: process.env.E2E_BACKEND_URL || 'http://localhost:8080',
  foreignTenantId: process.env.E2E_FOREIGN_TENANT_ID || '91babc18-1fe0-4df3-8d2c-b350e6052b33',
  foreignShipmentId:
    process.env.E2E_FOREIGN_SHIPMENT_ID || '00000000-0000-4000-8000-000000000099',
}

/**
 * Authenticate via real backend login API inside browser context, then navigate.
 * Uses REAL_LOCAL_BACKEND auth — not mock.
 * @param {Page} page
 */
export async function loginViaApi(page) {
  await page.goto('/login', { waitUntil: 'domcontentloaded' })
  const result = await page.evaluate(
    async ({ tenantId, email, password, apiBase }) => {
      const response = await fetch(`${apiBase}/api/v1/auth/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
        body: JSON.stringify({ tenant_id: tenantId, email, password }),
      })
      const payload = await response.json()
      if (!response.ok) {
        return { ok: false, status: response.status, message: payload?.error?.message || 'login failed' }
      }
      localStorage.setItem(
        'freight_admin_session',
        JSON.stringify({ token: payload.access_token, user: payload.user }),
      )
      localStorage.setItem('freight_admin_tenant_id', tenantId)
      return { ok: true, status: response.status }
    },
    {
      tenantId: E2E_CONFIG.tenantId,
      email: E2E_CONFIG.adminEmail,
      password: E2E_CONFIG.adminPassword,
      apiBase: E2E_CONFIG.backendUrl,
    },
  )
  if (!result.ok) {
    throw new Error(`Browser login API failed: HTTP ${result.status} ${result.message}`)
  }
  await page.reload({ waitUntil: 'domcontentloaded' })
  await page.getByRole('link', { name: /Control Tower/i }).click({ timeout: 30_000 })
  await page.waitForURL(/\/control-tower/, { timeout: 30_000 })
}

/**
 * @param {Page} page
 * @param {{ tenantId?: string, email?: string, password?: string }} [overrides]
 */
export async function loginViaUi(page, overrides = {}) {
  const tenantId = overrides.tenantId ?? E2E_CONFIG.tenantId
  const email = overrides.email ?? E2E_CONFIG.adminEmail
  const password = overrides.password ?? E2E_CONFIG.adminPassword

  await page.goto('/login', { waitUntil: 'networkidle' })
  const inputs = page.locator('.ui-input__control')
  await inputs.nth(0).click()
  await inputs.nth(0).fill(tenantId)
  await inputs.nth(1).click()
  await inputs.nth(1).fill(email)
  await inputs.nth(2).click()
  await inputs.nth(2).fill(password)
  await page.locator('form.login-form').evaluate((form) => form.requestSubmit())
  await page.waitForURL(/\/(dashboard|control-tower)/, { timeout: 30_000 })
}

/**
 * @param {Page} page
 * @param {{ roles?: string[], email?: string }} session
 */
export async function seedMockSession(page, session) {
  const payload = {
    token: `e2e-mock-token-${Date.now()}`,
    user: {
      id: 'e2e-user-0001',
      tenant_id: E2E_CONFIG.tenantId,
      email: session.email ?? 'viewer@7rights.local',
      full_name: 'E2E Viewer',
      preferred_locale: 'en-US',
      status: 'ACTIVE',
      roles: session.roles ?? ['CONSIGNEE_VIEWER'],
    },
  }

  await page.addInitScript((data) => {
    localStorage.setItem('freight_admin_session', JSON.stringify({ token: data.token, user: data.user }))
    localStorage.setItem('freight_admin_tenant_id', data.user.tenant_id)
  }, payload)
}

/**
 * @param {Page} page
 */
export async function gotoControlTower(page) {
  if (!page.url().includes('/control-tower')) {
    await page.getByRole('link', { name: /Control Tower/i }).click()
    await page.waitForURL(/\/control-tower/, { timeout: 30_000 })
  }
  await page.waitForSelector('.control-tower-v01, .ui-empty', { timeout: 30_000 })
}

/**
 * @param {Page} page
 */
export async function waitForControlTowerLoaded(page) {
  await page.waitForFunction(() => {
    const root = document.querySelector('.control-tower-v01')
    if (!root) return false
    const text = root.textContent || ''
    if (/Loading…|Загрузка/i.test(text) && !document.querySelector('.metric-card')) {
      return false
    }
    return true
  }, { timeout: 30_000 })
}

export function isKnownNonBlockingConsoleError(entry) {
  return /isApiUnavailableError is not a function/.test(entry.text)
}

/**
 * @param {Page} page
 */
export function attachConsoleCollector(page) {
  /** @type {{ type: string, text: string, classification: string }[]} */
  const entries = []

  page.on('console', (msg) => {
    const text = msg.text()
    if (/favicon|devtools|hmr|vite|nuxt/i.test(text)) return
    const type = msg.type()
    let classification = 'NOISE'
    if (/Hydration completed but contains mismatches/i.test(text)) classification = 'EXPECTED'
    else if (type === 'error') classification = 'ERROR'
    else if (type === 'warning') classification = 'WARNING'
    entries.push({ type, text, classification })
  })

  page.on('pageerror', (err) => {
    entries.push({
      type: 'pageerror',
      text: err.message,
      classification: 'ERROR',
    })
  })

  return entries
}

/**
 * @param {string} bodyText
 */
export function hasBrokenUiValues(bodyText) {
  return (
    /\bundefined\b/i.test(bodyText)
    || /\bNaN\b/.test(bodyText)
    || /\[object Object\]/i.test(bodyText)
  )
}
