import type {
  AllocationConfig,
  AwardConversionResult,
  FinalizeAwardResponse,
  QualificationRules,
  QuotaBalancePolicy,
  QuotaTarget,
  RunAllocationResponse,
  RunEvaluationResponse,
  ScoringFactorWeight,
  TenderBidRevision,
} from '~/types/tender'

export interface RunEvaluationPayload {
  tenant_id?: string
  scoring_template_version_id: string
  qualification_rules: QualificationRules
  required_volume?: number
}

interface ListItemsResponse<T> {
  items: T[]
}

function carrierHeaders(carrierCompanyId?: string | null): Record<string, string> | undefined {
  if (!carrierCompanyId) return undefined
  return { 'X-Carrier-Company-ID': carrierCompanyId }
}

export function useTenderEvaluationApi() {
  const tenantStore = useTenantStore()
  const { apiGet, apiPost } = useApi()

  function tenantId() {
    return tenantStore.tenantId
  }

  function withTenant<T extends Record<string, unknown>>(payload: T) {
    return { tenant_id: tenantId(), ...payload }
  }

  async function listEventBids(rfxEventId: string, carrierCompanyId?: string | null) {
    return apiGet<ListItemsResponse<TenderBidRevision>>(`/api/v1/rfx-events/${rfxEventId}/bids`, {
      headers: carrierHeaders(carrierCompanyId),
    })
  }

  async function getMyResponse(rfxEventId: string, carrierCompanyId: string) {
    return apiGet<TenderBidRevision>(`/api/v1/rfx-events/${rfxEventId}/responses/mine`, {
      headers: carrierHeaders(carrierCompanyId),
    })
  }

  async function listResponseRevisions(
    rfxEventId: string,
    responseId: string,
    carrierCompanyId?: string | null,
  ) {
    return apiGet<ListItemsResponse<TenderBidRevision>>(
      `/api/v1/rfx-events/${rfxEventId}/responses/${responseId}/revisions`,
      { headers: carrierHeaders(carrierCompanyId) },
    )
  }

  async function submitResponseRevision(
    rfxEventId: string,
    responseId: string,
    payload: {
      participant_company_id: string
      price_amount: number
      currency_code: string
      capacity_units: number
      transit_hours: number
      sla_score_input: number
      carrier_kpi_score_input: number
      reliability_score_input: number
      comment?: string
      idempotency_key?: string
    },
    carrierCompanyId?: string | null,
  ) {
    return apiPost<TenderBidRevision>(
      `/api/v1/rfx-events/${rfxEventId}/responses/${responseId}/revisions`,
      payload,
      { headers: carrierHeaders(carrierCompanyId ?? payload.participant_company_id) },
    )
  }

  async function runEvaluation(rfxEventId: string, payload: Omit<RunEvaluationPayload, 'tenant_id'>) {
    return apiPost<RunEvaluationResponse>(`/api/v1/rfx-events/${rfxEventId}/evaluate`, withTenant(payload))
  }

  async function createScoringTemplate(code: string, name: string, factors: ScoringFactorWeight[]) {
    return apiPost<{ template_id: string; version_id: string }>(
      '/api/v1/scoring-templates',
      withTenant({ code, name, factors }),
    )
  }

  async function runAllocationScenario(payload: {
    evaluation_id: string
    name: string
    config: AllocationConfig
    quota_targets?: QuotaTarget[]
    quota_policy?: QuotaBalancePolicy
    actual_shares?: Record<string, number>
  }) {
    return apiPost<RunAllocationResponse>('/api/v1/allocation-scenarios', withTenant(payload))
  }

  async function createAwardProposal(payload: {
    rfx_event_id: string
    evaluation_id: string
    scenario_id: string
    idempotency_key?: string
  }) {
    return apiPost<{ proposal_id: string }>('/api/v1/award-proposals', withTenant(payload))
  }

  async function submitAwardProposal(proposalId: string) {
    return apiPost<{ status: string }>(
      `/api/v1/award-proposals/${proposalId}/submit`,
      withTenant({}),
    )
  }

  async function approveAwardProposal(proposalId: string) {
    return apiPost<{ status: string }>(
      `/api/v1/award-proposals/${proposalId}/approve`,
      withTenant({}),
    )
  }

  async function rejectAwardProposal(proposalId: string) {
    return apiPost<{ status: string }>(
      `/api/v1/award-proposals/${proposalId}/reject`,
      withTenant({}),
    )
  }

  async function finalizeAwardProposal(proposalId: string, idempotencyKey?: string) {
    return apiPost<FinalizeAwardResponse>(
      `/api/v1/award-proposals/${proposalId}/finalize`,
      withTenant({ idempotency_key: idempotencyKey }),
    )
  }

  async function getBidRevisionCurrent(bidId: string, carrierCompanyId?: string | null) {
    return apiGet<TenderBidRevision>(`/api/v1/bids/${bidId}/revisions/current`, {
      headers: carrierHeaders(carrierCompanyId),
    })
  }

  async function listBidRevisions(bidId: string, carrierCompanyId?: string | null) {
    return apiGet<ListItemsResponse<TenderBidRevision>>(`/api/v1/bids/${bidId}/revisions`, {
      headers: carrierHeaders(carrierCompanyId),
    })
  }

  async function submitBidRevision(
    bidId: string,
    payload: {
      carrier_company_id: string
      total_amount: number
      currency_code: string
      capacity_units: number
      transit_hours: number
      sla_score_input: number
      carrier_kpi_score_input: number
      reliability_score_input: number
      comment?: string
      idempotency_key?: string
    },
    carrierCompanyId?: string | null,
  ) {
    return apiPost<TenderBidRevision>(
      `/api/v1/bids/${bidId}/revisions`,
      payload,
      { headers: carrierHeaders(carrierCompanyId ?? payload.carrier_company_id) },
    )
  }

  return {
    listEventBids,
    getMyResponse,
    listResponseRevisions,
    submitResponseRevision,
    runEvaluation,
    createScoringTemplate,
    runAllocationScenario,
    createAwardProposal,
    submitAwardProposal,
    approveAwardProposal,
    rejectAwardProposal,
    finalizeAwardProposal,
    getBidRevisionCurrent,
    listBidRevisions,
    submitBidRevision,
  }
}

export type { AwardConversionResult, RunEvaluationResponse }
