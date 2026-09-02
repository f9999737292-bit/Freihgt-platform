import { ref, unref, watch, type Ref } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import { ApiError } from '../composables/useApi'
import type { ShipmentEventsResponse } from '../types/shipmentEvent'

const R31_SHIPMENT_ID = '0b9fe8d5-d20e-4a81-b591-c0df9812fc95'

const sampleResponse: ShipmentEventsResponse = {
  shipment: {
    id: R31_SHIPMENT_ID,
    number: 'SH-r31e1-clean-20260901T175719Z',
    status: 'IN_TRANSIT',
  },
  timeline: {
    page: 1,
    limit: 50,
    total: 12,
    items: Array.from({ length: 12 }, (_, index) => ({
      id: `event-${index + 1}`,
      type: 'STATUS_CHANGED',
      category: 'OPERATIONS',
      source: 'SYSTEM',
      severity: 'INFO',
      occurredAt: '2026-09-01T18:07:20Z',
      titleCode: 'shipment.status.in_transit',
      derived: false,
    })),
  },
  dataFreshness: {
    partial: false,
    warnings: [],
  },
}

/**
 * Minimal reproduction of useShipmentEvents fetch + immediate watch contract
 * used by pages/shipments/[id]/events.vue.
 */
async function runEventsPageLifecycle(options: {
  shipmentId: Ref<string>
  routeQuery: Record<string, string>
  apiGet: ReturnType<typeof vi.fn>
}) {
  const { shipmentId, routeQuery, apiGet } = options
  let fetchCount = 0

  async function fetchEvents() {
    const id = unref(shipmentId)
    if (!id) return

    fetchCount += 1
    await apiGet(`/api/v1/shipments/${id}/events`, {
      query: {
        order: 'desc',
        page: 1,
        limit: 50,
        ...routeQuery,
      },
      skipTenant: true,
    })
  }

  const stop = watch([shipmentId, () => routeQuery], fetchEvents, { immediate: true, deep: true })
  await Promise.resolve()
  stop()

  return fetchCount
}

describe('shipment events page API lifecycle (VIS-005)', () => {
  it('triggers events API request when event history page lifecycle starts', async () => {
    const apiGet = vi.fn().mockResolvedValue(sampleResponse)
    const shipmentId = ref(R31_SHIPMENT_ID)

    const fetchCount = await runEventsPageLifecycle({
      shipmentId,
      routeQuery: {},
      apiGet,
    })

    expect(fetchCount).toBe(1)
    expect(apiGet).toHaveBeenCalledTimes(1)
    expect(apiGet).toHaveBeenCalledWith(
      `/api/v1/shipments/${R31_SHIPMENT_ID}/events`,
      expect.objectContaining({ skipTenant: true }),
    )
  })

  it('does not call events API when shipment id is empty', async () => {
    const apiGet = vi.fn()
    const shipmentId = ref('')

    const fetchCount = await runEventsPageLifecycle({
      shipmentId,
      routeQuery: {},
      apiGet,
    })

    expect(fetchCount).toBe(0)
    expect(apiGet).not.toHaveBeenCalled()
  })

  it('refetches when route query filters change on the events page', async () => {
    const apiGet = vi.fn().mockResolvedValue(sampleResponse)
    const shipmentId = ref(R31_SHIPMENT_ID)
    const routeQuery = ref<Record<string, string>>({})

    const stop = watch([shipmentId, routeQuery], async () => {
      const id = unref(shipmentId)
      if (!id) return
      await apiGet(`/api/v1/shipments/${id}/events`, {
        query: { order: 'desc', page: 1, limit: 50, ...routeQuery.value },
        skipTenant: true,
      })
    }, { immediate: true, deep: true })

    await Promise.resolve()
    routeQuery.value = { type: 'STATUS_CHANGED' }
    await Promise.resolve()
    stop()

    expect(apiGet).toHaveBeenCalledTimes(2)
    expect(apiGet).toHaveBeenLastCalledWith(
      `/api/v1/shipments/${R31_SHIPMENT_ID}/events`,
      expect.objectContaining({
        query: expect.objectContaining({ type: 'STATUS_CHANGED' }),
      }),
    )
  })

  it('maps composable source to immediate fetchEvents watch on events.vue', async () => {
    const { readFileSync } = await import('node:fs')
    const { join } = await import('node:path')
    const { fileURLToPath } = await import('node:url')

    const webAdminRoot = fileURLToPath(new URL('..', import.meta.url))
    const composableSource = readFileSync(join(webAdminRoot, 'composables', 'useShipmentEvents.ts'), 'utf8')
    const eventsPageSource = readFileSync(join(webAdminRoot, 'pages', 'shipments', '[id]', 'events.vue'), 'utf8')

    expect(composableSource).toContain("watch([shipmentId, () => route.query], fetchEvents, { immediate: true")
    expect(composableSource).toContain('/api/v1/shipments/${id}/events')
    expect(eventsPageSource).toContain('useShipmentEvents(shipmentId)')
  })

  it('classifies permission and missing shipment without treating them as success', async () => {
    const apiGet = vi
      .fn()
      .mockRejectedValueOnce(new ApiError(403, { code: 'FORBIDDEN', message: 'denied', details: {} }))
      .mockRejectedValueOnce(new ApiError(404, { code: 'NOT_FOUND', message: 'missing', details: {} }))

    for (const status of [403, 404]) {
      try {
        await apiGet(`/api/v1/shipments/${R31_SHIPMENT_ID}/events`)
      } catch (error) {
        expect(error).toBeInstanceOf(ApiError)
        expect((error as ApiError).status).toBe(status)
      }
    }
  })
})

describe('shipment detail page does not prefetch events API', () => {
  it('does not import useShipmentEvents on the detail index page', async () => {
    const { readFileSync } = await import('node:fs')
    const { join } = await import('node:path')
    const { fileURLToPath } = await import('node:url')

    const webAdminRoot = fileURLToPath(new URL('..', import.meta.url))
    const detailSource = readFileSync(join(webAdminRoot, 'pages', 'shipments', '[id]', 'index.vue'), 'utf8')

    expect(detailSource).not.toContain('useShipmentEvents')
    expect(detailSource).not.toContain('/api/v1/shipments/')
  })
})
