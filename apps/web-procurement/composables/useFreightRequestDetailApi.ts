import type { Bid } from '~/types/bid'
import type { FreightRequest } from '~/types/rfx'
import { ApiError } from '~/utils/apiClient'

export interface ListFreightRequestBidsParams {
  status?: string
}

export function useFreightRequestDetailApi() {
  const { apiGet } = useApi()

  async function getFreightRequest(id: string) {
    return apiGet<FreightRequest>(`/api/v1/freight-requests/${encodeURIComponent(id)}`)
  }

  async function listFreightRequestBids(
    id: string,
    params: ListFreightRequestBidsParams = {},
  ) {
    const query: Record<string, string | number | undefined> = {}
    if (params.status) query.status = params.status

    const data = await apiGet<{ items?: Bid[] }>(
      `/api/v1/freight-requests/${encodeURIComponent(id)}/bids`,
      { query },
    )
    return data.items ?? []
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
    getFreightRequest,
    listFreightRequestBids,
    isUnauthorizedError,
    isNotFoundError,
    isServerError,
    isNetworkError,
  }
}
