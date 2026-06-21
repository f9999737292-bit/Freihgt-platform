import type { PaginatedResponse } from '~/types/api'
import type {
  AddRfxParticipantPayload,
  CreateRfxEventPayload,
  ListRfxEventsFilters,
  RfxEvent,
  RfxParticipant,
  UpdateRfxEventPayload,
} from '~/types/rfx'
import { ApiError } from '~/composables/useApi'

export interface ListRfxParticipantsParams {
  status?: string
}

export function useRfxApi() {
  const tenantStore = useTenantStore()
  const { apiGet, apiPost, apiPatch } = useApi()

  function tenantId() {
    return tenantStore.tenantId
  }

  function tenantQuery(extra: Record<string, string | number | undefined> = {}) {
    return { tenant_id: tenantId(), ...extra }
  }

  async function listRfxEvents(params: ListRfxEventsFilters = {}) {
    const query: Record<string, string | number | undefined> = {
      ...tenantQuery(),
      limit: params.limit ?? 20,
      offset: params.offset ?? 0,
    }
    if (params.rfx_type) query.rfx_type = params.rfx_type
    if (params.category) query.category = params.category
    if (params.status) query.status = params.status
    if (params.owner_company_id) query.owner_company_id = params.owner_company_id
    // TODO: backend search filter not implemented yet
    if (params.search?.trim()) query.search = params.search.trim()

    const data = await apiGet<PaginatedResponse<RfxEvent>>('/api/v1/rfx-events', { query })
    return { ...data, items: data.items ?? [] }
  }

  async function getRfxEvent(id: string) {
    return apiGet<RfxEvent>(`/api/v1/rfx-events/${id}`, { query: tenantQuery() })
  }

  async function createRfxEvent(payload: Omit<CreateRfxEventPayload, 'tenant_id'>) {
    return apiPost<RfxEvent>('/api/v1/rfx-events', {
      ...payload,
      tenant_id: tenantId(),
      description: payload.description?.trim() || undefined,
      currency_code: payload.currency_code?.trim() || undefined,
    })
  }

  async function updateRfxEvent(id: string, payload: UpdateRfxEventPayload) {
    return apiPatch<RfxEvent>(`/api/v1/rfx-events/${id}`, payload, { query: tenantQuery() })
  }

  async function publishRfxEvent(id: string) {
    return apiPost<{ id: string; status: string }>(
      `/api/v1/rfx-events/${id}/publish`,
      undefined,
      { query: tenantQuery() },
    )
  }

  async function cancelRfxEvent(id: string) {
    return apiPost<{ id: string; status: string }>(
      `/api/v1/rfx-events/${id}/cancel`,
      undefined,
      { query: tenantQuery() },
    )
  }

  async function listRfxParticipants(rfxEventId: string, params: ListRfxParticipantsParams = {}) {
    const query: Record<string, string | number | undefined> = tenantQuery()
    if (params.status) query.status = params.status
    const data = await apiGet<{ items: RfxParticipant[] }>(
      `/api/v1/rfx-events/${rfxEventId}/participants`,
      { query },
    )
    return data.items ?? []
  }

  async function addRfxParticipant(
    rfxEventId: string,
    payload: Omit<AddRfxParticipantPayload, 'tenant_id'>,
  ) {
    return apiPost<RfxParticipant>(`/api/v1/rfx-events/${rfxEventId}/participants`, {
      ...payload,
      tenant_id: tenantId(),
    })
  }

  function isApiUnavailableError(error: unknown): boolean {
    if (error instanceof ApiError) {
      return error.status === 0 || error.status >= 500 || error.code === 'SERVICE_UNAVAILABLE'
    }
    return error instanceof TypeError
  }

  return {
    listRfxEvents,
    getRfxEvent,
    createRfxEvent,
    updateRfxEvent,
    publishRfxEvent,
    cancelRfxEvent,
    listRfxParticipants,
    addRfxParticipant,
    isApiUnavailableError,
  }
}
