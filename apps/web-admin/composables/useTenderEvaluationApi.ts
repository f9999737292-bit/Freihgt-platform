import type { ScoringFactorWeight } from '~/types/tender'

export interface RunEvaluationPayload {
  tenant_id?: string
  scoring_template_version_id: string
  qualification_rules: {
    minimum_sla_score?: number
    minimum_capacity?: number
    require_carrier_active?: boolean
  }
  required_volume?: number
}

export interface RunEvaluationResponse {
  evaluation_id: string
  qualification: Array<{
    carrier_company_id: string
    result: 'QUALIFIED' | 'DISQUALIFIED'
    reasons: string[]
  }>
  scores: Array<{
    carrier_company_id: string
    total_score: number
    price_score: number
    sla_score: number
    carrier_kpi_score: number
    capacity_score: number
    reliability_score: number
    transit_time_score: number
    contributions: Array<{ factor: string; weight: number; raw_score: number; contribution: number }>
  }>
}

export function useTenderEvaluationApi() {
  const tenantStore = useTenantStore()
  const { apiPost } = useApi()

  function tenantId() {
    return tenantStore.tenantId
  }

  async function runEvaluation(rfxEventId: string, payload: Omit<RunEvaluationPayload, 'tenant_id'>) {
    return apiPost<RunEvaluationResponse>(`/api/v1/rfx-events/${rfxEventId}/evaluate`, {
      tenant_id: tenantId(),
      ...payload,
    })
  }

  async function createScoringTemplate(code: string, name: string, factors: ScoringFactorWeight[]) {
    return apiPost<{ template_id: string; version_id: string }>('/api/v1/scoring-templates', {
      tenant_id: tenantId(),
      code,
      name,
      factors,
    })
  }

  return { runEvaluation, createScoringTemplate }
}
