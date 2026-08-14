import type { ETAHistoryResponse, ShipmentETAContext, ShipmentETASummary } from '~/types/eta'

export function formatETAAge(seconds?: number | null): string | null {
  if (seconds == null || seconds < 0) return null
  if (seconds < 60) return `${seconds}s`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m`
  const hours = Math.floor(minutes / 60)
  const rem = minutes % 60
  return rem > 0 ? `${hours}h ${rem}m` : `${hours}h`
}

export function formatDeviationMinutes(seconds?: number | null): string | null {
  if (seconds == null) return null
  const minutes = Math.round(Math.abs(seconds) / 60)
  if (minutes === 0) return '0'
  return String(minutes)
}

function buildPlannedQuery(context?: ShipmentETAContext): string {
  if (!context) return ''
  const params = new URLSearchParams()
  if (context.plannedPickupAt) params.set('plannedPickupAt', context.plannedPickupAt)
  if (context.plannedDeliveryAt) params.set('plannedDeliveryAt', context.plannedDeliveryAt)
  if (context.actualPickupAt) params.set('actualPickupAt', context.actualPickupAt)
  if (context.actualDeliveryAt) params.set('actualDeliveryAt', context.actualDeliveryAt)
  if (context.shipmentStatus) params.set('shipmentStatus', context.shipmentStatus)
  return params.toString()
}

export function useEtaApi() {
  const { apiGet } = useApi()

  async function getShipmentETA(shipmentId: string, context?: ShipmentETAContext) {
    const query = buildPlannedQuery(context)
    const path = query
      ? `/api/v1/shipments/${shipmentId}/eta?${query}`
      : `/api/v1/shipments/${shipmentId}/eta`
    return apiGet<ShipmentETASummary>(path)
  }

  async function listETAHistory(
    shipmentId: string,
    options?: { targetType?: string; from?: string; to?: string; limit?: number; offset?: number },
  ) {
    const params = new URLSearchParams()
    if (options?.targetType) params.set('targetType', options.targetType)
    if (options?.from) params.set('from', options.from)
    if (options?.to) params.set('to', options.to)
    if (options?.limit != null) params.set('limit', String(options.limit))
    if (options?.offset != null) params.set('offset', String(options.offset))
    const query = params.toString()
    const path = query
      ? `/api/v1/shipments/${shipmentId}/eta/history?${query}`
      : `/api/v1/shipments/${shipmentId}/eta/history`
    return apiGet<ETAHistoryResponse>(path)
  }

  return {
    getShipmentETA,
    listETAHistory,
    formatETAAge,
    formatDeviationMinutes,
  }
}
