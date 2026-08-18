import type { Company, CompanyListFilters } from '~/types/company'
import type { PaginatedResponse } from '~/types/api'
import type { UserCompanyMembership } from '~/types/company'
import { ApiError } from '~/utils/apiClient'

export function useCompanies() {
  const tenantStore = useTenantStore()
  const { apiGet } = useApi()

  function tenantId() {
    return tenantStore.tenantId
  }

  async function listCompanies(filters: CompanyListFilters = {}) {
    const query: Record<string, string | number | undefined> = {
      tenant_id: tenantId(),
      limit: filters.limit ?? 100,
      offset: filters.offset ?? 0,
    }
    if (filters.search?.trim()) query.search = filters.search.trim()
    if (filters.company_type) query.company_type = filters.company_type
    if (filters.status) query.status = filters.status

    const data = await apiGet<PaginatedResponse<Company>>('/api/v1/companies', { query })
    return { ...data, items: data.items ?? [] }
  }

  async function getUserCompanies(userId: string) {
    const data = await apiGet<{ items: UserCompanyMembership[] }>(`/api/v1/users/${userId}/companies`, {
      query: { tenant_id: tenantId() },
    })
    return data.items ?? []
  }

  function isApiUnavailableError(error: unknown): boolean {
    if (error instanceof ApiError) {
      return error.status === 0 || error.status >= 500 || error.code === 'SERVICE_UNAVAILABLE'
    }
    return error instanceof TypeError
  }

  return {
    listCompanies,
    getUserCompanies,
    isApiUnavailableError,
  }
}
