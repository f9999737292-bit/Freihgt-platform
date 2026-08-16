import type { ShipmentLocationHistoryResponse, ShipmentTrackingSummary } from '~/types/tracking'

export function useTrackingApi() {
  const { apiGet } = useApi()

  async function getShipmentTracking(shipmentId: string) {
    return apiGet<ShipmentTrackingSummary>(`/api/v1/shipments/${shipmentId}/tracking`)
  }

  async function listShipmentLocations(
    shipmentId: string,
    params: { limit?: number; offset?: number; from?: string; to?: string } = {},
  ) {
    return apiGet<ShipmentLocationHistoryResponse>(`/api/v1/shipments/${shipmentId}/tracking/locations`, {
      query: params,
    })
  }

  return {
    getShipmentTracking,
    listShipmentLocations,
  }
}

export function formatTrackingAge(seconds?: number): string | null {
  if (seconds == null || seconds < 0) return null
  if (seconds < 60) return `${seconds}s`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m`
  const hours = Math.floor(minutes / 60)
  return `${hours}h`
}
