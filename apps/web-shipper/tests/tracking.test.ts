import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { PortalClient } from '@freight-platform/portal-client'
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import TrackingBoard from '../components/tracking/TrackingBoard.vue'
import {
  FORBIDDEN_BUSINESS_FACT_KEYS,
  SHIPPER_TRACKING_SUFFIXES,
  buildHistoryQuery,
  shipperReadSpec,
  type ETASummary,
  type LocationFact,
  type SlotSummary,
  type TrackingSummary,
} from '../domain/tracking'
import type { PortalViewKind } from '../domain/viewState'

const LEAKS = ['providerDeviceId', 'providerEventId', 'providerSlotId', 'X-Internal-Service-Token']

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

describe('tracking board', () => {
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

  it('renders NOT_CONFIGURED as a named tracking state', () => {
    const view = board({
      tracking: {
        trackingStatus: 'NOT_CONFIGURED',
        freshness: { status: 'unknown' },
        quality: { status: 'unknown' },
        lastKnownPosition: null,
      },
    })
    expect(view.get('[data-testid="tracking-not-configured"]').text()).toContain('shipper.trackingNotConfigured')
    expect(view.get('[data-testid="tracking-status"]').text()).toBe('NOT_CONFIGURED')
    expect(view.get('[data-testid="tracking-freshness"]').text()).toBe('unknown')
    expect(view.find('[data-testid="position-absent"]').exists()).toBe(true)
  })

  it('shows an empty location history without provider identifiers', () => {
    const leaked = {
      ...location,
      providerDeviceId: 'provider-device-secret',
      providerEventId: 'provider-event-secret',
      providerSlotId: 'provider-slot-secret',
      serviceToken: 'X-Internal-Service-Token',
    }
    const filled = board({ locations: [leaked], locationsTotal: 1 })
    const row = filled.get('[data-testid="location-row"]').text()
    expect(row).toContain('55.75')
    expect(row).toContain('good')
    for (const leak of [...LEAKS, 'provider-device-secret', 'provider-event-secret', 'provider-slot-secret']) {
      expect(filled.text()).not.toContain(leak)
    }

    const empty = board({ locations: [], locationsTotal: 0 })
    expect(empty.get('[data-testid="locations-empty"]').text()).toContain('shipper.noLocations')
  })

  it('renders pickup ETA and an absent delivery forecast, including stale facts', () => {
    const view = board({
      eta: {
        pickup: eta.pickup,
        delivery: {
          status: 'EXPIRED',
          freshnessStatus: 'stale',
          qualityStatus: 'unknown',
          estimatedArrivalAt: '2026-12-02T16:00:00Z',
        },
      },
    })
    expect(view.get('[data-testid="eta-pickup"]').text()).toContain('2026-12-01T11:00:00Z')
    expect(view.get('[data-testid="eta-delivery"]').text()).toContain('EXPIRED')
    expect(view.get('[data-testid="eta-delivery"]').text()).toContain('stale')

    const absent = board()
    expect(absent.find('[data-testid="eta-delivery-absent"]').exists()).toBe(true)
    expect(absent.get('[data-testid="eta-pickup"]').text()).toContain('AVAILABLE')
  })

  it('renders a pickup window and an absent delivery window', () => {
    const view = board()
    expect(view.get('[data-testid="slot-pickup"]').text()).toContain('2026-12-01T08:00:00Z')
    expect(view.get('[data-testid="slot-pickup"]').text()).toContain('Europe/Moscow')
    expect(view.find('[data-testid="slot-delivery-absent"]').exists()).toBe(true)
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

    const etaDown = board({ etaKind: 'unavailable', eta: null, etaTitle: 'Unavailable' })
    expect(etaDown.get('[data-testid="eta-section"]').text()).toContain('Unavailable')
    expect(etaDown.get('[data-testid="tracking-status"]').text()).toBe('ACTIVE')
    expect(etaDown.find('[data-testid="slot-pickup"]').exists()).toBe(true)

    const slotsDown = board({ slotsKind: 'unavailable', slots: null, slotsTitle: 'Unavailable' })
    expect(slotsDown.get('[data-testid="slots-section"]').text()).toContain('Unavailable')
    expect(slotsDown.find('[data-testid="position-latitude"]').exists()).toBe(true)
    expect(slotsDown.get('[data-testid="position-latitude"]').text()).toBe('55.75')
  })

  it('shows forbidden, not found, and unavailable copy', () => {
    expect(board({ trackingKind: 'forbidden', tracking: null, trackingTitle: 'Forbidden' }).text()).toContain('Forbidden')
    expect(board({ trackingKind: 'not_found', tracking: null, trackingTitle: 'Not found' }).text()).toContain('Not found')
    expect(board({ locationsKind: 'unavailable', locations: [], locationsTitle: 'Unavailable' }).text()).toContain('Unavailable')
  })
})

describe('shipper tracking request contract', () => {
  it('uses only the safe routes, company scope, and bounded history', async () => {
    const calls: string[] = []
    const client = new PortalClient({
      baseUrl: 'http://gateway.test',
      getAccessToken: () => 'token',
      getAllowedCompanyIds: () => ['co-shipper'],
      fetchImpl: async (input, init) => {
        const url = new URL(String(input))
        const headers = new Headers(init?.headers)
        calls.push(`${url.pathname}${url.search}`)
        expect(url.pathname.startsWith('/api/v1/shipper/shipments/shp-1')).toBe(true)
        expect(url.pathname === '/api/v1/shipments' || url.pathname.startsWith('/api/v1/shipments/')).toBe(false)
        expect(url.searchParams.get('shipper_company_id')).toBe('co-shipper')
        expect(headers.get('x-company-id')).toBe('co-shipper')
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

    for (const suffix of SHIPPER_TRACKING_SUFFIXES) {
      const history = suffix.endsWith('locations')
        ? { limit: 500, offset: -1, from: '2026-12-01T00:00:00Z' }
        : suffix.endsWith('eta/history')
          ? { targetType: 'pickup' as const, limit: 80, offset: 0 }
          : suffix.endsWith('slots/history')
            ? { slotType: 'delivery' as const, limit: 50, offset: 0 }
            : undefined
      const spec = shipperReadSpec('shp-1', suffix, 'co-shipper', history)
      await client.request(spec.path, { companyId: 'co-shipper', query: spec.query })
    }

    expect(calls).toHaveLength(SHIPPER_TRACKING_SUFFIXES.length)
    expect(calls.some((call) => call.includes('/tracking/locations') && call.includes('limit=50') && call.includes('offset=0'))).toBe(true)
    expect(calls.some((call) => call.includes('/eta/history') && call.includes('targetType=pickup'))).toBe(true)
    expect(calls.some((call) => call.includes('/slots/history') && call.includes('slotType=delivery'))).toBe(true)
    expect(calls.some((call) => call.includes('limit=500') || call.includes('limit=80'))).toBe(false)
  })

  it('rejects business-fact query keys and omits provider fields from the client source', () => {
    expect(() => buildHistoryQuery({ plannedPickupAt: '2026-12-01T08:00:00Z' } as never)).toThrow(/business fact/)
    expect(() => buildHistoryQuery({ pickupEstimatedArrivalAt: '2026-12-01T11:00:00Z' } as never)).toThrow(/business fact/)

    const files = [
      'domain/tracking.ts',
      'composables/useShipperApi.ts',
      'components/tracking/TrackingBoard.vue',
      'components/tracking/EtaFacts.vue',
      'components/tracking/SlotFacts.vue',
      'components/tracking/EtaHistoryTable.vue',
      'components/tracking/SlotHistoryTable.vue',
      'pages/shipments/[id]/tracking.vue',
    ]
    for (const file of files) {
      const source = readFileSync(join(process.cwd(), file), 'utf8')
      expect(source.includes('/api/v1/shipments')).toBe(false)
      for (const leak of LEAKS) expect(source).not.toContain(leak)
      expect(source).not.toMatch(/\b(POST|PUT|PATCH|DELETE)\b/)
    }
  })
})
