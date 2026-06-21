import type { Bid } from '~/types/rfx'
import { ApiError } from '~/composables/useApi'

export function useBidsApi() {
  const tenantStore = useTenantStore()
  const { apiPost } = useApi()

  function tenantQuery() {
    return { tenant_id: tenantStore.tenantId }
  }

  async function submitBid(id: string) {
    return apiPost<{ id: string; status: string }>(`/api/v1/bids/${id}/submit`, undefined, {
      query: tenantQuery(),
    })
  }

  async function acceptBid(id: string) {
    return apiPost<{ id: string; status: string }>(`/api/v1/bids/${id}/accept`, undefined, {
      query: tenantQuery(),
    })
  }

  function isApiUnavailableError(error: unknown): boolean {
    if (error instanceof ApiError) {
      return error.status === 0 || error.status >= 500 || error.code === 'SERVICE_UNAVAILABLE'
    }
    return error instanceof TypeError
  }

  function isAccepted(bid: Bid): boolean {
    return bid.status === 'ACCEPTED'
  }

  return {
    submitBid,
    acceptBid,
    isApiUnavailableError,
    isAccepted,
  }
}
