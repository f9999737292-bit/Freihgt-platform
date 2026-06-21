import type { PaginatedResponse } from '~/types/api'
import type { CreateDriverPayload, Driver, ListDriversFilters } from '~/types/shipment'
import { ApiError } from '~/composables/useApi'

export function useDriversApi() {
  const tenantStore = useTenantStore()
  const { apiGet, apiPost } = useApi()

  function tenantId() {
    return tenantStore.tenantId
  }

  function tenantQuery(extra: Record<string, string | number | undefined> = {}) {
    return { tenant_id: tenantId(), ...extra }
  }

  async function listDrivers(params: ListDriversFilters = {}) {
    const query: Record<string, string | number | undefined> = {
      ...tenantQuery(),
      limit: params.limit ?? 100,
      offset: params.offset ?? 0,
    }
    if (params.carrier_company_id) query.carrier_company_id = params.carrier_company_id
    if (params.status) query.status = params.status

    const data = await apiGet<PaginatedResponse<Driver>>('/api/v1/drivers', { query })
    return { ...data, items: data.items ?? [] }
  }

  async function getDriver(id: string) {
    return apiGet<Driver>(`/api/v1/drivers/${id}`, { query: tenantQuery() })
  }

  async function createDriver(payload: Omit<CreateDriverPayload, 'tenant_id'>) {
    return apiPost<Driver>('/api/v1/drivers', {
      ...payload,
      tenant_id: tenantId(),
      user_id: payload.user_id || null,
      phone: payload.phone?.trim() || undefined,
      license_number: payload.license_number?.trim() || undefined,
      license_country: payload.license_country.toUpperCase(),
    })
  }

  function isApiUnavailableError(error: unknown): boolean {
    if (error instanceof ApiError) {
      return error.status === 0 || error.status >= 500 || error.code === 'SERVICE_UNAVAILABLE'
    }
    return error instanceof TypeError
  }

  return {
    listDrivers,
    getDriver,
    createDriver,
    isApiUnavailableError,
  }
}
