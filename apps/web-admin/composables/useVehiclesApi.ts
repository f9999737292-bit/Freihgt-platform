import type { PaginatedResponse } from '~/types/api'
import type { CreateVehiclePayload, ListVehiclesFilters, Vehicle } from '~/types/shipment'
import { ApiError } from '~/composables/useApi'

export function useVehiclesApi() {
  const tenantStore = useTenantStore()
  const { apiGet, apiPost } = useApi()

  function tenantId() {
    return tenantStore.tenantId
  }

  function tenantQuery(extra: Record<string, string | number | undefined> = {}) {
    return { tenant_id: tenantId(), ...extra }
  }

  async function listVehicles(params: ListVehiclesFilters = {}) {
    const query: Record<string, string | number | undefined> = {
      ...tenantQuery(),
      limit: params.limit ?? 100,
      offset: params.offset ?? 0,
    }
    if (params.carrier_company_id) query.carrier_company_id = params.carrier_company_id
    if (params.status) query.status = params.status

    const data = await apiGet<PaginatedResponse<Vehicle>>('/api/v1/vehicles', { query })
    return { ...data, items: data.items ?? [] }
  }

  async function getVehicle(id: string) {
    return apiGet<Vehicle>(`/api/v1/vehicles/${id}`, { query: tenantQuery() })
  }

  async function createVehicle(payload: Omit<CreateVehiclePayload, 'tenant_id'>) {
    return apiPost<Vehicle>('/api/v1/vehicles', {
      ...payload,
      tenant_id: tenantId(),
      equipment_type: payload.equipment_type?.trim() || undefined,
      capacity_weight: payload.capacity_weight ?? undefined,
      capacity_volume: payload.capacity_volume ?? undefined,
      registration_country: payload.registration_country.toUpperCase(),
    })
  }

  function isApiUnavailableError(error: unknown): boolean {
    if (error instanceof ApiError) {
      return error.status === 0 || error.status >= 500 || error.code === 'SERVICE_UNAVAILABLE'
    }
    return error instanceof TypeError
  }

  return {
    listVehicles,
    getVehicle,
    createVehicle,
    isApiUnavailableError,
  }
}
