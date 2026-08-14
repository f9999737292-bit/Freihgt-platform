import { test, expect } from '@playwright/test'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import {
  E2E_CONFIG,
  attachConsoleCollector,
  gotoControlTower,
  hasBrokenUiValues,
  isKnownNonBlockingConsoleError,
  loginViaApi,
  loginViaUi,
  seedMockSession,
  waitForControlTowerLoaded,
} from './helpers.mjs'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const artifactsDir = path.join(__dirname, 'artifacts', 'screenshots')

function ensureArtifactsDir() {
  fs.mkdirSync(artifactsDir, { recursive: true })
}

async function screenshot(page, name) {
  ensureArtifactsDir()
  await page.screenshot({ path: path.join(artifactsDir, `${name}.png`), fullPage: true })
}

test.describe.serial('Control Tower UI E2E v0.1', () => {
  /** @type {ReturnType<typeof attachConsoleCollector>} */
  let consoleEntries = []

  test.beforeEach(async ({ page }) => {
    consoleEntries = attachConsoleCollector(page)
  })

  test('CT-E2E-001 Control Tower page loads', async ({ page }) => {
    await loginViaApi(page)
    await gotoControlTower(page)
    await waitForControlTowerLoaded(page)

    await expect(page.locator('.control-tower-v01')).toBeVisible()
    await expect(page.locator('.ct-toolbar')).toBeVisible()
    await expect(page.locator('.control-tower-v01__kpi-grid')).toBeVisible()
    await expect(page.locator('.filters-row')).toBeVisible()
    await expect(page.locator('.control-tower-v01__main-grid')).toBeVisible()

    const bodyText = await page.locator('.control-tower-v01').innerText()
    expect(hasBrokenUiValues(bodyText)).toBe(false)

    const fatalErrors = consoleEntries.filter(
      (e) => e.classification === 'ERROR' && !isKnownNonBlockingConsoleError(e),
    )
    expect(fatalErrors, JSON.stringify(fatalErrors, null, 2)).toHaveLength(0)

    await screenshot(page, '01-control-tower-loaded')
  })

  test('CT-E2E-002 Backend status', async ({ page }) => {
    await loginViaApi(page)
    await gotoControlTower(page)
    await waitForControlTowerLoaded(page)

    const toolbarText = await page.locator('.ct-toolbar').innerText()
    expect(toolbarText).toMatch(/Backend connection|Подключение к backend/i)

    const health = await page.request.get(`${E2E_CONFIG.backendUrl}/health`)
    expect(health.ok()).toBe(true)

    if (health.ok()) {
      expect(toolbarText).toMatch(/Online|В сети|online/i)
      await expect(page.locator('.api-unavailable')).toHaveCount(0)
    }

    await screenshot(page, '02-backend-status')
  })

  test('CT-E2E-003 KPI cards', async ({ page }) => {
    await loginViaApi(page)
    await gotoControlTower(page)
    await waitForControlTowerLoaded(page)

    const cards = page.locator('.metric-card')
    await expect(cards.first()).toBeVisible()
    expect(await cards.count()).toBeGreaterThanOrEqual(4)

    for (const card of await cards.all()) {
      const text = await card.innerText()
      expect(hasBrokenUiValues(text)).toBe(false)
      expect(text.trim().length).toBeGreaterThan(0)
    }

    await screenshot(page, '02-kpi-and-active-shipments')
  })

  test('CT-E2E-004 Active shipments', async ({ page }) => {
    await loginViaApi(page)
    await gotoControlTower(page)
    await waitForControlTowerLoaded(page)

    const tableCard = page.locator('.control-tower-v01__table-card')
    await expect(tableCard).toBeVisible()

    const emptyState = tableCard.locator('.ui-empty-state')
    const table = tableCard.locator('table')

    if (await emptyState.count()) {
      await expect(emptyState.first()).toBeVisible()
    } else {
      await expect(table).toBeVisible()
      const headerCells = table.locator('thead th')
      expect(await headerCells.count()).toBeGreaterThan(3)
      const bodyText = await tableCard.innerText()
      expect(hasBrokenUiValues(bodyText)).toBe(false)
    }
  })

  test('CT-E2E-005 Critical events', async ({ page }) => {
    await loginViaApi(page)
    await gotoControlTower(page)
    await waitForControlTowerLoaded(page)

    const eventsCard = page.locator('.control-tower-v01__events-card')
    await expect(eventsCard).toBeVisible()

    const eventsPanel = eventsCard.locator('.critical-events')
    await expect(eventsPanel).toBeVisible()

    const bodyText = await eventsPanel.innerText()
    expect(hasBrokenUiValues(bodyText)).toBe(false)
    expect(bodyText.length).toBeGreaterThan(0)

    await screenshot(page, '03-critical-events')
  })

  test('CT-E2E-006 Filters', async ({ page }) => {
    await loginViaApi(page)
    await gotoControlTower(page)
    await waitForControlTowerLoaded(page)

    const isDemoMode = await page.locator('.control-tower-v01__demo-banner').isVisible()
    const summaryRequests = []
    page.on('request', (req) => {
      if (req.url().includes('/api/v1/control-tower/summary')) {
        summaryRequests.push(req.url())
      }
    })

    const searchInput = page.locator('.filters-row .ui-input__control').first()
    await searchInput.fill('DEMO-SH-001')
    await page.waitForTimeout(600)

    const statusSelect = page.locator('.filters-row select').first()
    if (await statusSelect.count()) {
      await statusSelect.selectOption({ index: 1 })
    }
    await page.waitForTimeout(500)

    expect(page.url()).toMatch(/q=DEMO|status=/i)

    const resetButton = page.getByRole('button', { name: /Reset|Сброс/i })
    if (await resetButton.count()) {
      await resetButton.click()
      await page.waitForTimeout(500)
    }

    if (!isDemoMode) {
      expect(summaryRequests.length).toBeGreaterThan(0)
    }

    const bodyText = await page.locator('.control-tower-v01').innerText()
    expect(hasBrokenUiValues(bodyText)).toBe(false)

    await screenshot(page, '04-filter-applied')
  })

  test('CT-E2E-007 Manual refresh', async ({ page }) => {
    await loginViaApi(page)
    await gotoControlTower(page)
    await waitForControlTowerLoaded(page)

    const refreshButton = page.getByRole('button', { name: /Refresh|Обновить/i }).first()
    await expect(refreshButton).toBeVisible()

    const beforeText = await page.locator('.ct-toolbar').innerText()
    await refreshButton.click()
    await waitForControlTowerLoaded(page)
    await refreshButton.click()
    await waitForControlTowerLoaded(page)

    const afterText = await page.locator('.ct-toolbar').innerText()
    expect(afterText).toMatch(/Last updated|Последнее обновление/i)
    expect(beforeText).toMatch(/Last updated|Последнее обновление/i)
    expect(await page.locator('.metric-card').count()).toBeGreaterThan(0)
  })

  test('CT-E2E-008 Auto-refresh', async ({ page }) => {
    await loginViaApi(page)
    await gotoControlTower(page)
    await waitForControlTowerLoaded(page)

    const autoCheckbox = page.locator('.ct-toolbar input[type="checkbox"]')
    await expect(autoCheckbox).toBeVisible()

    let summaryRequests = 0
    page.on('request', (req) => {
      if (req.url().includes('/api/v1/control-tower/summary')) summaryRequests += 1
    })

    const before = summaryRequests
    await autoCheckbox.check()
    await expect(autoCheckbox).toBeChecked()
    await page.waitForTimeout(3000)

    const afterEnable = summaryRequests
    expect(afterEnable).toBeGreaterThanOrEqual(before)

    const burstStart = summaryRequests
    await page.waitForTimeout(5000)
    const burstDelta = summaryRequests - burstStart
    expect(burstDelta).toBeLessThan(5)

    await autoCheckbox.uncheck()
    await expect(page.locator('.filters-row')).toBeVisible()
  })

  test('CT-E2E-009 Empty state', async ({ page }) => {
    await loginViaApi(page)
    await gotoControlTower(page)
    await waitForControlTowerLoaded(page)

    const searchInput = page.locator('.filters-row .ui-input__control').first()
    await searchInput.fill('zzzz-no-match-e2e-000')
    await page.waitForTimeout(600)
    await waitForControlTowerLoaded(page)

    await expect(
      page.getByRole('heading', { name: /No shipments match filters|No active shipments|Нет перевозок/i }),
    ).toBeVisible()

    const bodyText = await page.locator('.control-tower-v01').innerText()
    expect(hasBrokenUiValues(bodyText)).toBe(false)
  })

  test('CT-E2E-010 API unavailable', async ({ page }) => {
    await loginViaApi(page)

    await page.route('**/api/v1/control-tower/summary**', (route) =>
      route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ error: { code: 'SERVICE_UNAVAILABLE', message: 'unavailable' } }) }),
    )

    await gotoControlTower(page)
    await page.waitForSelector('.api-unavailable, .control-tower-v01__demo-banner', { timeout: 30_000 })

    const unavailable = page.locator('.api-unavailable')
    const demoBanner = page.locator('.control-tower-v01__demo-banner')

    if (await unavailable.count()) {
      await expect(unavailable.first()).toBeVisible()
      const text = await unavailable.first().innerText()
      expect(text).not.toMatch(/stack trace|secret|password|Bearer/i)
    } else if (await demoBanner.count()) {
      await expect(demoBanner).toBeVisible()
    } else {
      const bodyText = await page.locator('body').innerText()
      expect(bodyText).not.toMatch(/\[object Object\]|undefined/)
    }

    await screenshot(page, '05-api-unavailable')
    await page.unroute('**/api/v1/control-tower/summary**')
  })

  test('CT-E2E-011 RBAC forbidden', async ({ browser }) => {
    const restrictedUser = {
      token: `e2e-restricted-token-${Date.now()}`,
      user: {
        id: 'e2e-user-restricted',
        tenant_id: E2E_CONFIG.tenantId,
        email: 'consignee-viewer@7rights.local',
        full_name: 'E2E Consignee Viewer',
        preferred_locale: 'en-US',
        status: 'ACTIVE',
        roles: ['CONSIGNEE_VIEWER'],
      },
    }

    const context = await browser.newContext({ baseURL: E2E_CONFIG.frontendUrl })
    await context.addInitScript((session) => {
      localStorage.setItem('freight_admin_session', JSON.stringify(session))
      localStorage.setItem('freight_admin_tenant_id', session.user.tenant_id)
    }, restrictedUser)
    const page = await context.newPage()

    await page.goto('/dashboard', { waitUntil: 'domcontentloaded' })
    await expect(page.getByRole('link', { name: /Control Tower/i })).toHaveCount(0)

    await page.goto('/control-tower', { waitUntil: 'domcontentloaded' })
    const pathname = new URL(page.url()).pathname
    const onControlTowerRoute = pathname === '/control-tower'
    const deniedHeading = page.getByRole('heading', { name: /Access denied|доступ запрещён/i })
    const hasInPageDeny = (await deniedHeading.count()) > 0
    const redirectedAway = !onControlTowerRoute

    expect(redirectedAway || hasInPageDeny).toBe(true)
    await expect(page.locator('.control-tower-v01__table-card table tbody tr')).toHaveCount(0)

    await context.close()
  })

  test('CT-E2E-012 Shipment Event History', async ({ page }) => {
    await loginViaApi(page)
    await gotoControlTower(page)
    await waitForControlTowerLoaded(page)

    const eventLink = page.locator('a[href*="/events"]').first()
    if (await eventLink.count()) {
      await eventLink.click()
      await page.waitForURL(/\/shipments\/.+\/events/, { timeout: 20_000 })
    } else {
      await page.goto(`/shipments/${E2E_CONFIG.foreignShipmentId}/events`, { waitUntil: 'networkidle' })
    }

    await page.waitForSelector('.page-stack, .state-card, .ui-empty-state', { timeout: 20_000 })

    const bodyText = await page.locator('.page-stack, body').first().innerText()
    expect(hasBrokenUiValues(bodyText)).toBe(false)
    expect(bodyText.length).toBeGreaterThan(0)

    await screenshot(page, '06-shipment-event-history')
  })

  test('CT-E2E-013 Tenant isolation', async ({ page, request }) => {
    await loginViaApi(page)

    const token = await page.evaluate(() => {
      const raw = localStorage.getItem('freight_admin_session')
      if (!raw) return null
      const parsed = JSON.parse(raw)
      return parsed.token
    })
    expect(token).toBeTruthy()

    const foreignShipmentId = E2E_CONFIG.foreignShipmentId
    const spoofedTenant = E2E_CONFIG.foreignTenantId

    const withSpoofedHeader = await request.get(
      `${E2E_CONFIG.backendUrl}/api/v1/shipments/${foreignShipmentId}/events`,
      {
        headers: {
          Authorization: `Bearer ${token}`,
          'X-Tenant-ID': spoofedTenant,
          Accept: 'application/json',
        },
      },
    )

    expect([404, 403]).toContain(withSpoofedHeader.status())

    const withQueryTenant = await request.get(
      `${E2E_CONFIG.backendUrl}/api/v1/shipments/${foreignShipmentId}/events?tenant_id=${spoofedTenant}`,
      {
        headers: {
          Authorization: `Bearer ${token}`,
          Accept: 'application/json',
        },
      },
    )

    expect([404, 403]).toContain(withQueryTenant.status())

    await page.goto(`/shipments/${foreignShipmentId}/events`, { waitUntil: 'networkidle' })
    const pageText = await page.locator('body').innerText()
    expect(pageText).not.toMatch(new RegExp(spoofedTenant, 'i'))
    expect(hasBrokenUiValues(pageText)).toBe(false)
  })
})
