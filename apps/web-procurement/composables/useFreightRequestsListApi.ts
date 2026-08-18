import type { PaginatedResponse } from '~/types/api'
import type { FreightRequest, ListFreightRequestsFilters } from '~/types/rfx'
import { ApiError } from '~/utils/apiClient'

export function useFreightRequestsListApi() {
  const { apiGet } = useApi()

  async function listFreightRequests(params: ListFreightRequestsFilters = {}) {
    const query: Record<string, string | number | undefined> = {
      limit: params.limit ?? 20,
      offset: params.offset ?? 0,
    }
    if (params.request_type) query.request_type = params.request_type
    if (params.status) query.status = params.status
    if (params.shipper_company_id) query.shipper_company_id = params.shipper_company_id

    const data = await apiGet<PaginatedResponse<FreightRequest>>('/api/v1/freight-requests', {
      query,
    })
    return { ...data, items: data.items ?? [] }
  }

  function isUnauthorizedError(error: unknown): boolean {
    return error instanceof ApiError && error.status === 401
  }

  function isNotFoundError(error: unknown): boolean {
    return error instanceof ApiError && error.status === 404
  }

  function isServerError(error: unknown): boolean {
    return error instanceof ApiError && error.status >= 500
  }

  function isNetworkError(error: unknown): boolean {
    if (error instanceof ApiError) {
      return error.status === 0 || error.code === 'NETWORK_ERROR'
    }
    return error instanceof TypeError
  }

  return {
    listFreightRequests,
    isUnauthorizedError,
    isNotFoundError,
    isServerError,
    isNetworkError,
  }
}
