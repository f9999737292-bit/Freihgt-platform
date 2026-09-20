import { expect, test } from '@playwright/test'
import {
  assertNoErpIntegrationCalls,
  attachHumanProvenanceNetworkProbe,
  eventId,
  expectWorkspaceLoaded,
  formatHumanGetDenial,
  formatHumanProvenanceNetworkProbe,
  seedBuyerSession,
  stubTenderWorkspace,
} from './helpers'

test.describe('human provenance creation_channel', () => {
  test('buyer sees localized channel from human GET and never calls ERP integration', async ({ page }) => {
    const probe = attachHumanProvenanceNetworkProbe(page)
    await seedBuyerSession(page)
    await stubTenderWorkspace(page)
    const eventGET = page.waitForResponse((resp) => {
      const url = new URL(resp.url())
      return resp.request().method() === 'GET' && url.pathname === `/api/v1/rfx-events/${eventId}`
    }, { timeout: 30_000 })
    await page.goto(`/tenders/${eventId}`, { waitUntil: 'domcontentloaded' })
    const loaded = await eventGET
    expect(loaded.status(), formatHumanProvenanceNetworkProbe(probe)).toBe(200)
    const body = await loaded.json() as { creation_channel?: string; external_link?: unknown }
    expect(body.creation_channel).toBe('MANUAL')
    expect(body.external_link).toBeUndefined()
    await expectWorkspaceLoaded(page, 'RFX-F4-1')
    await expect(page.getByTestId('tender-creation-channel')).toHaveText('Created manually')
    await expect(page.locator('body')).not.toContainText('external_link')
    assertNoErpIntegrationCalls(probe)
  })

  test('unknown channel stays localized and does not show the raw code', async ({ page }) => {
    const probe = attachHumanProvenanceNetworkProbe(page)
    await seedBuyerSession(page)
    await stubTenderWorkspace(page, {
      body: {
        id: eventId,
        tenant_id: 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa',
        owner_company_id: 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb',
        rfx_number: 'RFX-F4-1',
        title: 'Human provenance draft',
        status: 'DRAFT',
        rfx_type: 'LANE_TENDER',
        category: 'FREIGHT',
        creation_channel: 'SAP',
      },
    })
    await page.goto(`/tenders/${eventId}`, { waitUntil: 'domcontentloaded' })
    await expectWorkspaceLoaded(page, 'RFX-F4-1')
    await expect(page.getByTestId('tender-creation-channel')).toHaveText('Unknown creation channel')
    await expect(page.getByTestId('tender-creation-channel')).not.toHaveText('SAP')
    await expect(page.locator('body')).not.toContainText('SAP')
    assertNoErpIntegrationCalls(probe)
  })

  test('403 keeps the existing empty state and hides channel data', async ({ page }) => {
    const probe = attachHumanProvenanceNetworkProbe(page)
    await seedBuyerSession(page)
    await stubTenderWorkspace(page, {
      status: 403,
      body: { error: { code: 'FORBIDDEN', message: 'denied', details: {} } },
    })
    const eventGET = page.waitForResponse((resp) => {
      const url = new URL(resp.url())
      return resp.request().method() === 'GET' && url.pathname === `/api/v1/rfx-events/${eventId}`
    }, { timeout: 30_000 })
    await page.goto(`/tenders/${eventId}`, { waitUntil: 'domcontentloaded' })
    const loaded = await eventGET
    expect(loaded.status(), formatHumanGetDenial({
      method: 'GET',
      path: `/api/v1/rfx-events/${eventId}`,
      status: loaded.status(),
      role: 'PROCUREMENT_MANAGER',
      companyId: 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb',
      fixture: 'helpers.stubTenderWorkspace status=403',
      layer: 'playwright stub FORBIDDEN on buyer human GET',
      probe,
    })).toBe(403)
    await expect(page.getByTestId('tender-not-found')).toBeVisible()
    await expect(page.getByTestId('tender-not-found')).toContainText('Tender not found')
    await expect(page.getByTestId('tender-creation-channel')).toHaveCount(0)
    await expect(page.locator('body')).not.toContainText('Created manually')
    await expect(page.locator('body')).not.toContainText('MANUAL')
    assertNoErpIntegrationCalls(probe)
  })

  test('404 keeps the existing empty state and hides channel data', async ({ page }) => {
    const probe = attachHumanProvenanceNetworkProbe(page)
    await seedBuyerSession(page)
    await stubTenderWorkspace(page, {
      status: 404,
      body: { error: { code: 'NOT_FOUND', message: 'denied', details: {} } },
    })
    const eventGET = page.waitForResponse((resp) => {
      const url = new URL(resp.url())
      return resp.request().method() === 'GET' && url.pathname === `/api/v1/rfx-events/${eventId}`
    }, { timeout: 30_000 })
    await page.goto(`/tenders/${eventId}`, { waitUntil: 'domcontentloaded' })
    const loaded = await eventGET
    expect(loaded.status(), formatHumanGetDenial({
      method: 'GET',
      path: `/api/v1/rfx-events/${eventId}`,
      status: loaded.status(),
      role: 'PROCUREMENT_MANAGER',
      companyId: 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb',
      fixture: 'helpers.stubTenderWorkspace status=404',
      layer: 'playwright stub NOT_FOUND on buyer human GET',
      probe,
    })).toBe(404)
    await expect(page.getByTestId('tender-not-found')).toBeVisible()
    await expect(page.getByTestId('tender-not-found')).toContainText('Tender not found')
    await expect(page.getByTestId('tender-creation-channel')).toHaveCount(0)
    await expect(page.locator('body')).not.toContainText('Created manually')
    await expect(page.locator('body')).not.toContainText('MANUAL')
    assertNoErpIntegrationCalls(probe)
  })
})
