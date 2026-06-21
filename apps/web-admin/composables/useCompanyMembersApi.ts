import type { AddCompanyMemberPayload, CompanyMember } from '~/types/company'
import type { PaginatedResponse } from '~/types/api'
import { ApiError } from '~/composables/useApi'

export interface ListCompanyMembersParams {
  limit?: number
  offset?: number
  status?: string
}

export function useCompanyMembersApi() {
  const tenantStore = useTenantStore()
  const { apiGet, apiPost } = useApi()

  function tenantId() {
    return tenantStore.tenantId
  }

  async function listCompanyMembers(companyId: string, params: ListCompanyMembersParams = {}) {
    const query: Record<string, string | number | undefined> = {
      tenant_id: tenantId(),
      limit: params.limit ?? 20,
      offset: params.offset ?? 0,
    }
    if (params.status) query.status = params.status

    const data = await apiGet<PaginatedResponse<CompanyMember>>(
      `/api/v1/companies/${companyId}/members`,
      { query },
    )
    return { ...data, items: data.items ?? [] }
  }

  async function addCompanyMember(
    companyId: string,
    payload: Omit<AddCompanyMemberPayload, 'tenant_id'>,
  ) {
    return apiPost(`/api/v1/companies/${companyId}/members`, {
      ...payload,
      tenant_id: tenantId(),
      position: payload.position?.trim() || undefined,
      role_id: payload.role_id?.trim() || undefined,
    })
  }

  function isApiUnavailableError(error: unknown): boolean {
    if (error instanceof ApiError) {
      return error.status === 0 || error.status >= 500 || error.code === 'SERVICE_UNAVAILABLE'
    }
    return error instanceof TypeError
  }

  return {
    listCompanyMembers,
    addCompanyMember,
    isApiUnavailableError,
  }
}
