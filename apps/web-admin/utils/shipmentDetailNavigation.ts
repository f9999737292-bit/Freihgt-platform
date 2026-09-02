/** Shipment detail ↔ event history navigation targets (web-admin). */
export function shipmentEventHistoryRoute(shipmentId: string): string {
  const id = shipmentId.trim()
  if (!id) {
    throw new Error('shipmentId is required')
  }
  return `/shipments/${id}/events`
}

export function shipmentDetailRoute(shipmentId: string): string {
  const id = shipmentId.trim()
  if (!id) {
    throw new Error('shipmentId is required')
  }
  return `/shipments/${id}`
}

export function shipmentsListRoute(): string {
  return '/shipments'
}
