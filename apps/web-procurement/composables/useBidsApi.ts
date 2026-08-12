import type { Bid } from '~/types/bid'
import { ApiError } from '~/composables/useApi'

export function useBidsApi() {
  const { apiGet } = useApi()

  async function getBid(id: string) {
    return apiGet<Bid>(`/api/v1/bids/${encodeURIComponent(id)}`)
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
    getBid,
    isUnauthorizedError,
    isNotFoundError,
    isServerError,
    isNetworkError,
  }
}
