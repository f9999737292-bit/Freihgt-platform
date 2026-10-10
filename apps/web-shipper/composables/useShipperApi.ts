import type { ShipperShipment } from '../domain/shipment'
import {
  shipperReadSpec,
  type ETAHistoryItem,
  type ETASummary,
  type HistoryPage,
  type HistoryQuery,
  type LocationFact,
  type ShipperTrackingSuffix,
  type SlotHistoryItem,
  type SlotSummary,
  type TrackingSummary,
} from '../domain/tracking'

export function useShipperApi() {
  const office = useShipperOffice()

  function scoped(extra: Record<string, string | undefined> = {}) {
    const companyId = office.requireCompany()
    return {
      companyId,
      query: { shipper_company_id: companyId, ...extra },
    }
  }

  function read<T>(shipmentId: string, suffix: '' | ShipperTrackingSuffix, history?: HistoryQuery) {
    const companyId = office.requireCompany()
    const spec = shipperReadSpec(shipmentId, suffix, companyId, history)
    return office.client.request<T>(spec.path, { companyId, query: spec.query })
  }

  function listShipments(status?: string) {
    const filter = status?.trim()
    return office.client.request<{ items?: ShipperShipment[]; total?: number }>(
      '/api/v1/shipper/shipments',
      scoped(filter ? { status: filter } : {}),
    )
  }

  function getShipment(id: string) {
    return read<ShipperShipment>(id, '')
  }

  function getTracking(shipmentId: string) {
    return read<TrackingSummary>(shipmentId, '/tracking')
  }

  function listTrackingLocations(shipmentId: string, query: HistoryQuery = {}) {
    return read<HistoryPage<LocationFact>>(shipmentId, '/tracking/locations', query)
  }

  function getETA(shipmentId: string) {
    return read<ETASummary>(shipmentId, '/eta')
  }

  function listETAHistory(shipmentId: string, query: HistoryQuery & { targetType: 'pickup' | 'delivery' }) {
    return read<HistoryPage<ETAHistoryItem>>(shipmentId, '/eta/history', query)
  }

  function getSlots(shipmentId: string) {
    return read<SlotSummary>(shipmentId, '/slots')
  }

  function listSlotHistory(shipmentId: string, query: HistoryQuery & { slotType: 'pickup' | 'delivery' }) {
    return read<HistoryPage<SlotHistoryItem>>(shipmentId, '/slots/history', query)
  }

  return {
    listShipments,
    getShipment,
    getTracking,
    listTrackingLocations,
    getETA,
    listETAHistory,
    getSlots,
    listSlotHistory,
  }
}
