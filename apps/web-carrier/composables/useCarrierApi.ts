import type { PortalViewKind } from '../domain/viewState'

export interface TenderSummary {
  id: string
  rfx_number?: string
  title?: string
  status?: string
  response_deadline?: string
  own_response_status?: string | null
  participant_status?: string
}

export interface OfferLineInput {
  rfx_lot_id?: string | null
  amount: number
  currency_code: string
  comment?: string | null
}

export interface CarrierResponse {
  id: string
  status?: string
  offer_lines?: Array<OfferLineInput & { id?: string }>
}

export interface OwnAward {
  id: string
  total_amount?: number
  currency_code?: string
  awarded_at?: string
  rfx_response_id?: string
}

export interface TransportOrderSummary {
  transport_order_id: string
  transport_order_number?: string
  transport_order_status?: string
  amount?: number
  currency_code?: string
  provenance?: { amount?: number; currency_code?: string }
}

export function useCarrierApi() {
  const office = useCarrierOffice()

  function scoped(extra: Record<string, string | number | undefined | null> = {}) {
    const companyId = office.requireCompany()
    return {
      companyId,
      query: { carrier_company_id: companyId, ...extra },
    }
  }

  async function listTenders(filters: { status?: string; response_filter?: string; search?: string }) {
    const scope = scoped({ limit: 20, offset: 0, ...filters })
    return office.client.request<{ items?: TenderSummary[] }>('/api/v1/carrier/rfx-events', scope)
  }

  async function getTender(id: string) {
    const scope = scoped()
    return office.client.request<TenderSummary>(`/api/v1/carrier/rfx-events/${encodeURIComponent(id)}`, scope)
  }

  async function getOwnResponse(id: string) {
    const scope = scoped()
    return office.client.request<CarrierResponse>(`/api/v1/rfx-events/${encodeURIComponent(id)}/own-response`, scope)
  }

  async function createResponse(id: string) {
    const companyId = office.requireCompany()
    return office.client.request<CarrierResponse>(`/api/v1/rfx-events/${encodeURIComponent(id)}/responses`, {
      method: 'POST',
      companyId,
      body: { participant_company_id: companyId },
    })
  }

  async function saveOffer(responseId: string, offerLines: OfferLineInput[]) {
    const companyId = office.requireCompany()
    return office.client.request<CarrierResponse>(`/api/v1/rfx-responses/${encodeURIComponent(responseId)}`, {
      method: 'PATCH',
      companyId,
      body: { offer_lines: offerLines },
    })
  }

  async function submitResponse(responseId: string) {
    const companyId = office.requireCompany()
    return office.client.request<CarrierResponse>(`/api/v1/rfx-responses/${encodeURIComponent(responseId)}/submit`, {
      method: 'POST',
      companyId,
      body: {},
    })
  }

  async function getOwnAward(id: string) {
    const scope = scoped()
    return office.client.request<OwnAward>(`/api/v1/rfx-events/${encodeURIComponent(id)}/own-award`, scope)
  }

  async function listTransportOrders() {
    const scope = scoped({ limit: 50, offset: 0 })
    return office.client.request<{ items?: TransportOrderSummary[] }>('/api/v1/carrier/transport-orders', scope)
  }

  async function getTransportOrder(id: string) {
    const companyId = office.requireCompany()
    return office.client.request<TransportOrderSummary>(
      `/api/v1/order-execution/transport-orders/${encodeURIComponent(id)}`,
      {
        companyId,
        query: { company_id: companyId, actor: 'CARRIER' },
      },
    )
  }

  async function listFleet() {
    const companyId = office.requireCompany()
    const scope = { companyId, query: { carrier_company_id: companyId } }
    const [drivers, vehicles] = await Promise.all([
      office.client.request<{ items?: Array<{ id: string; full_name?: string; status?: string }> }>('/api/v1/drivers', scope),
      office.client.request<{ items?: Array<{ id: string; plate_number?: string; status?: string }> }>('/api/v1/vehicles', scope),
    ])
    return {
      drivers: drivers.items ?? [],
      vehicles: vehicles.items ?? [],
    }
  }

  return {
    listTenders,
    getTender,
    getOwnResponse,
    createResponse,
    saveOffer,
    submitResponse,
    getOwnAward,
    listTransportOrders,
    getTransportOrder,
    listFleet,
  }
}

export type { PortalViewKind }
