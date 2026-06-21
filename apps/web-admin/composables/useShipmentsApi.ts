import type { PaginatedResponse } from '~/types/api'
import type {
  AcceptShipmentPayload,
  AssignDriverPayload,
  AssignVehiclePayload,
  CancelShipmentPayload,
  CreateShipmentFromBidPayload,
  CreateShipmentFromOrderPayload,
  ListShipmentsFilters,
  Shipment,
  UpdateShipmentStatusPayload,
} from '~/types/shipment'
import { ApiError } from '~/composables/useApi'

export function useShipmentsApi() {
  const tenantStore = useTenantStore()
  const { apiGet, apiPost, apiPatch } = useApi()

  function tenantId() {
    return tenantStore.tenantId
  }

  function tenantQuery(extra: Record<string, string | number | undefined> = {}) {
    return { tenant_id: tenantId(), ...extra }
  }

  async function listShipments(params: ListShipmentsFilters = {}) {
    const query: Record<string, string | number | undefined> = {
      ...tenantQuery(),
      limit: params.limit ?? 20,
      offset: params.offset ?? 0,
    }
    if (params.status) query.status = params.status
    if (params.shipper_company_id) query.shipper_company_id = params.shipper_company_id
    if (params.consignee_company_id) query.consignee_company_id = params.consignee_company_id
    if (params.carrier_company_id) query.carrier_company_id = params.carrier_company_id
    // TODO: backend search/shipment_number filter not implemented yet
    if (params.search?.trim()) query.search = params.search.trim()

    const data = await apiGet<PaginatedResponse<Shipment>>('/api/v1/shipments', { query })
    return { ...data, items: data.items ?? [] }
  }

  async function getShipment(id: string) {
    return apiGet<Shipment>(`/api/v1/shipments/${id}`, { query: tenantQuery() })
  }

  async function createShipmentFromTransportOrder(
    payload: Omit<CreateShipmentFromOrderPayload, 'tenant_id'>,
  ) {
    return apiPost<Shipment>('/api/v1/shipments/from-transport-order', {
      ...payload,
      tenant_id: tenantId(),
      forwarder_company_id: payload.forwarder_company_id?.trim() || null,
      planned_pickup_at: payload.planned_pickup_at || undefined,
      planned_delivery_at: payload.planned_delivery_at || undefined,
    })
  }

  async function createShipmentFromBid(payload: Omit<CreateShipmentFromBidPayload, 'tenant_id'>) {
    return apiPost<Shipment>('/api/v1/shipments/from-bid', {
      ...payload,
      tenant_id: tenantId(),
      planned_pickup_at: payload.planned_pickup_at || undefined,
      planned_delivery_at: payload.planned_delivery_at || undefined,
    })
  }

  async function assignDriver(id: string, payload: Omit<AssignDriverPayload, 'tenant_id'>) {
    return apiPost<Shipment>(`/api/v1/shipments/${id}/assign-driver`, {
      ...payload,
      tenant_id: tenantId(),
    })
  }

  async function assignVehicle(id: string, payload: Omit<AssignVehiclePayload, 'tenant_id'>) {
    return apiPost<Shipment>(`/api/v1/shipments/${id}/assign-vehicle`, {
      ...payload,
      tenant_id: tenantId(),
    })
  }

  async function acceptShipment(id: string, payload: Omit<AcceptShipmentPayload, 'tenant_id'> = {}) {
    return apiPost<Shipment>(`/api/v1/shipments/${id}/accept`, {
      ...payload,
      tenant_id: tenantId(),
    })
  }

  async function updateShipmentStatus(
    id: string,
    payload: Omit<UpdateShipmentStatusPayload, 'tenant_id'>,
  ) {
    return apiPatch<Shipment>(`/api/v1/shipments/${id}/status`, {
      ...payload,
      tenant_id: tenantId(),
      actual_time: payload.actual_time || new Date().toISOString(),
    })
  }

  async function cancelShipment(id: string, payload: Omit<CancelShipmentPayload, 'tenant_id'>) {
    return apiPost<Shipment>(`/api/v1/shipments/${id}/cancel`, {
      ...payload,
      tenant_id: tenantId(),
    })
  }

  function isApiUnavailableError(error: unknown): boolean {
    if (error instanceof ApiError) {
      return error.status === 0 || error.status >= 500 || error.code === 'SERVICE_UNAVAILABLE'
    }
    return error instanceof TypeError
  }

  return {
    listShipments,
    getShipment,
    createShipmentFromTransportOrder,
    createShipmentFromBid,
    assignDriver,
    assignVehicle,
    acceptShipment,
    updateShipmentStatus,
    cancelShipment,
    isApiUnavailableError,
  }
}
