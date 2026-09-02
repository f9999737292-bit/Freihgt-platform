import { describe, expect, it, vi } from 'vitest'
import {
  shipmentDetailRoute,
  shipmentEventHistoryRoute,
  shipmentsListRoute,
} from '../utils/shipmentDetailNavigation'

const R31_SHIPMENT_ID = '0b9fe8d5-d20e-4a81-b591-c0df9812fc95'

describe('shipment event history navigation routes', () => {
  it('builds shipment-specific event history route', () => {
    expect(shipmentEventHistoryRoute(R31_SHIPMENT_ID)).toBe(
      `/shipments/${R31_SHIPMENT_ID}/events`,
    )
  })

  it('preserves shipment id in event history route', () => {
    const route = shipmentEventHistoryRoute(R31_SHIPMENT_ID)
    expect(route).toContain(R31_SHIPMENT_ID)
    expect(route.endsWith('/events')).toBe(true)
  })

  it('builds shipment detail route for back navigation', () => {
    expect(shipmentDetailRoute(R31_SHIPMENT_ID)).toBe(`/shipments/${R31_SHIPMENT_ID}`)
  })

  it('builds shipments list route', () => {
    expect(shipmentsListRoute()).toBe('/shipments')
  })
})

describe('shipment detail navigation handlers', () => {
  it('navigates to event history with preserved shipment id', () => {
    const push = vi.fn()
    const shipmentId = R31_SHIPMENT_ID

    push(shipmentEventHistoryRoute(shipmentId))

    expect(push).toHaveBeenCalledWith(`/shipments/${shipmentId}/events`)
  })

  it('navigates back to shipments list without mutating shipment id routes', () => {
    const push = vi.fn()

    push(shipmentsListRoute())

    expect(push).toHaveBeenCalledWith('/shipments')
    expect(push).not.toHaveBeenCalledWith(expect.stringContaining('/events'))
  })

  it('navigates from event history back to shipment detail', () => {
    const push = vi.fn()
    const shipmentId = R31_SHIPMENT_ID

    push(shipmentDetailRoute(shipmentId))

    expect(push).toHaveBeenCalledWith(`/shipments/${shipmentId}`)
  })

  it('keeps document action routes separate from event history navigation', () => {
    const push = vi.fn()
    const shipmentId = R31_SHIPMENT_ID

    push(`/documents?shipment_id=${shipmentId}&document_type=POD`)
    push(shipmentEventHistoryRoute(shipmentId))

    expect(push).toHaveBeenNthCalledWith(
      1,
      `/documents?shipment_id=${shipmentId}&document_type=POD`,
    )
    expect(push).toHaveBeenNthCalledWith(2, `/shipments/${shipmentId}/events`)
  })
})

describe('shipment event history control contract', () => {
  it('uses event history i18n key title string in product copy map', async () => {
    const ru = await import('../i18n/ru-RU.json')
    expect(ru.default.shipmentEvents.title).toBe('История событий')
  })
})
