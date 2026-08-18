import type { CreateRfxLanePayload, CreateRfxLotPayload, RfxLane, RfxLot } from '~/types/rfx'
import { isApiUnavailableError } from '~/utils/apiError'

export function useRfxLotsApi() {
  const tenantStore = useTenantStore()
  const { apiGet, apiPost } = useApi()

  function tenantId() {
    return tenantStore.tenantId
  }

  async function listLots(rfxEventId: string) {
    const data = await apiGet<{ items: RfxLot[] }>(`/api/v1/rfx-events/${rfxEventId}/lots`)
    return data.items ?? []
  }

  async function createLot(rfxEventId: string, payload: Omit<CreateRfxLotPayload, 'tenant_id'>) {
    return apiPost<RfxLot>(`/api/v1/rfx-events/${rfxEventId}/lots`, {
      ...payload,
      tenant_id: tenantId(),
      description: payload.description?.trim() || undefined,
      category: payload.category?.trim() || undefined,
      currency_code: payload.currency_code?.trim() || undefined,
    })
  }

  async function createLane(lotId: string, payload: Omit<CreateRfxLanePayload, 'tenant_id'>) {
    return apiPost<RfxLane>(`/api/v1/rfx-lots/${lotId}/lanes`, {
      ...payload,
      tenant_id: tenantId(),
      equipment_type: payload.equipment_type?.trim() || undefined,
      volume_unit: payload.volume_unit?.trim() || undefined,
      required_service_level: payload.required_service_level?.trim() || undefined,
    })
  }

  return {
    listLots,
    createLot,
    createLane,
    isApiUnavailableError,
  }
}
