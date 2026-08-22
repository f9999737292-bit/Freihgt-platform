import type {
  FreightCostAccessorialCategory,
  FreightCostActor,
  FreightCostDetailVM,
  FreightCostFinancialFinality,
  FreightCostListQuery,
  FreightCostOrderRowVM,
  FreightCostReconciliationStatus,
  FreightCostSummaryAggregateDTO,
  FreightCostSummaryDTO,
  FreightCostSummaryKpisDTO,
} from '~/types/freightCost'
import { ApiError } from '~/utils/apiClient'
import { isApiUnavailableError, shouldShowNotFound } from '~/utils/apiError'
import { paginateItems } from '~/utils/contractRate'

export type FreightCostWorkspaceViewState =
  | 'loading'
  | 'missing_company'
  | 'forbidden'
  | 'live_unavailable'
  | 'backend_unavailable'
  | 'empty'
  | 'mixed_currency'
  | 'ready'

export type FreightCostDetailViewState =
  | 'loading'
  | 'not_found'
  | 'forbidden'
  | 'live_unavailable'
  | 'backend_unavailable'
  | 'ready'

export type FreightCostOverviewKpiKey =
  | 'planned_total'
  | 'accrued_total'
  | 'forecast_exposure_total'
  | 'current_actual_total'
  | 'final_actual_total'
  | 'current_variance_total'
  | 'final_variance_total'
  | 'reconciliation_mismatch_count'

export interface FreightCostNavItem {
  key: string
  to: string
  labelKey: string
  buyerOnly?: boolean
}

export interface FreightCostTableColumn {
  key: string
  labelKey: string
  buyerOnly?: boolean
}

export interface FreightCostFilterState {
  from: string
  to: string
  date_dimension: string
  currency: string
  carrier_id: string
  origin_location_code: string
  destination_location_code: string
  order_status: string
  settlement_status: string
  variance_state: string
  reconciliation_state: FreightCostReconciliationStatus | ''
  q: string
}

export interface FreightCostDetailSection {
  key: string
  labelKey: string
  buyerOnly?: boolean
}

export const FREIGHT_COST_DEFAULT_PAGE_SIZE = 20

export const FREIGHT_COST_OVERVIEW_KPI_KEYS: FreightCostOverviewKpiKey[] = [
  'planned_total',
  'accrued_total',
  'forecast_exposure_total',
  'current_actual_total',
  'final_actual_total',
  'current_variance_total',
  'final_variance_total',
  'reconciliation_mismatch_count',
]

const BUYER_ONLY_KPI_KEYS = new Set<FreightCostOverviewKpiKey>([
  'accrued_total',
  'forecast_exposure_total',
  'current_variance_total',
  'final_variance_total',
  'reconciliation_mismatch_count',
])

const BUYER_ONLY_SUMMARY_FIELDS = new Set<keyof FreightCostSummaryDTO>([
  'accrued_amount',
  'forecast_exposure',
  'current_variance_amount',
  'final_variance_amount',
  'current_variance_percent',
  'final_variance_percent',
])

const FORBIDDEN_KPI_KEYS = new Set(['settled_unpaid_exposure'])

export function createDefaultFreightCostFilters(): FreightCostFilterState {
  return {
    from: '',
    to: '',
    date_dimension: '',
    currency: '',
    carrier_id: '',
    origin_location_code: '',
    destination_location_code: '',
    order_status: '',
    settlement_status: '',
    variance_state: '',
    reconciliation_state: '',
    q: '',
  }
}

export function buildFreightCostNavItems(actor: FreightCostActor): FreightCostNavItem[] {
  const items: FreightCostNavItem[] = [
    { key: 'overview', to: '/freight-costs', labelKey: 'freightCosts.nav.overview' },
    { key: 'planned-vs-actual', to: '/freight-costs/planned-vs-actual', labelKey: 'freightCosts.nav.plannedVsActual' },
    { key: 'shipments', to: '/freight-costs/shipments', labelKey: 'freightCosts.nav.shipments' },
    { key: 'variance', to: '/freight-costs/variance', labelKey: 'freightCosts.nav.variance', buyerOnly: true },
    { key: 'accessorials', to: '/freight-costs/accessorials', labelKey: 'freightCosts.nav.accessorials', buyerOnly: true },
    { key: 'carriers', to: '/freight-costs/carriers', labelKey: 'freightCosts.nav.carriers' },
    { key: 'lanes', to: '/freight-costs/lanes', labelKey: 'freightCosts.nav.lanes' },
  ]
  if (actor === 'CARRIER') {
    return items.filter((item) => !item.buyerOnly)
  }
  return items
}

export function getPlannedVsActualColumns(actor: FreightCostActor): FreightCostTableColumn[] {
  const columns: FreightCostTableColumn[] = [
    { key: 'order_reference', labelKey: 'freightCosts.columns.orderReference' },
    { key: 'carrier_name', labelKey: 'freightCosts.columns.carrier' },
    { key: 'planned_amount', labelKey: 'freightCosts.columns.planned' },
    { key: 'accrued_amount', labelKey: 'freightCosts.columns.accrued', buyerOnly: true },
    { key: 'forecast_exposure', labelKey: 'freightCosts.kpi.plannedPlusProposedExposure', buyerOnly: true },
    { key: 'current_actual_amount', labelKey: 'freightCosts.columns.currentActual' },
    { key: 'final_actual_amount', labelKey: 'freightCosts.columns.finalActual' },
    { key: 'current_variance_amount', labelKey: 'freightCosts.columns.currentVariance', buyerOnly: true },
    { key: 'final_variance_amount', labelKey: 'freightCosts.columns.finalVariance', buyerOnly: true },
    { key: 'currency_code', labelKey: 'freightCosts.columns.currency' },
    { key: 'financial_finality', labelKey: 'freightCosts.columns.finality' },
    { key: 'billing_reconciliation_status', labelKey: 'freightCosts.columns.reconciliation' },
    { key: 'cost_updated_at', labelKey: 'freightCosts.columns.updatedAt' },
  ]
  if (actor === 'CARRIER') {
    return columns.filter((column) => !column.buyerOnly)
  }
  return columns
}

export function getFreightCostDetailSections(actor: FreightCostActor): FreightCostDetailSection[] {
  const sections: FreightCostDetailSection[] = [
    { key: 'summary', labelKey: 'freightCosts.detail.summary' },
    { key: 'planned_snapshot', labelKey: 'freightCosts.detail.plannedSnapshot' },
    { key: 'accrual_breakdown', labelKey: 'freightCosts.detail.accrualBreakdown', buyerOnly: true },
    { key: 'forecast_exposure', labelKey: 'freightCosts.kpi.plannedPlusProposedExposure', buyerOnly: true },
    { key: 'actual_settlement', labelKey: 'freightCosts.detail.actualSettlement' },
    { key: 'variance', labelKey: 'freightCosts.detail.variance', buyerOnly: true },
    { key: 'variance_drivers', labelKey: 'freightCosts.detail.varianceDrivers', buyerOnly: true },
    { key: 'reconciliation', labelKey: 'freightCosts.detail.reconciliation', buyerOnly: true },
    { key: 'provenance', labelKey: 'freightCosts.detail.provenance' },
  ]
  if (actor === 'CARRIER') {
    return sections.filter((section) => !section.buyerOnly)
  }
  return sections
}

export function shouldShowFreightCostField(
  field: keyof FreightCostSummaryDTO | keyof FreightCostOrderRowVM,
  actor: FreightCostActor,
): boolean {
  if (actor === 'BUYER') return true
  if (BUYER_ONLY_SUMMARY_FIELDS.has(field as keyof FreightCostSummaryDTO)) return false
  if (field === 'accrued_amount'
    || field === 'forecast_exposure'
    || field === 'current_variance_amount'
    || field === 'final_variance_amount') {
    return false
  }
  return true
}

export function maskFreightCostSummaryForCarrier(
  summary: FreightCostSummaryDTO,
): FreightCostSummaryDTO {
  return {
    ...summary,
    accrued_amount: null,
    forecast_exposure: null,
    forecast_source_status: summary.forecast_source_status,
    current_variance_amount: null,
    final_variance_amount: null,
    current_variance_percent: null,
    final_variance_percent: null,
  }
}

export function maskFreightCostRowForCarrier(row: FreightCostOrderRowVM): FreightCostOrderRowVM {
  return {
    ...row,
    accrued_amount: null,
    forecast_exposure: null,
    current_variance_amount: null,
    final_variance_amount: null,
  }
}

export function maskFreightCostDetailForCarrier(detail: FreightCostDetailVM): FreightCostDetailVM {
  return {
    ...detail,
    summary: maskFreightCostSummaryForCarrier(detail.summary),
    variance_drivers: [],
    reconciliation_findings: [],
  }
}

export function mapFreightCostSummaryToRowVM(
  summary: FreightCostSummaryDTO,
  extras: { order_reference: string; carrier_name: string },
): FreightCostOrderRowVM {
  return {
    transport_order_id: summary.transport_order_id,
    shipment_id: summary.shipment_id,
    order_reference: extras.order_reference,
    carrier_company_id: summary.carrier_company_id,
    carrier_name: extras.carrier_name,
    planned_amount: summary.planned_amount,
    accrued_amount: summary.accrued_amount,
    forecast_exposure: summary.forecast_exposure,
    current_actual_amount: summary.current_actual_amount,
    final_actual_amount: summary.final_actual_amount,
    current_variance_amount: summary.current_variance_amount,
    final_variance_amount: summary.final_variance_amount,
    currency_code: summary.currency_code,
    financial_finality: summary.financial_finality,
    billing_reconciliation_status: summary.billing_reconciliation_status,
    availability_summary: summary.availability_reasons,
    cost_updated_at: summary.cost_updated_at,
  }
}

export function getOverviewKpiKeysForActor(actor: FreightCostActor): FreightCostOverviewKpiKey[] {
  return FREIGHT_COST_OVERVIEW_KPI_KEYS.filter((key) => {
    if (FORBIDDEN_KPI_KEYS.has(key)) return false
    if (actor === 'CARRIER' && BUYER_ONLY_KPI_KEYS.has(key)) return false
    return true
  })
}

export function getOverviewKpiLabelKey(kpiKey: FreightCostOverviewKpiKey): string {
  if (kpiKey === 'forecast_exposure_total') {
    return 'freightCosts.kpi.plannedPlusProposedExposure'
  }
  return `freightCosts.kpi.${kpiKey}`
}

export function getOverviewKpiValue(
  aggregate: FreightCostSummaryAggregateDTO | null,
  kpiKey: FreightCostOverviewKpiKey,
): string | number | null {
  if (!aggregate) return null
  if (aggregate.mixed_currency && kpiKey !== 'reconciliation_mismatch_count') {
    return null
  }
  return aggregate.kpis[kpiKey as keyof FreightCostSummaryKpisDTO] ?? null
}

export function resolveFreightCostListViewState(input: {
  loading: boolean
  missingCompany: boolean
  forbidden: boolean
  liveUnavailable: boolean
  apiUnavailable: boolean
  itemCount: number
}): FreightCostWorkspaceViewState {
  if (input.loading) return 'loading'
  if (input.missingCompany) return 'missing_company'
  if (input.forbidden) return 'forbidden'
  if (input.liveUnavailable) return 'live_unavailable'
  if (input.apiUnavailable) return 'backend_unavailable'
  if (input.itemCount === 0) return 'empty'
  return 'ready'
}

export function resolveFreightCostOverviewViewState(input: {
  loading: boolean
  missingCompany: boolean
  forbidden: boolean
  liveUnavailable: boolean
  apiUnavailable: boolean
  mixedCurrency: boolean
  hasAggregate: boolean
}): FreightCostWorkspaceViewState {
  if (input.loading) return 'loading'
  if (input.missingCompany) return 'missing_company'
  if (input.forbidden) return 'forbidden'
  if (input.liveUnavailable) return 'live_unavailable'
  if (input.apiUnavailable) return 'backend_unavailable'
  if (input.mixedCurrency) return 'mixed_currency'
  if (!input.hasAggregate) return 'empty'
  return 'ready'
}

export function resolveFreightCostDetailViewState(input: {
  loading: boolean
  notFound: boolean
  forbidden: boolean
  liveUnavailable: boolean
  apiUnavailable: boolean
  hasDetail: boolean
}): FreightCostDetailViewState {
  if (input.loading) return 'loading'
  if (input.notFound) return 'not_found'
  if (input.forbidden) return 'forbidden'
  if (input.liveUnavailable) return 'live_unavailable'
  if (input.apiUnavailable) return 'backend_unavailable'
  if (input.hasDetail) return 'ready'
  return 'loading'
}

export function resolveFreightCostDetailError(error: unknown): 'forbidden' | 'not_found' | 'backend_unavailable' | 'live_unavailable' | 'error' {
  if (error instanceof ApiError) {
    if (error.status === 403) return 'forbidden'
    if (error.code === 'FREIGHT_COST_LIVE_UNAVAILABLE') return 'live_unavailable'
  }
  if (shouldShowNotFound(error)) return 'not_found'
  if (isApiUnavailableError(error)) return 'backend_unavailable'
  return 'error'
}

export function buildFreightCostFilterQuery(
  companyId: string,
  filters: FreightCostFilterState,
  pagination?: { limit: number; offset: number },
): FreightCostListQuery {
  const query: FreightCostListQuery = { company_id: companyId }
  if (filters.from.trim()) query.from = filters.from.trim()
  if (filters.to.trim()) query.to = filters.to.trim()
  if (filters.date_dimension.trim()) {
    query.date_dimension = filters.date_dimension as FreightCostListQuery['date_dimension']
  }
  if (filters.currency.trim()) query.currency = filters.currency.trim()
  if (filters.carrier_id.trim()) query.carrier_id = filters.carrier_id.trim()
  if (filters.origin_location_code.trim()) query.origin_location_code = filters.origin_location_code.trim()
  if (filters.destination_location_code.trim()) {
    query.destination_location_code = filters.destination_location_code.trim()
  }
  if (filters.order_status.trim()) query.order_status = filters.order_status.trim()
  if (filters.settlement_status.trim()) query.settlement_status = filters.settlement_status.trim()
  if (filters.variance_state.trim()) query.variance_state = filters.variance_state.trim()
  if (filters.reconciliation_state) query.reconciliation_state = filters.reconciliation_state
  if (filters.q.trim()) query.q = filters.q.trim()
  if (pagination) {
    query.limit = pagination.limit
    query.offset = pagination.offset
  }
  return query
}

export function activeFreightCostFilterChips(filters: FreightCostFilterState): string[] {
  const chips: string[] = []
  if (filters.from) chips.push(`from:${filters.from}`)
  if (filters.to) chips.push(`to:${filters.to}`)
  if (filters.date_dimension) chips.push(`date_dimension:${filters.date_dimension}`)
  if (filters.currency) chips.push(`currency:${filters.currency}`)
  if (filters.carrier_id) chips.push(`carrier_id:${filters.carrier_id}`)
  if (filters.origin_location_code) chips.push(`origin:${filters.origin_location_code}`)
  if (filters.destination_location_code) chips.push(`destination:${filters.destination_location_code}`)
  if (filters.order_status) chips.push(`order_status:${filters.order_status}`)
  if (filters.settlement_status) chips.push(`settlement_status:${filters.settlement_status}`)
  if (filters.variance_state) chips.push(`variance_state:${filters.variance_state}`)
  if (filters.reconciliation_state) chips.push(`reconciliation:${filters.reconciliation_state}`)
  if (filters.q) chips.push(`q:${filters.q}`)
  return chips
}

export function sortFreightCostRowsByUpdatedAt(rows: FreightCostOrderRowVM[]): FreightCostOrderRowVM[] {
  return [...rows].sort((left, right) => {
    const byDate = right.cost_updated_at.localeCompare(left.cost_updated_at)
    if (byDate !== 0) return byDate
    return right.transport_order_id.localeCompare(left.transport_order_id)
  })
}

export function paginateFreightCostRows(
  rows: FreightCostOrderRowVM[],
  limit: number,
  offset: number,
) {
  return paginateItems(rows, limit, offset)
}

export function finalityLabelKey(finality: FreightCostFinancialFinality): string {
  return `freightCosts.finality.${finality}`
}

export function reconciliationLabelKey(status: FreightCostReconciliationStatus | null): string {
  if (!status) return 'freightCosts.unavailable.money'
  return `freightCosts.reconciliation.${status}`
}

export function accessorialCategoryLabelKey(category: FreightCostAccessorialCategory): string {
  return `freightCosts.categories.${category}`
}

export function isFreightCostWorkspaceRoute(path: string): boolean {
  return path === '/freight-costs' || path.startsWith('/freight-costs/')
}

export function isFreightCostUnavailableRoute(path: string): boolean {
  return path === '/freight-costs/unavailable'
}

export function shouldRedirectFreightCostWorkspace(featureEnabled: boolean, path: string): boolean {
  if (featureEnabled) return false
  if (isFreightCostUnavailableRoute(path)) return false
  return isFreightCostWorkspaceRoute(path)
}

export function parseFreightCostFeatureFlag(raw: string | undefined): boolean {
  return raw === 'true'
}

export function hasSettledUnpaidExposureKpi(keys: string[]): boolean {
  return keys.some((key) => FORBIDDEN_KPI_KEYS.has(key))
}

export function crossTaxBasisSubtractionAllowed(): boolean {
  return false
}
