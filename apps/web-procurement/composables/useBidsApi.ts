import type { Bid } from '~/types/bid'
import { ApiError } from '~/composables/useApi'

export function useBidsApi() {
  const { apiGet, apiPost } = useApi()

  async function getBid(id: string) {
    return apiGet<Bid>(`/api/v1/bids/${encodeURIComponent(id)}`)
  }

  async function acceptBid(id: string) {
    return apiPost<{ id: string; status: string }>(
      `/api/v1/bids/${encodeURIComponent(id)}/accept`,
      {},
    )
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

  function isConflictError(error: unknown): boolean {
    return error instanceof ApiError && error.status === 409
  }

  return {
    getBid,
    acceptBid,
    isUnauthorizedError,
    isNotFoundError,
    isServerError,
    isNetworkError,
    isConflictError,
  }
}
