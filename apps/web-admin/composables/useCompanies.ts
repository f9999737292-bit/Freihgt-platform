import type {
  Company,
  CompanyListFilters,
  CreateCompanyPayload,
  UpdateCompanyPayload,
} from '~/types/company'
import type { PaginatedResponse } from '~/types/api'
import { ApiError } from '~/composables/useApi'

export function useCompanies() {
  const tenantStore = useTenantStore()
  const { apiGet, apiPost, apiPatch } = useApi()

  function tenantId() {
    return tenantStore.tenantId
  }

  async function listCompanies(filters: CompanyListFilters = {}) {
    const query: Record<string, string | number | undefined> = {
      tenant_id: tenantId(),
      limit: filters.limit ?? 20,
      offset: filters.offset ?? 0,
    }
    if (filters.search?.trim()) query.search = filters.search.trim()
    if (filters.company_type) query.company_type = filters.company_type
    if (filters.status) query.status = filters.status

    const data = await apiGet<PaginatedResponse<Company>>('/api/v1/companies', { query })

    let items = data.items ?? []
    if (filters.country_code) {
      const code = filters.country_code.toUpperCase()
      items = items.filter((item) => item.country_code?.toUpperCase() === code)
    }

    return { ...data, items }
  }

  async function getCompany(id: string) {
    return apiGet<Company>(`/api/v1/companies/${id}`)
  }

  async function createCompany(payload: CreateCompanyPayload) {
    return apiPost<Company>('/api/v1/companies', {
      ...payload,
      tenant_id: tenantId(),
      country_code: payload.country_code.toUpperCase(),
      short_name: payload.short_name || undefined,
      legal_name_en: payload.legal_name_en || undefined,
      legal_name_zh: payload.legal_name_zh || undefined,
      tax_id: payload.tax_id || undefined,
      registration_number: payload.registration_number || undefined,
    })
  }

  async function updateCompany(id: string, payload: UpdateCompanyPayload) {
    return apiPatch<Company>(`/api/v1/companies/${id}`, {
      ...payload,
      country_code: payload.country_code?.toUpperCase(),
      short_name: payload.short_name || undefined,
      legal_name_en: payload.legal_name_en || undefined,
      legal_name_zh: payload.legal_name_zh || undefined,
      tax_id: payload.tax_id || undefined,
      registration_number: payload.registration_number || undefined,
    })
  }

  function isApiUnavailableError(error: unknown): boolean {
    if (error instanceof ApiError) {
      return error.status === 0 || error.status >= 500 || error.code === 'SERVICE_UNAVAILABLE'
    }
    return error instanceof TypeError
  }

  return {
    listCompanies,
    getCompany,
    createCompany,
    updateCompany,
    isApiUnavailableError,
  }
}
