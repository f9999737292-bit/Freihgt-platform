import { expect, test, type Page } from '@playwright/test'
import {
  assertNoErpIntegrationCalls,
  attachHumanProvenanceNetworkProbe,
  expectWorkspaceLoaded,
  formatHumanGetDenial,
  formatHumanProvenanceNetworkProbe,
  requireHumanProvenanceEnv,
} from './helpers'

function liveFixture() {
  return {
    jwt: requireHumanProvenanceEnv('BROWSER_E2E_JWT'),
    tenantId: requireHumanProvenanceEnv('BROWSER_E2E_TENANT_ID'),
    companyId: requireHumanProvenanceEnv('BROWSER_E2E_BUYER_COMPANY_ID'),
    eventId: requireHumanProvenanceEnv('BROWSER_E2E_EVENT_ID'),
    userId: requireHumanProvenanceEnv('BROWSER_E2E_USER_ID'),
    gatewayURL: requireHumanProvenanceEnv('BROWSER_E2E_GATEWAY_URL'),
    rfxNumber: requireHumanProvenanceEnv('BROWSER_E2E_RFX_NUMBER'),
    carrierJwt: requireHumanProvenanceEnv('BROWSER_E2E_CARRIER_JWT'),
    carrierUserId: requireHumanProvenanceEnv('BROWSER_E2E_CARRIER_USER_ID'),
    carrierCompanyId: requireHumanProvenanceEnv('BROWSER_E2E_CARRIER_COMPANY_ID'),
    otherBuyerJwt: requireHumanProvenanceEnv('BROWSER_E2E_OTHER_BUYER_JWT'),
    otherBuyerUserId: requireHumanProvenanceEnv('BROWSER_E2E_OTHER_BUYER_USER_ID'),
    otherBuyerCompanyId: requireHumanProvenanceEnv('BROWSER_E2E_OTHER_BUYER_COMPANY_ID'),
  }
}

async function seedLiveSession(
  page: Page,
  input: { token: string; user: string; company: string; roles: string[] },
) {
  const fix = liveFixture()
  await page.addInitScript((payload) => {
    localStorage.setItem(
      'freight_procurement_session',
      JSON.stringify({
        token: payload.token,
        user: {
          id: payload.user,
          tenant_id: payload.tenant,
          email: 'human-provenance-live@freight.test',
          full_name: 'Human Provenance Live',
          preferred_locale: 'en-US',
          status: 'ACTIVE',
          roles: payload.roles,
        },
      }),
    )
    localStorage.setItem('freight_procurement_tenant_id', payload.tenant)
    localStorage.setItem('freight_procurement_company_id', payload.company)
    document.cookie = 'freight_procurement_locale=en-US; path=/'
  }, {
    token: input.token,
    tenant: fix.tenantId,
    company: input.company,
    user: input.user,
    roles: input.roles,
  })
}

test.describe('human provenance live stack', () => {
  test('buyer sees stored channel from human GET without ERP integration calls', async ({ page }) => {
    const fix = liveFixture()
    const probe = attachHumanProvenanceNetworkProbe(page)
    await seedLiveSession(page, {
      token: fix.jwt,
      user: fix.userId,
      company: fix.companyId,
      roles: ['PROCUREMENT_MANAGER'],
    })
    const eventGET = page.waitForResponse((resp) => {
      const url = resp.url()
      return (
        url.includes(`/api/v1/rfx-events/${fix.eventId}`)
        && !url.includes('/lots')
        && !url.includes('/participants')
        && !url.includes('/xlsx')
        && resp.request().method() === 'GET'
      )
    }, { timeout: 30_000 })
    await page.goto(`/tenders/${fix.eventId}`, { waitUntil: 'domcontentloaded' })
    const loaded = await eventGET
    expect(loaded.status(), formatHumanProvenanceNetworkProbe(probe)).toBe(200)
    expect(loaded.url()).toContain(`/api/v1/rfx-events/${fix.eventId}`)
    expect(loaded.url()).not.toContain('/integrations/erp/')
    const body = await loaded.json() as { creation_channel?: string; external_link?: unknown }
    expect(body.creation_channel).toBe('MANUAL')
    expect(body.external_link).toBeUndefined()
    await expectWorkspaceLoaded(page, fix.rfxNumber)
    await expect(page.getByTestId('tender-creation-channel')).toHaveText('Created manually')
    assertNoErpIntegrationCalls(probe)
  })

  test('carrier does not receive the event or channel', async ({ page }) => {
    await expectDeniedHumanGet(page, {
      token: liveFixture().carrierJwt,
      user: liveFixture().carrierUserId,
      company: liveFixture().carrierCompanyId,
      roles: ['CARRIER_DISPATCHER'],
      expectedStatus: 403,
      fixture: 'seedHumanProvenanceBrowserFixture.CarrierAct',
      layer: 'api-gateway PolicyBuyerRead on GET /api/v1/rfx-events/{id}; carrier must use /api/v1/carrier/rfx-events/{id}',
    })
  })

  test('foreign company buyer does not receive the event or channel', async ({ page }) => {
    await expectDeniedHumanGet(page, {
      token: liveFixture().otherBuyerJwt,
      user: liveFixture().otherBuyerUserId,
      company: liveFixture().otherBuyerCompanyId,
      roles: ['PROCUREMENT_MANAGER'],
      expectedStatus: 404,
      fixture: 'seedHumanProvenanceBrowserFixture.BuyerB/CompanyB',
      layer: 'rfx-service requireOwnerCompanyAccess returns NOT_FOUND for foreign owner company',
    })
  })
})

async function expectDeniedHumanGet(
  page: Page,
  actor: {
    token: string
    user: string
    company: string
    roles: string[]
    expectedStatus: number
    fixture: string
    layer: string
  },
) {
  const fix = liveFixture()
  const probe = attachHumanProvenanceNetworkProbe(page)
  await seedLiveSession(page, actor)
  const path = `/api/v1/rfx-events/${fix.eventId}`
  const eventGET = page.waitForResponse((resp) => {
    const url = resp.url()
    return (
      url.includes(path)
      && !url.includes('/lots')
      && !url.includes('/participants')
      && resp.request().method() === 'GET'
    )
  }, { timeout: 30_000 })
  await page.goto(`/tenders/${fix.eventId}`, { waitUntil: 'domcontentloaded' })
  const loaded = await eventGET
  expect(loaded.status(), formatHumanGetDenial({
    method: 'GET',
    path,
    status: loaded.status(),
    role: actor.roles.join(','),
    companyId: actor.company,
    fixture: actor.fixture,
    layer: actor.layer,
    probe,
  })).toBe(actor.expectedStatus)
  await expect(page.getByTestId('tender-not-found')).toBeVisible()
  await expect(page.getByTestId('tender-not-found')).toContainText('Tender not found')
  await expect(page.getByTestId('tender-creation-channel')).toHaveCount(0)
  await expect(page.locator('body')).not.toContainText('Created manually')
  await expect(page.locator('body')).not.toContainText('MANUAL')
  assertNoErpIntegrationCalls(probe)
}
