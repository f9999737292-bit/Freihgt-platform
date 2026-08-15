export const TENDER_SCORING_FACTORS = [
  'PRICE',
  'SLA',
  'CARRIER_KPI',
  'CAPACITY',
  'RELIABILITY',
  'TRANSIT_TIME',
] as const

export type TenderScoringFactor = (typeof TENDER_SCORING_FACTORS)[number]

export interface ScoringFactorWeight {
  factor: TenderScoringFactor | string
  weight: number
}

export const DEFAULT_SCORING_TEMPLATE: ScoringFactorWeight[] = [
  { factor: 'PRICE', weight: 35 },
  { factor: 'SLA', weight: 20 },
  { factor: 'CARRIER_KPI', weight: 15 },
  { factor: 'CAPACITY', weight: 10 },
  { factor: 'RELIABILITY', weight: 10 },
  { factor: 'TRANSIT_TIME', weight: 10 },
]

export const ALLOCATION_STRATEGIES = [
  'WINNER_TAKES_MOST',
  'DUAL_SOURCE',
  'DIVERSIFIED',
  'EQUAL_SPLIT',
  'SCORE_WEIGHTED',
  'CAPACITY_WEIGHTED',
  'MANUAL',
] as const

export type AllocationStrategy = (typeof ALLOCATION_STRATEGIES)[number]

export interface QualificationRules {
  minimum_sla_score?: number
  minimum_capacity?: number
  require_carrier_active?: boolean
}

export interface QualificationResult {
  carrier_company_id: string
  lot_id?: string
  result: 'QUALIFIED' | 'DISQUALIFIED'
  reasons: string[]
}

export interface FactorContribution {
  factor: string
  weight: number
  raw_score: number
  contribution: number
}

export interface CarrierScoreResult {
  carrier_company_id: string
  lot_id?: string
  bid_revision_id?: string
  total_score: number
  contributions: FactorContribution[]
  price_score: number
  sla_score: number
  carrier_kpi_score: number
  capacity_score: number
  reliability_score: number
  transit_time_score: number
}

export interface RunEvaluationResponse {
  evaluation_id: string
  qualification: QualificationResult[]
  scores: CarrierScoreResult[]
  scoring_snapshot?: {
    version_number: number
    factors: ScoringFactorWeight[]
  }
}

export interface TenderBidRevision {
  id: string
  rfx_response_id?: string
  rfx_event_id?: string
  bid_id?: string
  freight_request_id?: string
  participant_company_id?: string
  carrier_company_id?: string
  revision_number: number
  is_active: boolean
  price_amount?: number
  total_amount?: number
  currency_code: string
  capacity_units: number
  transit_hours: number
  sla_score_input: number
  carrier_kpi_score_input: number
  reliability_score_input: number
  comment?: string | null
  submitted_at?: string | null
  created_at: string
}

export interface AllocationConstraints {
  min_suppliers?: number
  max_suppliers?: number
  min_share_pct?: number
  max_share_pct?: number
  total_volume?: number
  max_carrier_share_pct?: number
}

export interface ManualShare {
  carrier_company_id: string
  share_pct: number
}

export interface AllocationConfig {
  strategy: AllocationStrategy | string
  rank_shares?: number[]
  manual_shares?: ManualShare[]
  constraints: AllocationConstraints
}

export interface AllocationLine {
  carrier_company_id: string
  lot_id?: string
  score: number
  base_share_pct: number
  balance_adjustment_pct: number
  proposed_share_pct: number
  committed_capacity: number
  proposed_volume: number
}

export interface AllocationSummary {
  expected_cost: number
  weighted_score: number
  supplier_count: number
  max_concentration_pct: number
  capacity_coverage_pct: number
}

export interface AllocationOutcome {
  status: 'COMPUTED' | 'INFEASIBLE' | string
  reasons?: string[]
  lines?: AllocationLine[]
  summary: AllocationSummary
}

export interface QuotaTarget {
  carrier_company_id: string
  target_share_pct: number
}

export interface QuotaBalancePolicy {
  tolerance_pct?: number
  carry_balance?: boolean
  max_correction_pct?: number
  period_type?: string
}

export interface QuotaPosition {
  carrier_company_id: string
  target_share_pct: number
  actual_share_pct: number
  balance_pp: number
  status: 'UNDERALLOCATED' | 'BALANCED' | 'OVERALLOCATED' | string
  next_adjustment_pct: number
}

export interface RunAllocationResponse {
  scenario_id: string
  outcome: AllocationOutcome
  quota: QuotaPosition[]
}

export type AwardProposalStatus =
  | 'DRAFT_PROPOSAL'
  | 'PENDING_APPROVAL'
  | 'APPROVED'
  | 'REJECTED'
  | 'AWARDED'
  | string

export interface AwardConversionResult {
  policy: string
  status: string
  shipment_id?: string
  transport_order_id?: string
  idempotency_key: string
  message?: string
}

export interface FinalizeAwardResponse {
  award_id: string
  conversion?: AwardConversionResult
}
