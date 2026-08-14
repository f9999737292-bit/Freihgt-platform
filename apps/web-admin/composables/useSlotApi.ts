import type { ShipmentSlotQueryContext, ShipmentSlotSummary, SlotHistoryResponse } from '~/types/slot'

export function formatSlotWindow(start?: string | null, end?: string | null): string | null {
  if (!start || !end) return null
  const fmt = (iso: string) =>
    new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  return `${fmt(start)}–${fmt(end)}`
}

function buildSlotQuery(context?: ShipmentSlotQueryContext): string {
  if (!context) return ''
  const params = new URLSearchParams()
  if (context.shipmentStatus) params.set('shipmentStatus', context.shipmentStatus)
  if (context.actualPickupAt) params.set('actualPickupAt', context.actualPickupAt)
  if (context.actualDeliveryAt) params.set('actualDeliveryAt', context.actualDeliveryAt)
  if (context.deliveryEtaStatus) params.set('deliveryEtaStatus', context.deliveryEtaStatus)
  if (context.deliveryEtaFreshness) params.set('deliveryEtaFreshness', context.deliveryEtaFreshness)
  if (context.deliveryEtaQuality) params.set('deliveryEtaQuality', context.deliveryEtaQuality)
  if (context.deliveryEstimatedArrivalAt) {
    params.set('deliveryEstimatedArrivalAt', context.deliveryEstimatedArrivalAt)
  }
  return params.toString()
}

export function useSlotApi() {
  const { apiGet } = useApi()

  async function getShipmentSlots(shipmentId: string, context?: ShipmentSlotQueryContext) {
    const query = buildSlotQuery(context)
    const path = query
      ? `/api/v1/shipments/${shipmentId}/slots?${query}`
      : `/api/v1/shipments/${shipmentId}/slots`
    return apiGet<ShipmentSlotSummary>(path)
  }

  async function listSlotHistory(
    shipmentId: string,
    options?: { slotType?: string; from?: string; to?: string; limit?: number; offset?: number },
  ) {
    const params = new URLSearchParams()
    if (options?.slotType) params.set('slotType', options.slotType)
    if (options?.from) params.set('from', options.from)
    if (options?.to) params.set('to', options.to)
    if (options?.limit != null) params.set('limit', String(options.limit))
    if (options?.offset != null) params.set('offset', String(options.offset))
    const query = params.toString()
    const path = query
      ? `/api/v1/shipments/${shipmentId}/slots/history?${query}`
      : `/api/v1/shipments/${shipmentId}/slots/history`
    return apiGet<SlotHistoryResponse>(path)
  }

  return { getShipmentSlots, listSlotHistory, formatSlotWindow }
}
