import { existsSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

const R31_SHIPMENT_ID = '0b9fe8d5-d20e-4a81-b591-c0df9812fc95'
const WEB_ADMIN_ROOT = fileURLToPath(new URL('..', import.meta.url))
const PAGES_ROOT = join(WEB_ADMIN_ROOT, 'pages', 'shipments')

const ShipmentDetailStub = { __page: 'ShipmentDetailPage' as const }
const ShipmentEventsStub = { __page: 'ShipmentEventsPage' as const }

/** Mirrors Nuxt file-based sibling routes after DETAIL_TO_INDEX fix. */
function createFixedShipmentRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      {
        path: '/shipments/:id',
        name: 'shipments-id',
        component: ShipmentDetailStub,
      },
      {
        path: '/shipments/:id/events',
        name: 'shipments-id-events',
        component: ShipmentEventsStub,
      },
    ],
  })
}

/** Regression model of the pre-fix nested parent without <NuxtPage />. */
function createBrokenNestedShipmentRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      {
        path: '/shipments/:id',
        component: ShipmentDetailStub,
        children: [
          {
            path: 'events',
            name: 'shipments-id-events-nested',
            component: ShipmentEventsStub,
          },
        ],
      },
    ],
  })
}

describe('shipment pages route tree (VIS-005)', () => {
  it('uses index.vue for detail and keeps events.vue as a sibling route file', () => {
    expect(existsSync(join(PAGES_ROOT, '[id]', 'index.vue'))).toBe(true)
    expect(existsSync(join(PAGES_ROOT, '[id]', 'events.vue'))).toBe(true)
    expect(existsSync(join(PAGES_ROOT, '[id].vue'))).toBe(false)
  })

  it('maps detail and events pages to distinct route components', async () => {
    const router = createFixedShipmentRouter()

    await router.push(`/shipments/${R31_SHIPMENT_ID}`)
    expect(router.currentRoute.value.path).toBe(`/shipments/${R31_SHIPMENT_ID}`)
    expect(router.currentRoute.value.matched.at(-1)?.components?.default).toBe(ShipmentDetailStub)

    await router.push(`/shipments/${R31_SHIPMENT_ID}/events`)
    expect(router.currentRoute.value.path).toBe(`/shipments/${R31_SHIPMENT_ID}/events`)
    expect(router.currentRoute.value.matched.at(-1)?.components?.default).toBe(ShipmentEventsStub)
    expect(router.currentRoute.value.matched.at(-1)?.components?.default).not.toBe(ShipmentDetailStub)
  })

  it('preserves shipment id when navigating detail → events → detail', async () => {
    const router = createFixedShipmentRouter()

    await router.push(`/shipments/${R31_SHIPMENT_ID}`)
    await router.push(`/shipments/${R31_SHIPMENT_ID}/events`)
    expect(String(router.currentRoute.value.params.id)).toBe(R31_SHIPMENT_ID)

    await router.push(`/shipments/${R31_SHIPMENT_ID}`)
    expect(String(router.currentRoute.value.params.id)).toBe(R31_SHIPMENT_ID)
    expect(router.currentRoute.value.matched.at(-1)?.components?.default).toBe(ShipmentDetailStub)
  })

  it('documents nested parent-child routing as the confirmed defect model', async () => {
    const broken = createBrokenNestedShipmentRouter()
    await broken.push(`/shipments/${R31_SHIPMENT_ID}/events`)

    expect(broken.currentRoute.value.matched.length).toBeGreaterThan(1)
    expect(broken.currentRoute.value.matched[0]?.components?.default).toBe(ShipmentDetailStub)
    expect(broken.currentRoute.value.matched.at(-1)?.components?.default).toBe(ShipmentEventsStub)
  })

  it('renders shipment detail markers only on index.vue', () => {
    const detailSource = readFileSync(join(PAGES_ROOT, '[id]', 'index.vue'), 'utf8')
    const eventsSource = readFileSync(join(PAGES_ROOT, '[id]', 'events.vue'), 'utf8')

    expect(detailSource).toContain('ShipmentsShipmentDetailsCard')
    expect(detailSource).toContain('data-testid="shipment-event-history"')
    expect(detailSource).not.toContain('ShipmentEventsShipmentTimeline')

    expect(eventsSource).toContain('ShipmentEventsShipmentTimeline')
    expect(eventsSource).toContain('data-testid="event-history-back-to-shipment"')
    expect(eventsSource).not.toContain('ShipmentsShipmentDetailsCard')
  })

  it('wires event history navigation handlers to sibling routes', () => {
    const detailSource = readFileSync(join(PAGES_ROOT, '[id]', 'index.vue'), 'utf8')
    const eventsSource = readFileSync(join(PAGES_ROOT, '[id]', 'events.vue'), 'utf8')

    expect(detailSource).toContain('shipmentEventHistoryRoute')
    expect(detailSource).toContain('goToEventHistory')
    expect(eventsSource).toContain('shipmentDetailRoute')
    expect(eventsSource).toContain('goToShipmentDetail')
    expect(eventsSource).toContain('useShipmentEvents(shipmentId)')
  })
})
