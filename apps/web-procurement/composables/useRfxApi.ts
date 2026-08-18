import type { PaginatedResponse } from '~/types/api'
import type {
  AddRfxParticipantPayload,
  CreateRfxEventPayload,
  ListRfxEventsFilters,
  RfxEvent,
  RfxParticipant,
  UpdateRfxEventPayload,
} from '~/types/rfx'
import { isApiUnavailableError } from '~/utils/apiError'

export interface ListRfxParticipantsParams {
  status?: string
}

export function useRfxApi() {
  const tenantStore = useTenantStore()
  const { apiGet, apiPost, apiPatch } = useApi()

  function tenantId() {
    return tenantStore.tenantId
  }

  async function listRfxEvents(params: ListRfxEventsFilters = {}) {
    const query: Record<string, string | number | undefined> = {
      limit: params.limit ?? 20,
      offset: params.offset ?? 0,
    }
    if (params.rfx_type) query.rfx_type = params.rfx_type
    if (params.category) query.category = params.category
    if (params.status) query.status = params.status
    if (params.owner_company_id) query.owner_company_id = params.owner_company_id
    if (params.search?.trim()) query.search = params.search.trim()

    const data = await apiGet<PaginatedResponse<RfxEvent>>('/api/v1/rfx-events', { query })
    return { ...data, items: data.items ?? [] }
  }

  async function getRfxEvent(id: string) {
    return apiGet<RfxEvent>(`/api/v1/rfx-events/${id}`)
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
    return apiPatch<RfxEvent>(`/api/v1/rfx-events/${id}`, payload)
  }

  async function publishRfxEvent(id: string) {
    return apiPost<{ id: string; status: string }>(`/api/v1/rfx-events/${id}/publish`, {})
  }

  async function cancelRfxEvent(id: string) {
    return apiPost<{ id: string; status: string }>(`/api/v1/rfx-events/${id}/cancel`, {})
  }

  async function listRfxParticipants(rfxEventId: string, params: ListRfxParticipantsParams = {}) {
    const query: Record<string, string | number | undefined> = {}
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
