import type { PaginatedResponse } from '~/types/api'
import type {
  Bid,
  CreateBidPayload,
  CreateFreightRequestFromOrderPayload,
  FreightRequest,
  ListFreightRequestsFilters,
} from '~/types/rfx'
import { ApiError } from '~/composables/useApi'

export interface ListFreightRequestBidsParams {
  status?: string
}

export function useFreightRequestsApi() {
  const tenantStore = useTenantStore()
  const { apiGet, apiPost } = useApi()

  function tenantId() {
    return tenantStore.tenantId
  }

  function tenantQuery(extra: Record<string, string | number | undefined> = {}) {
    return { tenant_id: tenantId(), ...extra }
  }

  async function listFreightRequests(params: ListFreightRequestsFilters = {}) {
    const query: Record<string, string | number | undefined> = {
      ...tenantQuery(),
      limit: params.limit ?? 20,
      offset: params.offset ?? 0,
    }
    if (params.request_type) query.request_type = params.request_type
    if (params.status) query.status = params.status
    if (params.shipper_company_id) query.shipper_company_id = params.shipper_company_id
    // TODO: backend search filter not implemented yet
    if (params.search?.trim()) query.search = params.search.trim()

    const data = await apiGet<PaginatedResponse<FreightRequest>>('/api/v1/freight-requests', { query })
    return { ...data, items: data.items ?? [] }
  }

  async function getFreightRequest(id: string) {
    return apiGet<FreightRequest>(`/api/v1/freight-requests/${id}`, { query: tenantQuery() })
  }

  async function createFreightRequestFromTransportOrder(
    payload: Omit<CreateFreightRequestFromOrderPayload, 'tenant_id'>,
  ) {
    return apiPost<FreightRequest>('/api/v1/freight-requests/from-transport-order', {
      ...payload,
      tenant_id: tenantId(),
      currency_code: payload.currency_code?.trim() || undefined,
      response_deadline: payload.response_deadline || undefined,
    })
  }

  async function publishFreightRequest(id: string) {
    return apiPost<{ id: string; status: string }>(
      `/api/v1/freight-requests/${id}/publish`,
      undefined,
      { query: tenantQuery() },
    )
  }

  async function listFreightRequestBids(id: string, params: ListFreightRequestBidsParams = {}) {
    const query: Record<string, string | number | undefined> = tenantQuery()
    if (params.status) query.status = params.status
    const data = await apiGet<{ items: Bid[] }>(`/api/v1/freight-requests/${id}/bids`, { query })
    return data.items ?? []
  }

  async function createBid(freightRequestId: string, payload: Omit<CreateBidPayload, 'tenant_id'>) {
    return apiPost<Bid>(`/api/v1/freight-requests/${freightRequestId}/bids`, {
      ...payload,
      tenant_id: tenantId(),
      currency_code: payload.currency_code?.trim() || undefined,
    })
  }

  function isApiUnavailableError(error: unknown): boolean {
    if (error instanceof ApiError) {
      return error.status === 0 || error.status >= 500 || error.code === 'SERVICE_UNAVAILABLE'
    }
    return error instanceof TypeError
  }

  return {
    listFreightRequests,
    getFreightRequest,
    createFreightRequestFromTransportOrder,
    publishFreightRequest,
    listFreightRequestBids,
    createBid,
    isApiUnavailableError,
  }
}
