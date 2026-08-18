import type { AuditEventItem, EvaluationResponseItem, RfxAwardResult, AwardTransportOrderItem, ConvertTransportOrdersResult } from '~/types/evaluation'

export function useRfxEvaluationApi() {
  const { apiGet, apiPost, apiPatch, apiDelete } = useApi()

  async function listEvaluationResponses(eventId: string) {
    const data = await apiGet<{ items: EvaluationResponseItem[] }>(
      `/api/v1/rfx-events/${encodeURIComponent(eventId)}/responses`,
    )
    return data.items ?? []
  }

  async function recalculateEvaluation(eventId: string) {
    const data = await apiPost<{ items: EvaluationResponseItem[] }>(
      `/api/v1/rfx-events/${encodeURIComponent(eventId)}/evaluation/recalculate`,
      {},
    )
    return data.items ?? []
  }

  async function updateManualScore(responseId: string, manualScore: number) {
    return apiPatch<EvaluationResponseItem>(`/api/v1/rfx-responses/${encodeURIComponent(responseId)}/evaluation`, {
      manual_score: manualScore,
    })
  }

  async function addToShortlist(responseId: string) {
    return apiPost(`/api/v1/rfx-responses/${encodeURIComponent(responseId)}/shortlist`, {})
  }

  async function removeFromShortlist(responseId: string) {
    return apiDelete(`/api/v1/rfx-responses/${encodeURIComponent(responseId)}/shortlist`)
  }

  async function awardResponse(eventId: string, responseId: string) {
    return apiPost<RfxAwardResult>(`/api/v1/rfx-events/${encodeURIComponent(eventId)}/award-response`, {
      response_id: responseId,
    })
  }

  async function listAuditEvents(eventId: string) {
    const data = await apiGet<{ items: AuditEventItem[] }>(
      `/api/v1/rfx-events/${encodeURIComponent(eventId)}/audit-events`,
    )
    return data.items ?? []
  }

  async function listAwardTransportOrders(eventId: string) {
    const data = await apiGet<{ items: AwardTransportOrderItem[] }>(
      `/api/v1/rfx-events/${encodeURIComponent(eventId)}/transport-orders`,
    )
    return data.items ?? []
  }

  async function convertAwardToTransportOrders(eventId: string) {
    return apiPost<ConvertTransportOrdersResult>(
      `/api/v1/rfx-events/${encodeURIComponent(eventId)}/transport-orders`,
      {},
    )
  }

  return {
    listEvaluationResponses,
    recalculateEvaluation,
    updateManualScore,
    addToShortlist,
    removeFromShortlist,
    awardResponse,
    listAuditEvents,
    listAwardTransportOrders,
    convertAwardToTransportOrders,
  }
}
