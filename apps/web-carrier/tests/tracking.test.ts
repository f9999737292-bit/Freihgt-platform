import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { PortalClient } from '@freight-platform/portal-client'
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import TrackingBoard from '../components/tracking/TrackingBoard.vue'
import {
  CARRIER_TRACKING_SUFFIXES,
  FORBIDDEN_BUSINESS_FACT_KEYS,
  carrierReadSpec,
  type ETASummary,
  type LocationFact,
  type SlotSummary,
  type TrackingSummary,
} from '../domain/tracking'
import type { PortalViewKind } from '../domain/viewState'

const LEAKS = ['providerDeviceId', 'providerEventId', 'providerCode', 'providerConfidence', 'providerSlotId']

const freshTracking: TrackingSummary = {
  trackingStatus: 'ACTIVE',
  freshness: { status: 'fresh', ageSeconds: 12 },
  quality: { status: 'good' },
  lastKnownPosition: {
    latitude: 55.75,
    longitude: 37.62,
    recordedAt: '2026-12-01T10:00:00Z',
    ageSeconds: 12,
  },
  speedKph: 40,
}

const location: LocationFact = {
  recordedAt: '2026-12-01T10:00:00Z',
  latitude: 55.75,
  longitude: 37.62,
  speedKph: 40,
  headingDegrees: 90,
  accuracyMeters: 8,
  quality: { status: 'good' },
}

const eta: ETASummary = {
  pickup: {
    status: 'AVAILABLE',
    freshnessStatus: 'fresh',
    qualityStatus: 'good',
    estimatedArrivalAt: '2026-12-01T11:00:00Z',
  },
  delivery: null,
}

const slots: SlotSummary = {
  pickup: {
    windowStatus: 'OPEN',
    windowStart: '2026-12-01T08:00:00Z',
    windowEnd: '2026-12-01T10:00:00Z',
    timezone: 'Europe/Moscow',
    qualityStatus: 'good',
  },
  delivery: null,
}

function board(overrides: Record<string, unknown> = {}) {
  return mount(TrackingBoard, {
    props: {
      trackingKind: 'ready' as PortalViewKind,
      tracking: freshTracking,
      trackingTitle: 'Loading',
      locationsKind: 'ready' as PortalViewKind,
      locations: [location],
      locationsTitle: 'Loading',
      locationsTotal: 1,
      locationsOffset: 0,
      etaKind: 'ready' as PortalViewKind,
      eta,
      etaTitle: 'Loading',
      etaHistoryKind: 'ready' as PortalViewKind,
      etaHistoryTitle: 'Loading',
      pickupEtaHistory: [],
      deliveryEtaHistory: [],
      slotsKind: 'ready' as PortalViewKind,
      slots,
      slotsTitle: 'Loading',
      slotHistoryKind: 'ready' as PortalViewKind,
      slotHistoryTitle: 'Loading',
      pickupSlotHistory: [],
      deliverySlotHistory: [],
      ...overrides,
    },
  })
}

describe('carrier tracking board', () => {
  it('renders a fresh tracking state and last known position', () => {
    const view = board()
    expect(view.get('[data-testid="tracking-status"]').text()).toBe('ACTIVE')
    expect(view.get('[data-testid="tracking-freshness"]').text()).toBe('fresh')
    expect(view.get('[data-testid="position-latitude"]').text()).toBe('55.75')
    expect(view.get('[data-testid="position-longitude"]').text()).toBe('37.62')
  })

  it('keeps a stale tracking state visible', () => {
    const view = board({
      tracking: {
        trackingStatus: 'STALE',
        freshness: { status: 'stale', ageSeconds: 900 },
        quality: { status: 'unknown' },
        lastKnownPosition: freshTracking.lastKnownPosition,
      },
    })
    expect(view.get('[data-testid="tracking-status"]').text()).toBe('STALE')
    expect(view.get('[data-testid="tracking-freshness"]').text()).toBe('stale')
    expect(view.get('[data-testid="tracking-section"]').text()).toContain('900')
  })

  it('renders NOT_CONFIGURED and a missing position as product states', () => {
    const view = board({
      tracking: {
        trackingStatus: 'NOT_CONFIGURED',
        freshness: { status: 'unknown' },
        quality: { status: 'unknown' },
        lastKnownPosition: null,
      },
    })
    expect(view.get('[data-testid="tracking-not-configured"]').text()).toContain('carrier.trackingNotConfigured')
    expect(view.get('[data-testid="tracking-status"]').text()).toBe('NOT_CONFIGURED')
    expect(view.find('[data-testid="position-absent"]').exists()).toBe(true)
  })

  it('shows an empty location history without provider identifiers', () => {
    const leaked = {
      ...location,
      providerDeviceId: 'provider-device-secret',
      providerEventId: 'provider-event-secret',
      providerCode: 'provider-code-secret',
      providerConfidence: 'provider-confidence-secret',
      providerSlotId: 'provider-slot-secret',
    }
    const filled = board({ locations: [leaked], locationsTotal: 1 })
    expect(filled.get('[data-testid="location-row"]').text()).toContain('55.75')
    for (const leak of [...LEAKS, 'provider-device-secret', 'provider-event-secret', 'provider-slot-secret']) {
      expect(filled.text()).not.toContain(leak)
    }
    const empty = board({ locations: [], locationsTotal: 0 })
    expect(empty.get('[data-testid="locations-empty"]').text()).toContain('carrier.noLocations')
  })

  it('renders pickup ETA and an absent delivery forecast', () => {
    const view = board()
    expect(view.get('[data-testid="eta-pickup"]').text()).toContain('2026-12-01T11:00:00Z')
    expect(view.find('[data-testid="eta-delivery-absent"]').exists()).toBe(true)
    expect(view.get('[data-testid="eta-delivery-absent"]').text()).toContain('carrier.forecastAbsent')
  })

  it('renders a pickup window timezone and an absent delivery window', () => {
    const view = board()
    expect(view.get('[data-testid="window-timezone"]').text()).toBe('Europe/Moscow')
    expect(view.get('[data-testid="window-start"]').text()).toBe('2026-12-01T08:00:00Z')
    expect(view.find('[data-testid="slot-delivery-absent"]').exists()).toBe(true)
    expect(view.get('[data-testid="slot-delivery-absent"]').text()).toContain('carrier.windowAbsent')
  })

  it('keeps the other sections when one request fails', () => {
    const unavailable = board({
      trackingKind: 'unavailable',
      tracking: null,
      trackingTitle: 'Unavailable',
    })
    expect(unavailable.get('[data-testid="tracking-section"]').text()).toContain('Unavailable')
    expect(unavailable.get('[data-testid="eta-pickup"]').text()).toContain('AVAILABLE')
    expect(unavailable.get('[data-testid="slot-pickup"]').text()).toContain('OPEN')
    expect(unavailable.find('[data-testid="location-row"]').exists()).toBe(true)

    const slotsMissing = board({ slotsKind: 'not_found', slots: null, slotsTitle: 'Not found' })
    expect(slotsMissing.get('[data-testid="slots-section"]').text()).toContain('Not found')
    expect(slotsMissing.get('[data-testid="position-latitude"]').text()).toBe('55.75')
  })

  it('shows forbidden and not found without a write control', () => {
    const forbidden = board({ trackingKind: 'forbidden', tracking: null, trackingTitle: 'Forbidden' })
    expect(forbidden.text()).toContain('Forbidden')
    expect(forbidden.find('[data-testid="change-status"]').exists()).toBe(false)
    expect(forbidden.find('[data-testid="book-slot"]').exists()).toBe(false)
    expect(forbidden.find('input[type="file"]').exists()).toBe(false)
  })
})

describe('carrier tracking request contract', () => {
  it('uses the six carrier routes, the selected company, and a history cap of 50', async () => {
    const calls: string[] = []
    const client = new PortalClient({
      baseUrl: 'http://gateway.test',
      getAccessToken: () => 'token',
      getAllowedCompanyIds: () => ['co-1'],
      fetchImpl: async (input, init) => {
        const url = new URL(String(input))
        const headers = new Headers(init?.headers)
        calls.push(`${url.pathname}${url.search}`)
        expect(url.pathname.startsWith('/api/v1/carrier/shipments/shipment-A')).toBe(true)
        expect(url.pathname.startsWith('/api/v1/shipments/')).toBe(false)
        expect(url.searchParams.get('carrier_company_id')).toBe('co-1')
        expect(headers.get('x-company-id')).toBe('co-1')
        for (const header of ['x-tenant-id', 'x-user-id', 'x-user-email', 'x-actor-kind', 'x-user-role', 'x-role', 'x-roles']) {
          expect(headers.get(header)).toBeNull()
        }
        for (const key of FORBIDDEN_BUSINESS_FACT_KEYS) {
          expect(url.searchParams.has(key)).toBe(false)
        }
        expect(init?.method ?? 'GET').toBe('GET')
        return new Response('{}', { status: 200 })
      },
    })

    for (const suffix of CARRIER_TRACKING_SUFFIXES) {
      const history = suffix.endsWith('locations')
        ? { limit: 500, offset: -1, from: '2026-12-01T00:00:00Z', to: '2026-12-02T00:00:00Z' }
        : suffix.endsWith('eta/history')
          ? { targetType: 'pickup' as const, limit: 80, offset: 0 }
          : suffix.endsWith('slots/history')
            ? { slotType: 'delivery' as const, limit: 50, offset: 0 }
            : undefined
      const spec = carrierReadSpec('shipment-A', suffix, 'co-1', history)
      await client.request(spec.path, { companyId: 'co-1', query: spec.query })
    }

    expect(calls).toHaveLength(CARRIER_TRACKING_SUFFIXES.length)
    expect(calls.some((call) => call.includes('/tracking/locations') && call.includes('limit=50') && call.includes('from=') && call.includes('offset=0'))).toBe(true)
    expect(calls.some((call) => call.includes('/eta/history') && call.includes('targetType=pickup') && call.includes('limit=50'))).toBe(true)
    expect(calls.some((call) => call.includes('/slots/history') && call.includes('slotType=delivery'))).toBe(true)
    expect(calls.some((call) => call.includes('limit=500') || call.includes('limit=80') || call.includes('execution_stop'))).toBe(false)
  })

  it('rejects authored ETA and slot facts and any non pickup or delivery target', () => {
    expect(() => carrierReadSpec('shipment-A', '/eta', 'co-1', { pickupEstimatedArrivalAt: '2026-12-01T11:00:00Z' } as never)).toThrow(/business fact/)
    expect(() => carrierReadSpec('shipment-A', '/slots', 'co-1', { windowStart: '2026-12-01T08:00:00Z' } as never)).toThrow(/business fact/)
    expect(() => carrierReadSpec('shipment-A', '/eta/history', 'co-1', { targetType: 'execution_stop' } as never)).toThrow(/eta target/)
    expect(() => carrierReadSpec('shipment-A', '/slots/history', 'co-1', { slotType: 'stop' } as never)).toThrow(/slot type/)
    const locations = carrierReadSpec('shipment-A', '/tracking/locations', 'co-1', { from: '2026-12-01T00:00:00Z', limit: 20 })
    expect(Object.keys(locations.query).sort()).toEqual(['carrier_company_id', 'from', 'limit', 'offset'])
  })

  it('keeps provider fields and legacy routes out of the carrier tracking screen', () => {
    const files = [
      'domain/tracking.ts',
      'components/tracking/TrackingBoard.vue',
      'components/tracking/EtaFacts.vue',
      'components/tracking/SlotFacts.vue',
      'components/tracking/EtaHistoryTable.vue',
      'components/tracking/SlotHistoryTable.vue',
      'pages/shipments/[id]/tracking.vue',
      'pages/transport-orders/index.vue',
      'pages/transport-orders/[id].vue',
      'layouts/default.vue',
    ]
    for (const file of files) {
      const source = readFileSync(join(process.cwd(), file), 'utf8')
      expect(source.includes('/api/v1/shipments/')).toBe(false)
      for (const leak of LEAKS) expect(source).not.toContain(leak)
    }
    const api = readFileSync(join(process.cwd(), 'composables/useCarrierApi.ts'), 'utf8')
    const trackingSlice = api.slice(api.indexOf('function readTracking'), api.indexOf('async function listFleet'))
    expect(trackingSlice).toContain('carrierReadSpec')
    expect(trackingSlice).toContain('office.requireCompany')
    expect(trackingSlice).not.toMatch(/method:\s*'(POST|PUT|PATCH|DELETE)'/)
    const list = readFileSync(join(process.cwd(), 'pages/transport-orders/index.vue'), 'utf8')
    const detail = readFileSync(join(process.cwd(), 'pages/transport-orders/[id].vue'), 'utf8')
    expect(list).toContain('item.shipment_id')
    expect(list).not.toContain('transport_order_id}/tracking')
    expect(detail).toContain('order.shipment?.id')
    expect(detail).not.toContain('transport_order_id}/tracking')
    const layout = readFileSync(join(process.cwd(), 'layouts/default.vue'), 'utf8')
    expect(layout).not.toContain('openTracking')
    const domain = readFileSync(join(process.cwd(), 'domain/tracking.ts'), 'utf8')
    expect(domain).toContain('/api/v1/carrier/shipments/')
    expect(domain).not.toContain('execution_stop')
  })
})
