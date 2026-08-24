import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import type {
  FreightCostAnalyticsOpportunityItemDTO,
  FreightCostAnalyticsOverviewDTO,
} from '~/types/freightCost'
import enFreightCosts from '~/i18n/en-US/freightCosts.json'
import ruFreightCosts from '~/i18n/ru-RU/freightCosts.json'
import zhFreightCosts from '~/i18n/zh-CN/freightCosts.json'
import {
  FREIGHT_COST_PUBLIC_API_PATHS,
  createMockFreightCostDataSource,
  createProductionFreightCostDataSource,
} from '~/utils/freightCostDataSource'
import {
  dataQualityLabelKey,
  formatAnalyticsMoney,
  formatCarrierAnalyticsName,
  formatOpportunityBaselineValue,
  formatOpportunityEstimatedDelta,
  formatOpportunityObservedValue,
  getOverviewSummaryKpiValue,
  opportunityTypeLabelKey,
  resolveFreightCostIntelligenceListViewState,
  resolveFreightCostIntelligenceOverviewViewState,
  shouldShowIntelligenceDataQualityBanner,
} from '~/utils/freightCostIntelligence'
import { shouldApplyFreightCostIntelligenceListLoad } from '~/composables/useFreightCostIntelligenceListLoad'
import { buildFreightCostNavItems } from '~/utils/freightCostWorkspace'

const CARRIER_UUID = 'a1b2c3d4-e5f6-4789-a012-3456789abcde'

function opportunity(overrides: Partial<FreightCostAnalyticsOpportunityItemDTO> = {}): FreightCostAnalyticsOpportunityItemDTO {
  return {
    opportunity_id: 'opp-1',
    type: 'LANE_COST_OUTLIER',
    scope: 'LANE',
    entity_key: 'RU:MOSCOW->RU:SPB|ROAD|TENT',
    observed_value: { amount: '45000.00', currency_code: 'RUB' },
    baseline_value: { amount: '38000.00', currency_code: 'RUB' },
    estimated_delta: { amount: '7000.00', currency_code: 'RUB' },
    currency_code: 'RUB',
    sample_size: 12,
    evidence: {
      observed_cost: '45000.00',
      baseline_cost: '38000.00',
      potential_delta: '7000.00',
      sample_size: 12,
      currency_code: 'RUB',
    },
    data_quality: 'AVAILABLE',
    calculated_at: '2026-08-23T12:00:00Z',
    rule_version: 1,
    ...overrides,
  }
}

function overview(overrides: Partial<FreightCostAnalyticsOverviewDTO> = {}): FreightCostAnalyticsOverviewDTO {
  return {
    currency_code: 'RUB',
    period: { from: '2026-05-01', to: '2026-08-01', date_dimension: 'COST_EFFECTIVE' },
    data_quality: 'AVAILABLE',
    mixed_currency: false,
    freshness: { projection_version: 3, calculated_at: '2026-08-23T12:00:00Z' },
    summary: {
      planned_total: '100000.00',
      current_actual_total: '95000.00',
      final_actual_total: '90000.00',
      current_variance_total: '-5000.00',
      final_variance_total: '-10000.00',
      reconciliation_mismatch_count: 2,
      order_count: 42,
    },
    top_lanes: [],
    ...overrides,
  }
}

function readSource(relativePath: string): string {
  return readFileSync(resolve(process.cwd(), relativePath), 'utf8')
}

describe('FC-D-INT intelligence navigation', () => {
  it('FC-D-INT-NAV-001 buyer nav includes opportunities tab', () => {
    const keys = buildFreightCostNavItems('BUYER').map((item) => item.key)
    expect(keys).toContain('opportunities')
  })

  it('FC-D-INT-NAV-002 carrier nav hides opportunities tab', () => {
    const keys = buildFreightCostNavItems('CARRIER').map((item) => item.key)
    expect(keys).not.toContain('opportunities')
  })
})

describe('FC-D-INT savings display', () => {
  it('FC-D-INT-SAV-001 estimated delta uses backend value not observed minus baseline', () => {
    const item = opportunity({
      observed_value: { amount: '45000.00', currency_code: 'RUB' },
      baseline_value: { amount: '38000.00', currency_code: 'RUB' },
      estimated_delta: { amount: '6500.00', currency_code: 'RUB' },
    })
    const formatted = formatOpportunityEstimatedDelta(item, 'en-US', 'Unavailable')
    expect(formatted).toContain('6')
    expect(formatted).toContain('500.00')
    expect(formatted).not.toContain('7,000.00')
  })

  it('FC-D-INT-SAV-002 intelligence utils use DTO money fields not evidence arithmetic', () => {
    const item = opportunity({
      observed_value: { amount: '45000.00', currency_code: 'RUB' },
      baseline_value: { amount: '38000.00', currency_code: 'RUB' },
      estimated_delta: { amount: '1234.56', currency_code: 'RUB' },
      evidence: {
        observed_cost: '99999.00',
        baseline_cost: '100.00',
        potential_delta: '99899.00',
      },
    })
    expect(formatOpportunityEstimatedDelta(item, 'en-US', 'Unavailable')).toBe('1,234.56 RUB')
    expect(formatOpportunityObservedValue(item, 'en-US', 'Unavailable')).toBe('45,000.00 RUB')
    expect(formatOpportunityBaselineValue(item, 'en-US', 'Unavailable')).toBe('38,000.00 RUB')
  })

  it('FC-D-INT-SAV-003 analytics composable documents backend-only savings', () => {
    const source = readSource('composables/useFreightCostAnalyticsApi.ts')
    expect(source).toContain('no client-side aggregation')
    expect(source).not.toMatch(/observed.*baseline|baseline.*observed/i)
  })
})

describe('FC-D-INT carrier display names', () => {
  it('FC-D-INT-CAR-001 UUID carrier name is not shown as display label', () => {
    expect(formatCarrierAnalyticsName(CARRIER_UUID, 'Reference unavailable')).toBe('Reference unavailable')
  })

  it('FC-D-INT-CAR-002 snapshot carrier name preserved', () => {
    expect(formatCarrierAnalyticsName('Carrier Alpha LLC', 'Reference unavailable')).toBe('Carrier Alpha LLC')
  })

  it('FC-D-INT-CAR-003 blank carrier name falls back to unavailable label', () => {
    expect(formatCarrierAnalyticsName('   ', 'Reference unavailable')).toBe('Reference unavailable')
  })
})

describe('FC-D-INT view states', () => {
  it('FC-D-INT-VST-001 list insufficient sample still ready when items exist', () => {
    expect(resolveFreightCostIntelligenceListViewState({
      loading: false,
      missingCompany: false,
      forbidden: false,
      liveUnavailable: false,
      apiUnavailable: false,
      dataQuality: 'INSUFFICIENT_SAMPLE',
      mixedCurrency: false,
      itemCount: 3,
    })).toBe('ready')
  })

  it('FC-D-INT-VST-002 list stale quality with items is ready', () => {
    expect(resolveFreightCostIntelligenceListViewState({
      loading: false,
      missingCompany: false,
      forbidden: false,
      liveUnavailable: false,
      apiUnavailable: false,
      dataQuality: 'STALE',
      mixedCurrency: false,
      itemCount: 1,
    })).toBe('ready')
  })

  it('FC-D-INT-VST-002b list not available with items still ready', () => {
    expect(resolveFreightCostIntelligenceListViewState({
      loading: false,
      missingCompany: false,
      forbidden: false,
      liveUnavailable: false,
      apiUnavailable: false,
      dataQuality: 'NOT_AVAILABLE',
      mixedCurrency: false,
      itemCount: 2,
    })).toBe('ready')
  })

  it('FC-D-INT-VST-002c list not available without items', () => {
    expect(resolveFreightCostIntelligenceListViewState({
      loading: false,
      missingCompany: false,
      forbidden: false,
      liveUnavailable: false,
      apiUnavailable: false,
      dataQuality: 'NOT_AVAILABLE',
      mixedCurrency: false,
      itemCount: 0,
    })).toBe('not_available')
  })

  it('FC-D-INT-VST-002d list api unavailable with items still ready', () => {
    expect(resolveFreightCostIntelligenceListViewState({
      loading: false,
      missingCompany: false,
      forbidden: false,
      liveUnavailable: false,
      apiUnavailable: true,
      dataQuality: 'AVAILABLE',
      mixedCurrency: false,
      itemCount: 1,
    })).toBe('ready')
  })

  it('FC-D-INT-VST-003 list mixed currency with no items', () => {
    expect(resolveFreightCostIntelligenceListViewState({
      loading: false,
      missingCompany: false,
      forbidden: false,
      liveUnavailable: false,
      apiUnavailable: false,
      dataQuality: 'MIXED_CURRENCY',
      mixedCurrency: true,
      itemCount: 0,
    })).toBe('mixed_currency')
  })

  it('FC-D-INT-VST-004 overview mixed currency without summary', () => {
    expect(resolveFreightCostIntelligenceOverviewViewState({
      loading: false,
      missingCompany: false,
      forbidden: false,
      liveUnavailable: false,
      apiUnavailable: false,
      overview: overview({ mixed_currency: true, summary: undefined }),
    })).toBe('mixed_currency')
  })

  it('FC-D-INT-VST-005 overview not available when data_quality is NOT_AVAILABLE', () => {
    expect(resolveFreightCostIntelligenceOverviewViewState({
      loading: false,
      missingCompany: false,
      forbidden: false,
      liveUnavailable: false,
      apiUnavailable: false,
      overview: overview({ data_quality: 'NOT_AVAILABLE', summary: undefined }),
    })).toBe('not_available')
  })

  it('FC-D-INT-VST-006 data quality banner shown for stale and insufficient sample', () => {
    expect(shouldShowIntelligenceDataQualityBanner('STALE', false)).toBe(true)
    expect(shouldShowIntelligenceDataQualityBanner('INSUFFICIENT_SAMPLE', false)).toBe(true)
    expect(shouldShowIntelligenceDataQualityBanner('AVAILABLE', false)).toBe(false)
    expect(shouldShowIntelligenceDataQualityBanner('AVAILABLE', true)).toBe(true)
  })
})

describe('FC-D-INT list load lifecycle', () => {
  it('FC-D-INT-LOAD-001 stale list response is ignored', () => {
    expect(shouldApplyFreightCostIntelligenceListLoad(1, 2, { items: [{ lane_key: 'a' }] })).toBe(false)
    expect(shouldApplyFreightCostIntelligenceListLoad(2, 2, { items: [{ lane_key: 'a' }] })).toBe(true)
  })

  it('FC-D-INT-LOAD-002 null list response is ignored', () => {
    expect(shouldApplyFreightCostIntelligenceListLoad(1, 1, null)).toBe(false)
  })

  it('FC-D-INT-LOAD-003 route watcher restores session and loads immediately', () => {
    const source = readSource('composables/useFreightCostIntelligenceRouteQuery.ts')
    expect(source).toContain('ensureSessionAndReload')
    expect(source).toContain('immediate: true')
    expect(source).toContain('restoreSession')
    expect(source).toContain('restoreTenant')
  })

  it('FC-D-INT-LOAD-004 list pages load on mount and watch route changes without immediate', () => {
    const lanes = readSource('pages/freight-costs/lanes/index.vue')
    expect(lanes).toContain('onMounted')
    expect(lanes).toContain('watch(')
    expect(lanes).not.toContain('{ immediate: true }')
    expect(lanes).toContain('FreightCostIntelligenceLaneTable')
    expect(lanes).not.toContain('<ClientOnly>')
  })

  it('FC-D-INT-LOAD-005 page context restores tenant before list fetch', () => {
    const source = readSource('composables/useFreightCostPageContext.ts')
    expect(source).toContain('restoreSession')
    expect(source).toContain('restoreTenant')
  })
})

describe('FC-D-INT money and KPI helpers', () => {
  it('FC-D-INT-MON-001 mixed currency nulls overview money KPIs', () => {
    expect(getOverviewSummaryKpiValue(overview().summary, 'planned_total', true)).toBeNull()
    expect(getOverviewSummaryKpiValue(overview().summary, 'order_count', true)).toBe(42)
  })

  it('FC-D-INT-MON-002 format analytics money uses decimal string amount', () => {
    expect(formatAnalyticsMoney({ amount: '1500.00', currency_code: 'RUB' }, 'en-US', 'Unavailable'))
      .toBe('1,500.00 RUB')
  })

  it('FC-D-INT-MON-003 null analytics amount is unavailable', () => {
    expect(formatAnalyticsMoney({ amount: null, currency_code: 'RUB' }, 'en-US', 'Unavailable')).toBe('Unavailable')
  })
})

describe('FC-D-INT i18n', () => {
  it('FC-D-INT-I18N-001 EN opportunity type label exists', () => {
    expect(enFreightCosts.freightCosts.intelligence.opportunityTypes.LANE_COST_OUTLIER).toBeTruthy()
    expect(opportunityTypeLabelKey('HIGH_ACCESSORIAL_RATE')).toBe('freightCosts.intelligence.opportunityTypes.HIGH_ACCESSORIAL_RATE')
  })

  it('FC-D-INT-I18N-002 RU data quality insufficient sample label', () => {
    expect(ruFreightCosts.freightCosts.intelligence.dataQuality.INSUFFICIENT_SAMPLE).toContain('выборк')
  })

  it('FC-D-INT-I18N-003 ZH stale data quality label', () => {
    expect(zhFreightCosts.freightCosts.intelligence.dataQuality.STALE).toBeTruthy()
    expect(dataQualityLabelKey('STALE')).toBe('freightCosts.intelligence.dataQuality.STALE')
  })

  it('FC-D-INT-I18N-004 EN backend savings hint frozen', () => {
    expect(enFreightCosts.freightCosts.intelligence.hints.backendSavingsOnly).toContain('server')
  })
})

describe('FC-D-INT data source', () => {
  it('FC-D-INT-DS-001 production paths include analytics routes', () => {
    expect(FREIGHT_COST_PUBLIC_API_PATHS).toContain('/api/v1/freight-costs/analytics/overview')
    expect(FREIGHT_COST_PUBLIC_API_PATHS).toContain('/api/v1/freight-costs/opportunities')
  })

  it('FC-D-INT-DS-002 mock adapter exposes analytics methods', async () => {
    const mock = createMockFreightCostDataSource({
      getAnalyticsOverview: async () => overview(),
    })
    const result = await mock.getAnalyticsOverview({ company_id: 'buyer-1' })
    expect(result.summary?.order_count).toBe(42)
  })

  it('FC-D-INT-DS-003 production data source includes analytics overview endpoint', () => {
    const source = readSource('utils/freightCostDataSource.ts')
    expect(source).toContain('/api/v1/freight-costs/analytics/overview')
    expect(createProductionFreightCostDataSource().getAnalyticsLanes).toBeTypeOf('function')
  })
})
