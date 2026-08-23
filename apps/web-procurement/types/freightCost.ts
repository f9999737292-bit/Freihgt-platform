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
