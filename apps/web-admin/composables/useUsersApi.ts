import type { PaginatedResponse } from '~/types/api'
import type {
  CreateUserPayload,
  ListUsersFilters,
  UpdateUserPayload,
  User,
  UserCompanyMembership,
} from '~/types/user'
import { ApiError } from '~/composables/useApi'

export function useUsersApi() {
  const tenantStore = useTenantStore()
  const { apiGet, apiPost, apiPatch } = useApi()

  function tenantId() {
    return tenantStore.tenantId
  }

  async function listUsers(params: ListUsersFilters = {}) {
    const query: Record<string, string | number | undefined> = {
      tenant_id: tenantId(),
      limit: params.limit ?? 20,
      offset: params.offset ?? 0,
    }
    if (params.search?.trim()) query.search = params.search.trim()
    if (params.status) query.status = params.status

    const data = await apiGet<PaginatedResponse<User>>('/api/v1/users', { query })
    return { ...data, items: data.items ?? [] }
  }

  async function getUser(id: string) {
    return apiGet<User>(`/api/v1/users/${id}`)
  }

  async function createUser(payload: Omit<CreateUserPayload, 'tenant_id'>) {
    return apiPost<User>('/api/v1/users', {
      ...payload,
      tenant_id: tenantId(),
      phone: payload.phone?.trim() || undefined,
    })
  }

  async function updateUser(id: string, payload: UpdateUserPayload) {
    return apiPatch<User>(`/api/v1/users/${id}`, {
      ...payload,
      phone: payload.phone?.trim() || undefined,
    })
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
    listUsers,
    getUser,
    createUser,
    updateUser,
    getUserCompanies,
    isApiUnavailableError,
  }
}
