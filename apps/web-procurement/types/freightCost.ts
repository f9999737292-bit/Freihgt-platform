/** Wire-format monetary amount — never parse with JS Number for arithmetic. */
export type DecimalString = string

export const FREIGHT_COST_DATA_STAGES = [
  'PLANNED_ONLY',
  'ACCRUAL_PARTIAL',
  'ACCRUAL_COMPLETE',
  'ACTUAL_PARTIAL',
  'ACTUAL_COMPLETE',
] as const

export type FreightCostDataStage = (typeof FREIGHT_COST_DATA_STAGES)[number]

export const FREIGHT_COST_FINANCIAL_FINALITIES = [
  'NOT_EVALUATED',
  'DRAFT',
  'CURRENT_ACTUAL',
  'FINAL_ACTUAL',
  'CANCELLED',
] as const

export type FreightCostFinancialFinality = (typeof FREIGHT_COST_FINANCIAL_FINALITIES)[number]

export const FREIGHT_COST_FORECAST_SOURCE_STATUSES = ['KNOWN', 'KNOWN_EMPTY', 'UNKNOWN'] as const

export type FreightCostForecastSourceStatus = (typeof FREIGHT_COST_FORECAST_SOURCE_STATUSES)[number]

export const FREIGHT_COST_RECONCILIATION_STATUSES = ['MATCH', 'MISMATCH', 'UNLINKED'] as const

export type FreightCostReconciliationStatus = (typeof FREIGHT_COST_RECONCILIATION_STATUSES)[number]

export const FREIGHT_COST_PLANNED_SOURCES = [
  'CONTRACT_RATE',
  'SPOT_AWARD',
  'MANUAL_OVERRIDE',
  'UNKNOWN',
] as const

export type FreightCostPlannedSource = (typeof FREIGHT_COST_PLANNED_SOURCES)[number]

export const FREIGHT_COST_SOURCES_AVAILABLE = [
  'PLANNED',
  'ACCRUAL',
  'FORECAST',
  'CURRENT_ACTUAL',
  'FINAL_ACTUAL',
  'BILLING',
  'PAID',
] as const

export type FreightCostSourceAvailable = (typeof FREIGHT_COST_SOURCES_AVAILABLE)[number]

export const FREIGHT_COST_ACCESSORIAL_CATEGORIES = [
  'DETENTION',
  'WAITING',
  'FUEL',
  'TOLL',
  'HANDLING',
  'STORAGE',
  'OTHER',
  'UNKNOWN',
] as const

export type FreightCostAccessorialCategory = (typeof FREIGHT_COST_ACCESSORIAL_CATEGORIES)[number]

export const FREIGHT_COST_VARIANCE_DRIVER_TYPES = [
  'ACCESSORIAL',
  'PRINCIPAL',
  'FUEL',
  'DETENTION',
  'WAITING',
  'UNATTRIBUTED',
] as const

export type FreightCostVarianceDriverType = (typeof FREIGHT_COST_VARIANCE_DRIVER_TYPES)[number]

export const FREIGHT_COST_DATE_DIMENSIONS = [
  'TRANSPORT_ORDER_CREATED_AT',
  'SETTLEMENT_BUSINESS_DATE',
  'BILLING_REGISTER_DATE',
] as const

export type FreightCostDateDimension = (typeof FREIGHT_COST_DATE_DIMENSIONS)[number]

export type FreightCostActor = 'BUYER' | 'CARRIER'

/** Public per-transport-order summary DTO (v2.1E contract). */
export interface FreightCostSummaryDTO {
  transport_order_id: string
  shipment_id: string | null
  buyer_company_id: string
  carrier_company_id: string
  currency_code: string
  data_stage: FreightCostDataStage
  financial_finality: FreightCostFinancialFinality
  sources_available: FreightCostSourceAvailable[]
  planned_amount: DecimalString | null
  accrued_amount: DecimalString | null
  forecast_exposure: DecimalString | null
  forecast_source_status: FreightCostForecastSourceStatus
  current_actual_amount: DecimalString | null
  final_actual_amount: DecimalString | null
  billing_register_amount: DecimalString | null
  paid_amount: DecimalString | null
  current_variance_amount: DecimalString | null
  final_variance_amount: DecimalString | null
  current_variance_percent: DecimalString | null
  final_variance_percent: DecimalString | null
  billing_reconciliation_status: FreightCostReconciliationStatus | null
  cost_updated_at: string
  availability_reasons: string[]
}

export interface FreightCostSummaryPeriodDTO {
  from: string
  to: string
  date_dimension: FreightCostDateDimension
}

export interface FreightCostSummaryKpisDTO {
  planned_total: DecimalString | null
  accrued_total: DecimalString | null
  forecast_exposure_total: DecimalString | null
  pending_proposed_accessorial_total: DecimalString | null
  current_actual_total: DecimalString | null
  final_actual_total: DecimalString | null
  current_variance_total: DecimalString | null
  final_variance_total: DecimalString | null
  reconciliation_mismatch_count: number
}

/** Aggregate summary DTO (v2.1E contract). */
export interface FreightCostSummaryAggregateDTO {
  currency_code: string
  period: FreightCostSummaryPeriodDTO
  kpis: FreightCostSummaryKpisDTO
  mixed_currency: boolean
}

export interface FreightCostListResponse {
  items: FreightCostSummaryDTO[]
  total: number
  limit: number
  offset: number
}

/** Planned vs Actual table row view-model. */
export interface FreightCostOrderRowVM {
  transport_order_id: string
  shipment_id: string | null
  order_reference: string
  carrier_company_id: string
  carrier_name: string
  planned_amount: DecimalString | null
  accrued_amount: DecimalString | null
  forecast_exposure: DecimalString | null
  current_actual_amount: DecimalString | null
  final_actual_amount: DecimalString | null
  current_variance_amount: DecimalString | null
  final_variance_amount: DecimalString | null
  currency_code: string
  financial_finality: FreightCostFinancialFinality
  billing_reconciliation_status: FreightCostReconciliationStatus | null
  availability_summary: string[]
  cost_updated_at: string
}

export interface FreightCostVarianceDriverVM {
  driver_type: FreightCostVarianceDriverType
  category: FreightCostAccessorialCategory | null
  amount: DecimalString | null
  description: string
}

export interface FreightCostReconciliationFindingVM {
  finding_id: string
  finding_type: string
  status: string
  message: string
}

/** Shipment cost detail view-model. */
export interface FreightCostDetailVM {
  summary: FreightCostSummaryDTO
  order_reference: string
  carrier_name: string
  planned_source: FreightCostPlannedSource | null
  variance_drivers: FreightCostVarianceDriverVM[]
  reconciliation_findings: FreightCostReconciliationFindingVM[]
}

export interface FreightCostAccessorialSpendRowVM {
  normalized_category: FreightCostAccessorialCategory
  total_amount: DecimalString
  currency_code: string
  order_count: number
}

export interface FreightCostAccessorialSpendResponse {
  items: FreightCostAccessorialSpendRowVM[]
  currency_code: string
  data_capability?: 'AVAILABLE' | 'NOT_AVAILABLE'
}

export interface FreightCostCarrierPerformanceRowVM {
  carrier_company_id: string
  carrier_name: string
  order_count: number
  planned_total: DecimalString | null
  current_actual_total: DecimalString | null
  final_actual_total: DecimalString | null
  current_variance_total: DecimalString | null
  currency_code: string
}

export interface FreightCostCarrierPerformanceResponse {
  items: FreightCostCarrierPerformanceRowVM[]
  currency_code: string
}

export interface FreightCostLanePerformanceRowVM {
  origin_location_code: string
  destination_location_code: string
  lane_label: string
  order_count: number
  planned_total: DecimalString | null
  current_actual_total: DecimalString | null
  final_actual_total: DecimalString | null
  current_variance_total: DecimalString | null
  currency_code: string
}

export interface FreightCostLanePerformanceResponse {
  items: FreightCostLanePerformanceRowVM[]
  currency_code: string
  data_capability?: 'AVAILABLE' | 'NOT_AVAILABLE'
}

export interface FreightCostVarianceDetailDTO {
  transport_order_id: string
  variance_drivers: FreightCostVarianceDriverVM[]
  reconciliation_findings: FreightCostReconciliationFindingVM[]
}

export interface FreightCostListQuery {
  company_id?: string
  from?: string
  to?: string
  date_dimension?: FreightCostDateDimension
  currency?: string
  carrier_id?: string
  origin_location_code?: string
  destination_location_code?: string
  order_status?: string
  settlement_status?: string
  variance_state?: string
  reconciliation_state?: FreightCostReconciliationStatus | ''
  q?: string
  limit?: number
  offset?: number
}

export interface FreightCostSummaryQuery {
  company_id?: string
  from?: string
  to?: string
  date_dimension?: FreightCostDateDimension
  currency?: string
  carrier_id?: string
}

export const FREIGHT_COST_ANALYTICS_DATA_QUALITIES = [
  'AVAILABLE',
  'PARTIAL',
  'NOT_AVAILABLE',
  'INSUFFICIENT_SAMPLE',
  'STALE',
  'MIXED_CURRENCY',
] as const

export type FreightCostAnalyticsDataQuality = (typeof FREIGHT_COST_ANALYTICS_DATA_QUALITIES)[number]

export const FREIGHT_COST_OPPORTUNITY_TYPES = [
  'LANE_COST_OUTLIER',
  'CARRIER_COST_OUTLIER',
  'COST_ABOVE_LANE_MEDIAN',
  'HIGH_ACCESSORIAL_RATE',
  'REPEATED_VARIANCE',
  'CLASSIFICATION_ANOMALY',
] as const

export type FreightCostOpportunityType = (typeof FREIGHT_COST_OPPORTUNITY_TYPES)[number]

export const FREIGHT_COST_OPPORTUNITY_SCOPES = [
  'LANE',
  'CARRIER',
  'ORDER',
  'ACCESSORIAL',
] as const

export type FreightCostOpportunityScope = (typeof FREIGHT_COST_OPPORTUNITY_SCOPES)[number]

/** Analytics wire-format money — amount is a decimal string or null when unavailable. */
export interface FreightCostAnalyticsMoneyDTO {
  amount: DecimalString | null
  currency_code: string
}

export interface FreightCostAnalyticsFreshnessDTO {
  calculated_at?: string
  data_through?: string
  projection_version: number
  benchmark_cost_basis?: string
}

export interface FreightCostAnalyticsPeriodDTO {
  from: string
  to: string
  date_dimension: string
}

export interface FreightCostAnalyticsOverviewSummaryDTO {
  planned_total: DecimalString | null
  current_actual_total: DecimalString | null
  final_actual_total: DecimalString | null
  current_variance_total?: DecimalString | null
  final_variance_total?: DecimalString | null
  reconciliation_mismatch_count?: number
  order_count: number
}

export interface FreightCostAnalyticsOverviewTopLaneDTO {
  lane_key: string
  lane_label: string
  order_count: number
  spend_total: FreightCostAnalyticsMoneyDTO
}

export interface FreightCostAnalyticsOverviewAccessorialDTO {
  total_amount: FreightCostAnalyticsMoneyDTO
  order_count: number
}

export interface FreightCostAnalyticsOpportunityEvidenceDTO {
  observed_cost?: DecimalString | null
  baseline_cost?: DecimalString | null
  potential_delta?: DecimalString | null
  sample_size?: number
  currency_code?: string
  lane_key?: string
  cohort_median?: DecimalString | null
  cohort_p90?: DecimalString | null
  carrier_company_id?: string
  reason_code?: string
  occurrence_count?: number
  accessorial_rate?: DecimalString | null
  baseline_p75_rate?: DecimalString | null
}

export interface FreightCostAnalyticsOpportunityItemDTO {
  opportunity_id: string
  type: FreightCostOpportunityType | string
  scope: FreightCostOpportunityScope | string
  entity_key: string
  observed_value: FreightCostAnalyticsMoneyDTO
  baseline_value: FreightCostAnalyticsMoneyDTO
  estimated_delta: FreightCostAnalyticsMoneyDTO
  currency_code: string
  sample_size: number
  evidence: FreightCostAnalyticsOpportunityEvidenceDTO
  data_quality: FreightCostAnalyticsDataQuality | string
  calculated_at: string
  rule_version: number
}

export interface FreightCostAnalyticsOverviewOpportunitySummaryDTO {
  count: number
  top_items: FreightCostAnalyticsOpportunityItemDTO[]
}

export interface FreightCostAnalyticsOverviewDTO {
  currency_code: string
  period: FreightCostAnalyticsPeriodDTO
  data_quality: FreightCostAnalyticsDataQuality | string
  mixed_currency: boolean
  freshness: FreightCostAnalyticsFreshnessDTO
  summary?: FreightCostAnalyticsOverviewSummaryDTO
  top_lanes?: FreightCostAnalyticsOverviewTopLaneDTO[]
  accessorial?: FreightCostAnalyticsOverviewAccessorialDTO
  opportunities?: FreightCostAnalyticsOverviewOpportunitySummaryDTO
}

export interface FreightCostAnalyticsBenchmarkDTO {
  sample_size: number
  mean: FreightCostAnalyticsMoneyDTO
  median: FreightCostAnalyticsMoneyDTO
  p25: FreightCostAnalyticsMoneyDTO
  p75: FreightCostAnalyticsMoneyDTO
  p90: FreightCostAnalyticsMoneyDTO
  min: FreightCostAnalyticsMoneyDTO
  max: FreightCostAnalyticsMoneyDTO
  data_quality: FreightCostAnalyticsDataQuality | string
}

export interface FreightCostAnalyticsLaneItemDTO {
  lane_key: string
  lane_label: string
  origin_country: string
  origin_city: string
  destination_country: string
  destination_city: string
  transport_mode: string
  equipment_type: string
  order_count: number
  carrier_count: number
  planned_total: FreightCostAnalyticsMoneyDTO
  current_actual_total: FreightCostAnalyticsMoneyDTO
  final_actual_total: FreightCostAnalyticsMoneyDTO
  variance_total: FreightCostAnalyticsMoneyDTO
  benchmark: FreightCostAnalyticsBenchmarkDTO
}

export interface FreightCostAnalyticsCarrierItemDTO {
  carrier_company_id: string
  carrier_name: string
  order_count: number
  lane_count: number
  planned_total: FreightCostAnalyticsMoneyDTO
  current_actual_total: FreightCostAnalyticsMoneyDTO
  final_actual_total: FreightCostAnalyticsMoneyDTO
  variance_total: FreightCostAnalyticsMoneyDTO
  comparable_order_count: number
  lane_normalized_delta: FreightCostAnalyticsMoneyDTO
  data_quality: FreightCostAnalyticsDataQuality | string
}

export interface FreightCostAnalyticsAccessorialItemDTO {
  normalized_category: FreightCostAccessorialCategory | string
  total_amount: FreightCostAnalyticsMoneyDTO
  order_count: number
  line_count: number
  share_of_spend: DecimalString | null
  accessorial_order_rate: DecimalString | null
  data_quality: FreightCostAnalyticsDataQuality | string
}

export interface FreightCostAnalyticsListEnvelope<TItem> {
  currency_code: string
  period: FreightCostAnalyticsPeriodDTO
  data_quality: FreightCostAnalyticsDataQuality | string
  mixed_currency: boolean
  freshness: FreightCostAnalyticsFreshnessDTO
  items: TItem[]
  total: number
  limit: number
  offset: number
}

export type FreightCostAnalyticsLanesResponse = FreightCostAnalyticsListEnvelope<FreightCostAnalyticsLaneItemDTO>
export type FreightCostAnalyticsCarriersResponse = FreightCostAnalyticsListEnvelope<FreightCostAnalyticsCarrierItemDTO>
export type FreightCostAnalyticsAccessorialsResponse = FreightCostAnalyticsListEnvelope<FreightCostAnalyticsAccessorialItemDTO>
export type FreightCostAnalyticsOpportunitiesResponse = FreightCostAnalyticsListEnvelope<FreightCostAnalyticsOpportunityItemDTO>

export interface FreightCostAnalyticsQuery {
  company_id?: string
  from?: string
  to?: string
  date_dimension?: string
  currency?: string
  limit?: number
  offset?: number
  sort?: string
  transport_mode?: string
  equipment_type?: string
  carrier_company_id?: string
  lane_key?: string
}
