import type { ShipperShipment } from '../domain/shipment'

export function useShipperApi() {
  const office = useShipperOffice()

  function scoped(extra: Record<string, string | undefined> = {}) {
    const companyId = office.requireCompany()
    return {
      companyId,
      query: { shipper_company_id: companyId, ...extra },
    }
  }

  function listShipments(status?: string) {
    const filter = status?.trim()
    return office.client.request<{ items?: ShipperShipment[]; total?: number }>(
      '/api/v1/shipper/shipments',
      scoped(filter ? { status: filter } : {}),
    )
  }

  function getShipment(id: string) {
    return office.client.request<ShipperShipment>(
      `/api/v1/shipper/shipments/${encodeURIComponent(id)}`,
      scoped(),
    )
  }

  return {
    listShipments,
    getShipment,
  }
}
