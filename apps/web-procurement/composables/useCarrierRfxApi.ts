import type { PaginatedResponse } from '~/types/api'
import type { RfxEvent, RfxLane, RfxLot, RfxParticipant } from '~/types/rfx'
import type { CarrierInvitedTender, CarrierResponseFilter, CarrierRfxResponse } from '~/types/carrierRfx'
import { isApiUnavailableError } from '~/utils/apiError'

export interface ListCarrierTendersParams {
  carrier_company_id?: string
  status?: string
  response_filter?: CarrierResponseFilter
  search?: string
  limit?: number
  offset?: number
}

export function useCarrierRfxApi() {
  const { apiGet, apiPost, apiPatch } = useApi()

  function carrierQuery(carrierCompanyId?: string) {
    return carrierCompanyId ? { carrier_company_id: carrierCompanyId } : {}
  }

  async function listInvitedTenders(params: ListCarrierTendersParams = {}) {
    const query: Record<string, string | number | undefined> = {
      limit: params.limit ?? 20,
      offset: params.offset ?? 0,
      ...carrierQuery(params.carrier_company_id),
    }
    if (params.status) query.status = params.status
    if (params.response_filter) query.response_filter = params.response_filter
    if (params.search?.trim()) query.search = params.search.trim()

    const data = await apiGet<PaginatedResponse<CarrierInvitedTender>>('/api/v1/carrier/rfx-events', { query })
    return { ...data, items: data.items ?? [] }
  }

  async function getTender(id: string) {
    return apiGet<RfxEvent>(`/api/v1/rfx-events/${encodeURIComponent(id)}`)
  }

  async function getOwnParticipant(eventId: string, carrierCompanyId?: string) {
    return apiGet<RfxParticipant>(`/api/v1/rfx-events/${encodeURIComponent(eventId)}/own-participant`, {
      query: carrierQuery(carrierCompanyId),
    })
  }

  async function getOwnResponse(eventId: string, carrierCompanyId?: string) {
    return apiGet<CarrierRfxResponse>(`/api/v1/rfx-events/${encodeURIComponent(eventId)}/own-response`, {
      query: carrierQuery(carrierCompanyId),
    })
  }

  async function createResponse(eventId: string, carrierCompanyId: string) {
    return apiPost<CarrierRfxResponse>(`/api/v1/rfx-events/${encodeURIComponent(eventId)}/responses`, {
      participant_company_id: carrierCompanyId,
    })
  }

  async function submitResponse(responseId: string) {
    return apiPost<CarrierRfxResponse>(`/api/v1/rfx-responses/${encodeURIComponent(responseId)}/submit`, {})
  }

  async function updateResponseCommercial(
    responseId: string,
    offerLines: Array<{ rfx_lot_id?: string | null; amount: number; currency_code: string; comment?: string | null }>,
  ) {
    return apiPatch<CarrierRfxResponse>(`/api/v1/rfx-responses/${encodeURIComponent(responseId)}`, {
      offer_lines: offerLines,
    })
  }

  async function getOwnAward(eventId: string, carrierCompanyId?: string) {
    return apiGet<{ id: string; rfx_response_id: string; total_amount?: number; currency_code?: string }>(
      `/api/v1/rfx-events/${encodeURIComponent(eventId)}/own-award`,
      { query: carrierQuery(carrierCompanyId) },
    )
  }

  async function listLots(eventId: string) {
    const data = await apiGet<{ items: RfxLot[] }>(`/api/v1/rfx-events/${encodeURIComponent(eventId)}/lots`)
    return data.items ?? []
  }

  async function listLanes(lotId: string, carrierCompanyId?: string) {
    const data = await apiGet<{ items: RfxLane[] }>(`/api/v1/rfx-lots/${encodeURIComponent(lotId)}/lanes`, {
      query: carrierQuery(carrierCompanyId),
    })
    return data.items ?? []
  }

  return {
    listInvitedTenders,
    getTender,
    getOwnParticipant,
    getOwnResponse,
    createResponse,
    submitResponse,
    updateResponseCommercial,
    getOwnAward,
    listLots,
    listLanes,
    isApiUnavailableError,
  }
}
