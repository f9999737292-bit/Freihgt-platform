import type {
  FreightCostAnalyticsDataQuality,
  FreightCostAnalyticsMoneyDTO,
  FreightCostAnalyticsOverviewDTO,
  FreightCostAnalyticsOverviewSummaryDTO,
  FreightCostAnalyticsOpportunityItemDTO,
} from '~/types/freightCost'
import { formatDecimalMoney, formatDecimalPercent, isNullMoney } from '~/utils/freightCostMoney'
import { formatFreightCostDisplayLabel } from '~/utils/freightCostWorkspace'

export type FreightCostIntelligenceViewState =
  | 'loading'
  | 'missing_company'
  | 'forbidden'
  | 'live_unavailable'
  | 'backend_unavailable'
  | 'not_available'
  | 'empty'
  | 'mixed_currency'
  | 'ready'

export interface FreightCostIntelligenceListInput {
  loading: boolean
  missingCompany: boolean
  forbidden: boolean
  liveUnavailable: boolean
  apiUnavailable: boolean
  dataQuality?: FreightCostAnalyticsDataQuality | string | null
  mixedCurrency: boolean
  itemCount: number
}

export interface FreightCostIntelligenceOverviewInput {
  loading: boolean
  missingCompany: boolean
  forbidden: boolean
  liveUnavailable: boolean
  apiUnavailable: boolean
  overview: FreightCostAnalyticsOverviewDTO | null
}

const DATA_QUALITY_BANNER_QUALITIES = new Set<FreightCostAnalyticsDataQuality | string>([
  'PARTIAL',
  'INSUFFICIENT_SAMPLE',
  'STALE',
  'NOT_AVAILABLE',
  'MIXED_CURRENCY',
])

export function formatAnalyticsMoney(
  money: FreightCostAnalyticsMoneyDTO | null | undefined,
  locale: string,
  unavailableLabel: string,
  mixedCurrency = false,
): string {
  if (mixedCurrency) return unavailableLabel
  if (!money) return unavailableLabel
  if (isNullMoney(money.amount)) return unavailableLabel
  return formatDecimalMoney(money.amount, money.currency_code, locale)
}

export function formatAnalyticsRatio(
  ratio: string | null | undefined,
  locale: string,
  unavailableLabel: string,
): string {
  if (isNullMoney(ratio)) return unavailableLabel
  return formatDecimalPercent(ratio, locale)
}

export function formatCarrierAnalyticsName(name: string | null | undefined, unavailableLabel: string): string {
  const label = formatFreightCostDisplayLabel(name)
  return label || unavailableLabel
}

export function dataQualityLabelKey(quality: FreightCostAnalyticsDataQuality | string | null | undefined): string {
  const key = String(quality ?? 'NOT_AVAILABLE').toUpperCase()
  return `freightCosts.intelligence.dataQuality.${key}`
}

export function dataQualityTone(quality: FreightCostAnalyticsDataQuality | string | null | undefined): 'neutral' | 'success' | 'warning' | 'danger' | 'info' {
  switch (String(quality ?? '').toUpperCase()) {
    case 'AVAILABLE':
      return 'success'
    case 'PARTIAL':
    case 'STALE':
      return 'warning'
    case 'INSUFFICIENT_SAMPLE':
    case 'MIXED_CURRENCY':
      return 'info'
    case 'NOT_AVAILABLE':
      return 'danger'
    default:
      return 'neutral'
  }
}

export function opportunityTypeLabelKey(type: string): string {
  return `freightCosts.intelligence.opportunityTypes.${type}`
}

export function opportunityScopeLabelKey(scope: string): string {
  return `freightCosts.intelligence.opportunityScopes.${scope}`
}

export function shouldShowIntelligenceDataQualityBanner(
  quality: FreightCostAnalyticsDataQuality | string | null | undefined,
  mixedCurrency: boolean,
): boolean {
  if (mixedCurrency) return true
  return DATA_QUALITY_BANNER_QUALITIES.has(String(quality ?? '').toUpperCase())
}

export function resolveFreightCostIntelligenceListViewState(
  input: FreightCostIntelligenceListInput,
): FreightCostIntelligenceViewState {
  if (input.loading) return 'loading'
  if (input.missingCompany) return 'missing_company'
  if (input.forbidden) return 'forbidden'
  if (input.liveUnavailable) return 'live_unavailable'
  if (input.apiUnavailable) return 'backend_unavailable'
  if (String(input.dataQuality ?? '').toUpperCase() === 'NOT_AVAILABLE') return 'not_available'
  if (input.mixedCurrency && input.itemCount === 0) return 'mixed_currency'
  if (input.itemCount === 0) return 'empty'
  return 'ready'
}

export function resolveFreightCostIntelligenceOverviewViewState(
  input: FreightCostIntelligenceOverviewInput,
): FreightCostIntelligenceViewState {
  if (input.loading) return 'loading'
  if (input.missingCompany) return 'missing_company'
  if (input.forbidden) return 'forbidden'
  if (input.liveUnavailable) return 'live_unavailable'
  if (input.apiUnavailable) return 'backend_unavailable'
  if (!input.overview) return 'empty'
  if (String(input.overview.data_quality ?? '').toUpperCase() === 'NOT_AVAILABLE') return 'not_available'
  if (input.overview.mixed_currency && !input.overview.summary) return 'mixed_currency'
  if (!input.overview.summary && !input.overview.top_lanes?.length && !input.overview.opportunities?.count) {
    return 'empty'
  }
  return 'ready'
}

export function getOverviewSummaryKpiKeys(): Array<keyof FreightCostAnalyticsOverviewSummaryDTO> {
  return [
    'planned_total',
    'current_actual_total',
    'final_actual_total',
    'current_variance_total',
    'final_variance_total',
    'reconciliation_mismatch_count',
    'order_count',
  ]
}

export function getOverviewSummaryKpiValue(
  summary: FreightCostAnalyticsOverviewSummaryDTO | null | undefined,
  key: keyof FreightCostAnalyticsOverviewSummaryDTO,
  mixedCurrency: boolean,
): string | number | null {
  if (!summary) return null
  if (mixedCurrency && key !== 'reconciliation_mismatch_count' && key !== 'order_count') {
    return null
  }
  return summary[key] ?? null
}

export function formatOpportunityEstimatedDelta(
  item: FreightCostAnalyticsOpportunityItemDTO,
  locale: string,
  unavailableLabel: string,
): string {
  return formatAnalyticsMoney(item.estimated_delta, locale, unavailableLabel)
}

export function formatOpportunityObservedValue(
  item: FreightCostAnalyticsOpportunityItemDTO,
  locale: string,
  unavailableLabel: string,
): string {
  return formatAnalyticsMoney(item.observed_value, locale, unavailableLabel)
}

export function formatOpportunityBaselineValue(
  item: FreightCostAnalyticsOpportunityItemDTO,
  locale: string,
  unavailableLabel: string,
): string {
  return formatAnalyticsMoney(item.baseline_value, locale, unavailableLabel)
}
