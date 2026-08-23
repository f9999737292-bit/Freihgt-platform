import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import type {
  FreightCostDetailVM,
  FreightCostOrderRowVM,
  FreightCostSummaryAggregateDTO,
  FreightCostSummaryDTO,
} from '~/types/freightCost'
import { FREIGHT_COST_ACCESSORIAL_CATEGORIES } from '~/types/freightCost'
import { ApiError } from '~/utils/apiClient'
import {
  FREIGHT_COST_FORBIDDEN_BROWSER_HEADERS,
  FREIGHT_COST_FORBIDDEN_BROWSER_PATHS,
  createMockFreightCostDataSource,
  createProductionFreightCostDataSource,
  isFreightCostLiveUnavailableError,
} from '~/utils/freightCostDataSource'
import {
  formatDecimalMoney,
  formatDecimalPercent,
  isExplicitZeroMoney,
  isNullMoney,
  moneyAriaLabel,
} from '~/utils/freightCostMoney'
import {
  canReadFreightCostsForRoles,
  canSeeAccessorialAnalyticsNavForRoles,
  canSeeBuyerInternalFreightCostFieldsForRoles,
  canSeeVarianceAnalysisNavForRoles,
  isCarrierFreightCostReaderForRoles,
  resolveFreightCostActorFromRoles,
  shouldShowFreightCostsNav,
} from '~/utils/freightCostPermissions'
import {
  FREIGHT_COST_OVERVIEW_KPI_KEYS,
  activeFreightCostFilterChips,
  buildFreightCostFilterQuery,
  buildFreightCostNavItems,
  crossTaxBasisSubtractionAllowed,
  getFreightCostDetailSections,
  getOverviewKpiKeysForActor,
  getOverviewKpiLabelKey,
  getOverviewKpiValue,
  getPlannedVsActualColumns,
  hasSettledUnpaidExposureKpi,
  mapFreightCostSummaryToRowVM,
  maskFreightCostDetailForCarrier,
  maskFreightCostRowForCarrier,
  maskFreightCostSummaryForCarrier,
  paginateFreightCostRows,
  parseFreightCostFeatureFlag,
  reconciliationLabelKey,
  resolveFreightCostDetailError,
  resolveFreightCostDetailViewState,
  resolveFreightCostListViewState,
  resolveFreightCostOverviewViewState,
  shouldRedirectFreightCostWorkspace,
  shouldShowFreightCostField,
  sortFreightCostRowsByUpdatedAt,
} from '~/utils/freightCostWorkspace'
import enFreightCosts from '~/i18n/en-US/freightCosts.json'
import ruFreightCosts from '~/i18n/ru-RU/freightCosts.json'
import zhFreightCosts from '~/i18n/zh-CN/freightCosts.json'

function summary(overrides: Partial<FreightCostSummaryDTO> = {}): FreightCostSummaryDTO {
  return {
    transport_order_id: 'to-1',
    shipment_id: 'sh-1',
    buyer_company_id: 'buyer-1',
    carrier_company_id: 'carrier-1',
    currency_code: 'RUB',
    data_stage: 'ACCRUAL_COMPLETE',
    financial_finality: 'CURRENT_ACTUAL',
    sources_available: ['PLANNED', 'ACCRUAL'],
    planned_amount: '1000.00',
    accrued_amount: '1050.00',
    forecast_exposure: '1100.00',
    forecast_source_status: 'KNOWN',
    current_actual_amount: '1050.00',
    final_actual_amount: null,
    billing_register_amount: null,
    paid_amount: null,
    current_variance_amount: '50.00',
    final_variance_amount: null,
    current_variance_percent: '5.00',
    final_variance_percent: null,
    billing_reconciliation_status: 'MATCH',
    cost_updated_at: '2026-08-22T12:00:00Z',
    availability_reasons: [],
    ...overrides,
  }
}

function row(overrides: Partial<FreightCostOrderRowVM> = {}): FreightCostOrderRowVM {
  return {
    transport_order_id: 'to-1',
    shipment_id: 'sh-1',
    order_reference: 'ORD-1',
    carrier_company_id: 'carrier-1',
    carrier_name: 'Carrier A',
    planned_amount: '1000.00',
    accrued_amount: '1050.00',
    forecast_exposure: '1100.00',
    current_actual_amount: '1050.00',
    final_actual_amount: null,
    current_variance_amount: '50.00',
    final_variance_amount: null,
    currency_code: 'RUB',
    financial_finality: 'CURRENT_ACTUAL',
    billing_reconciliation_status: 'MATCH',
    availability_summary: [],
    cost_updated_at: '2026-08-22T12:00:00Z',
    ...overrides,
  }
}

function aggregate(overrides: Partial<FreightCostSummaryAggregateDTO> = {}): FreightCostSummaryAggregateDTO {
  return {
    currency_code: 'RUB',
    period: {
      from: '2026-01-01',
      to: '2026-08-22',
      date_dimension: 'TRANSPORT_ORDER_CREATED_AT',
    },
    kpis: {
      planned_total: '100000.00',
      accrued_total: '95000.00',
      forecast_exposure_total: '105000.00',
      pending_proposed_accessorial_total: '5000.00',
      current_actual_total: '90000.00',
      final_actual_total: '85000.00',
      current_variance_total: '-10000.00',
      final_variance_total: '-15000.00',
      reconciliation_mismatch_count: 3,
    },
    mixed_currency: false,
    ...overrides,
  }
}

function readSource(relativePath: string): string {
  return readFileSync(resolve(process.cwd(), relativePath), 'utf8')
}

describe('FC-D-NAV freight cost navigation', () => {
  it('FC-D-NAV-001 flag off hides nav link', () => {
    expect(shouldShowFreightCostsNav(false, ['PROCUREMENT_MANAGER'])).toBe(false)
  })

  it('FC-D-NAV-002 flag on shows nav for authorized buyer', () => {
    expect(shouldShowFreightCostsNav(true, ['PROCUREMENT_MANAGER'])).toBe(true)
  })

  it('FC-D-NAV-003 unavailable route excluded from workspace redirect', () => {
    expect(shouldRedirectFreightCostWorkspace(false, '/freight-costs/unavailable')).toBe(false)
  })

  it('FC-D-NAV-004 buyer nav includes variance and accessorials', () => {
    const keys = buildFreightCostNavItems('BUYER').map((item) => item.key)
    expect(keys).toContain('variance')
    expect(keys).toContain('accessorials')
  })

  it('FC-D-NAV-005 carrier nav hides buyer-only items', () => {
    const keys = buildFreightCostNavItems('CARRIER').map((item) => item.key)
    expect(keys).not.toContain('variance')
    expect(keys).not.toContain('accessorials')
  })

  it('FC-D-NAV-006 carrier reader resolves carrier actor', () => {
    expect(resolveFreightCostActorFromRoles(['CARRIER_ADMIN'])).toBe('CARRIER')
  })
})

describe('FC-D-FLAG feature flag', () => {
  it('FC-D-FLAG-001 default flag parsing is false', () => {
    expect(parseFreightCostFeatureFlag(undefined)).toBe(false)
    expect(parseFreightCostFeatureFlag('false')).toBe(false)
  })

  it('FC-D-FLAG-002 middleware redirect target when disabled', () => {
    expect(shouldRedirectFreightCostWorkspace(false, '/freight-costs')).toBe(true)
    expect(shouldRedirectFreightCostWorkspace(false, '/freight-costs/planned-vs-actual')).toBe(true)
  })

  it('FC-D-FLAG-003 env true parsing enables workspace routes', () => {
    expect(parseFreightCostFeatureFlag('true')).toBe(true)
    expect(shouldRedirectFreightCostWorkspace(true, '/freight-costs')).toBe(false)
  })
})

describe('FC-D-OVR overview KPIs', () => {
  it('FC-D-OVR-001 buyer overview exposes all frozen KPI keys', () => {
    expect(getOverviewKpiKeysForActor('BUYER')).toEqual(FREIGHT_COST_OVERVIEW_KPI_KEYS)
  })

  it('FC-D-OVR-002 null KPI value stays unavailable not zero', () => {
    const agg = aggregate({ kpis: { ...aggregate().kpis, planned_total: null } })
    expect(getOverviewKpiValue(agg, 'planned_total')).toBeNull()
    expect(isNullMoney(getOverviewKpiValue(agg, 'planned_total') as string | null)).toBe(true)
  })

  it('FC-D-OVR-003 mixed currency aggregate nulls money KPIs', () => {
    const agg = aggregate({ mixed_currency: true })
    expect(getOverviewKpiValue(agg, 'planned_total')).toBeNull()
    expect(getOverviewKpiValue(agg, 'reconciliation_mismatch_count')).toBe(3)
  })

  it('FC-D-OVR-004 reconciliation mismatch count preserved', () => {
    expect(getOverviewKpiValue(aggregate(), 'reconciliation_mismatch_count')).toBe(3)
  })

  it('FC-D-OVR-005 forecast KPI label uses plannedPlusProposedExposure', () => {
    expect(getOverviewKpiLabelKey('forecast_exposure_total')).toBe('freightCosts.kpi.plannedPlusProposedExposure')
  })

  it('FC-D-OVR-006 settled unpaid exposure KPI forbidden', () => {
    expect(hasSettledUnpaidExposureKpi(FREIGHT_COST_OVERVIEW_KPI_KEYS)).toBe(false)
  })

  it('FC-D-OVR-007 cross-tax-basis subtraction denied', () => {
    expect(crossTaxBasisSubtractionAllowed()).toBe(false)
  })

  it('FC-D-OVR-008 carrier overview hides buyer-only KPIs', () => {
    const keys = getOverviewKpiKeysForActor('CARRIER')
    expect(keys).not.toContain('accrued_total')
    expect(keys).not.toContain('forecast_exposure_total')
    expect(keys).not.toContain('current_variance_total')
  })

  it('FC-D-OVR-009 overview ready state when aggregate present', () => {
    expect(resolveFreightCostOverviewViewState({
      loading: false,
      missingCompany: false,
      forbidden: false,
      liveUnavailable: false,
      apiUnavailable: false,
      mixedCurrency: false,
      hasAggregate: true,
    })).toBe('ready')
  })

  it('FC-D-OVR-010 overview live unavailable state', () => {
    expect(resolveFreightCostOverviewViewState({
      loading: false,
      missingCompany: false,
      forbidden: false,
      liveUnavailable: true,
      apiUnavailable: false,
      mixedCurrency: false,
      hasAggregate: false,
    })).toBe('live_unavailable')
  })

  it('FC-D-OVR-011 overview mixed currency state', () => {
    expect(resolveFreightCostOverviewViewState({
      loading: false,
      missingCompany: false,
      forbidden: false,
      liveUnavailable: false,
      apiUnavailable: false,
      mixedCurrency: true,
      hasAggregate: true,
    })).toBe('mixed_currency')
  })

  it('FC-D-OVR-012 explicit zero string is not treated as null', () => {
    expect(isExplicitZeroMoney('0.00')).toBe(true)
    expect(isNullMoney('0.00')).toBe(false)
  })
})

describe('FC-D-PVA planned vs actual', () => {
  it('FC-D-PVA-001 row mapping preserves decimal strings', () => {
    const mapped = mapFreightCostSummaryToRowVM(summary(), { order_reference: 'ORD-1', carrier_name: 'Carrier A' })
    expect(mapped.planned_amount).toBe('1000.00')
    expect(mapped.forecast_exposure).toBe('1100.00')
  })

  it('FC-D-PVA-002 buyer columns include accrued and forecast', () => {
    const keys = getPlannedVsActualColumns('BUYER').map((column) => column.key)
    expect(keys).toContain('accrued_amount')
    expect(keys).toContain('forecast_exposure')
  })

  it('FC-D-PVA-003 carrier columns mask buyer-internal amounts', () => {
    const keys = getPlannedVsActualColumns('CARRIER').map((column) => column.key)
    expect(keys).not.toContain('accrued_amount')
    expect(keys).not.toContain('forecast_exposure')
    expect(keys).not.toContain('current_variance_amount')
  })

  it('FC-D-PVA-004 carrier row mask nulls buyer fields', () => {
    const masked = maskFreightCostRowForCarrier(row())
    expect(masked.accrued_amount).toBeNull()
    expect(masked.forecast_exposure).toBeNull()
    expect(masked.current_variance_amount).toBeNull()
  })

  it('FC-D-PVA-005 sort by cost_updated_at desc with tie-break', () => {
    const sorted = sortFreightCostRowsByUpdatedAt([
      row({ transport_order_id: 'a', cost_updated_at: '2026-08-21T12:00:00Z' }),
      row({ transport_order_id: 'b', cost_updated_at: '2026-08-22T12:00:00Z' }),
    ])
    expect(sorted[0]?.transport_order_id).toBe('b')
  })

  it('FC-D-PVA-006 pagination default page size slice', () => {
    const rows = Array.from({ length: 25 }, (_, index) => row({ transport_order_id: `to-${index}` }))
    const page = paginateFreightCostRows(rows, 20, 0)
    expect(page.items).toHaveLength(20)
    expect(page.total).toBe(25)
  })

  it('FC-D-PVA-007 list ready state with rows', () => {
    expect(resolveFreightCostListViewState({
      loading: false,
      missingCompany: false,
      forbidden: false,
      liveUnavailable: false,
      apiUnavailable: false,
      itemCount: 2,
    })).toBe('ready')
  })

  it('FC-D-PVA-008 list live unavailable state', () => {
    expect(resolveFreightCostListViewState({
      loading: false,
      missingCompany: false,
      forbidden: false,
      liveUnavailable: true,
      apiUnavailable: false,
      itemCount: 0,
    })).toBe('live_unavailable')
  })

  it('FC-D-PVA-009 null planned amount remains null in row VM', () => {
    const mapped = mapFreightCostSummaryToRowVM(summary({ planned_amount: null }), {
      order_reference: 'ORD-1',
      carrier_name: 'Carrier A',
    })
    expect(mapped.planned_amount).toBeNull()
  })

  it('FC-D-PVA-010 forecast column label key is plannedPlusProposedExposure', () => {
    const forecastColumn = getPlannedVsActualColumns('BUYER').find((column) => column.key === 'forecast_exposure')
    expect(forecastColumn?.labelKey).toBe('freightCosts.kpi.plannedPlusProposedExposure')
  })
})

describe('FC-D-DET shipment detail', () => {
  it('FC-D-DET-001 buyer detail sections include variance drivers', () => {
    const keys = getFreightCostDetailSections('BUYER').map((section) => section.key)
    expect(keys).toContain('variance_drivers')
    expect(keys).toContain('reconciliation')
  })

  it('FC-D-DET-002 carrier detail sections hide buyer-only blocks', () => {
    const keys = getFreightCostDetailSections('CARRIER').map((section) => section.key)
    expect(keys).not.toContain('accrual_breakdown')
    expect(keys).not.toContain('variance_drivers')
  })

  it('FC-D-DET-003 summary mask for carrier nulls buyer amounts', () => {
    const masked = maskFreightCostSummaryForCarrier(summary())
    expect(masked.accrued_amount).toBeNull()
    expect(masked.forecast_exposure).toBeNull()
    expect(masked.current_variance_amount).toBeNull()
  })

  it('FC-D-DET-004 detail mask clears drivers and findings for carrier', () => {
    const detail: FreightCostDetailVM = {
      summary: summary(),
      order_reference: 'ORD-1',
      carrier_name: 'Carrier A',
      planned_source: 'CONTRACT_RATE',
      variance_drivers: [{ driver_type: 'ACCESSORIAL', category: 'DETENTION', amount: '10.00', description: 'Detention' }],
      reconciliation_findings: [{ finding_id: 'f-1', finding_type: 'MISMATCH', status: 'OPEN', message: 'Mismatch' }],
    }
    const masked = maskFreightCostDetailForCarrier(detail)
    expect(masked.variance_drivers).toEqual([])
    expect(masked.reconciliation_findings).toEqual([])
  })

  it('FC-D-DET-005 provenance section always visible', () => {
    expect(getFreightCostDetailSections('CARRIER').some((section) => section.key === 'provenance')).toBe(true)
  })

  it('FC-D-DET-006 forecast section buyer-only', () => {
    const buyer = getFreightCostDetailSections('BUYER').find((section) => section.key === 'forecast_exposure')
    const carrier = getFreightCostDetailSections('CARRIER').find((section) => section.key === 'forecast_exposure')
    expect(buyer).toBeTruthy()
    expect(carrier).toBeUndefined()
  })

  it('FC-D-DET-007 reconciliation badge label key mapping', () => {
    expect(reconciliationLabelKey('MATCH')).toBe('freightCosts.reconciliation.MATCH')
    expect(reconciliationLabelKey(null)).toBe('freightCosts.unavailable.money')
  })

  it('FC-D-DET-008 detail ready state when detail present', () => {
    expect(resolveFreightCostDetailViewState({
      loading: false,
      notFound: false,
      forbidden: false,
      liveUnavailable: false,
      apiUnavailable: false,
      hasDetail: true,
    })).toBe('ready')
  })

  it('FC-D-DET-009 detail live unavailable state', () => {
    expect(resolveFreightCostDetailViewState({
      loading: false,
      notFound: false,
      forbidden: false,
      liveUnavailable: true,
      apiUnavailable: false,
      hasDetail: false,
    })).toBe('live_unavailable')
  })

  it('FC-D-DET-010 detail forbidden state', () => {
    expect(resolveFreightCostDetailViewState({
      loading: false,
      notFound: false,
      forbidden: true,
      liveUnavailable: false,
      apiUnavailable: false,
      hasDetail: false,
    })).toBe('forbidden')
  })

  it('FC-D-DET-011 shouldShowFreightCostField hides accrued for carrier', () => {
    expect(shouldShowFreightCostField('accrued_amount', 'CARRIER')).toBe(false)
    expect(shouldShowFreightCostField('planned_amount', 'CARRIER')).toBe(true)
  })

  it('FC-D-DET-012 planned snapshot section present for buyer', () => {
    expect(getFreightCostDetailSections('BUYER').some((section) => section.key === 'planned_snapshot')).toBe(true)
  })

  it('FC-D-DET-013 actual settlement section visible for carrier', () => {
    expect(getFreightCostDetailSections('CARRIER').some((section) => section.key === 'actual_settlement')).toBe(true)
  })

  it('FC-D-DET-014 detail not found error mapping', () => {
    expect(resolveFreightCostDetailError(new ApiError(404, { code: 'NOT_FOUND', message: 'missing', details: {} })))
      .toBe('not_found')
  })

  it('FC-D-DET-015 detail live unavailable error mapping', () => {
    expect(resolveFreightCostDetailError(new ApiError(503, {
      code: 'FREIGHT_COST_LIVE_UNAVAILABLE',
      message: 'v2.1E',
      details: {},
    }))).toBe('live_unavailable')
  })
})

describe('FC-D-ACC accessorial taxonomy', () => {
  it('FC-D-ACC-001 frozen category vocabulary includes detention', () => {
    expect(FREIGHT_COST_ACCESSORIAL_CATEGORIES).toContain('DETENTION')
  })

  it('FC-D-ACC-002 UNKNOWN category present', () => {
    expect(FREIGHT_COST_ACCESSORIAL_CATEGORIES).toContain('UNKNOWN')
  })

  it('FC-D-ACC-003 OTHER category present', () => {
    expect(FREIGHT_COST_ACCESSORIAL_CATEGORIES).toContain('OTHER')
  })

  it('FC-D-ACC-004 accessorial nav buyer-only permission', () => {
    expect(canSeeAccessorialAnalyticsNavForRoles(['PROCUREMENT_MANAGER'])).toBe(true)
    expect(canSeeAccessorialAnalyticsNavForRoles(['CARRIER_ADMIN'])).toBe(false)
  })

  it('FC-D-ACC-005 EN i18n category label exists for UNKNOWN', () => {
    expect(enFreightCosts.freightCosts.categories.UNKNOWN).toBe('Unknown')
  })

  it('FC-D-ACC-006 RU i18n category label exists for OTHER', () => {
    expect(ruFreightCosts.freightCosts.categories.OTHER).toBe('Прочее')
  })
})

describe('FC-D-CAR carrier performance layout', () => {
  it('FC-D-CAR-001 buyer can read freight costs workspace', () => {
    expect(canReadFreightCostsForRoles(['PROCUREMENT_MANAGER'])).toBe(true)
  })

  it('FC-D-CAR-002 carrier reader identified', () => {
    expect(isCarrierFreightCostReaderForRoles(['CARRIER_ADMIN'])).toBe(true)
  })

  it('FC-D-CAR-003 carrier nav includes carriers route', () => {
    expect(buildFreightCostNavItems('CARRIER').some((item) => item.key === 'carriers')).toBe(true)
  })

  it('FC-D-CAR-004 buyer internal analytics permission', () => {
    expect(canSeeBuyerInternalFreightCostFieldsForRoles(['FINANCE_MANAGER'])).toBe(true)
  })

  it('FC-D-CAR-005 production data source uses live API v2.1E mode', () => {
    const source = createProductionFreightCostDataSource()
    expect(source.mode).toBe('LIVE_API_V2_1E')
  })
})

describe('FC-D-LAN lane performance layout', () => {
  it('FC-D-LAN-001 buyer nav includes lanes route', () => {
    expect(buildFreightCostNavItems('BUYER').some((item) => item.key === 'lanes')).toBe(true)
  })

  it('FC-D-LAN-002 carrier nav includes lanes route', () => {
    expect(buildFreightCostNavItems('CARRIER').some((item) => item.key === 'lanes')).toBe(true)
  })

  it('FC-D-LAN-003 filter query supports origin and destination codes', () => {
    const query = buildFreightCostFilterQuery('buyer-1', {
      from: '',
      to: '',
      date_dimension: '',
      currency: 'RUB',
      carrier_id: '',
      origin_location_code: 'MOW',
      destination_location_code: 'LED',
      order_status: '',
      settlement_status: '',
      variance_state: '',
      reconciliation_state: '',
      q: '',
    })
    expect(query.origin_location_code).toBe('MOW')
    expect(query.destination_location_code).toBe('LED')
  })

  it('FC-D-LAN-004 production data source uses live API v2.1E mode', () => {
    const source = createProductionFreightCostDataSource()
    expect(source.mode).toBe('LIVE_API_V2_1E')
  })

  it('FC-D-LAN-005 lane list empty state resolves to empty', () => {
    expect(resolveFreightCostListViewState({
      loading: false,
      missingCompany: false,
      forbidden: false,
      liveUnavailable: false,
      apiUnavailable: false,
      itemCount: 0,
    })).toBe('empty')
  })
})

describe('FC-D-FLT filters', () => {
  it('FC-D-FLT-001 active chips include currency filter', () => {
    const chips = activeFreightCostFilterChips({
      from: '',
      to: '',
      date_dimension: '',
      currency: 'RUB',
      carrier_id: '',
      origin_location_code: '',
      destination_location_code: '',
      order_status: '',
      settlement_status: '',
      variance_state: '',
      reconciliation_state: '',
      q: '',
    })
    expect(chips).toContain('currency:RUB')
  })

  it('FC-D-FLT-002 server query maps company id', () => {
    expect(buildFreightCostFilterQuery('buyer-1', {
      from: '2026-01-01',
      to: '2026-08-22',
      date_dimension: 'TRANSPORT_ORDER_CREATED_AT',
      currency: 'RUB',
      carrier_id: 'carrier-1',
      origin_location_code: '',
      destination_location_code: '',
      order_status: '',
      settlement_status: '',
      variance_state: '',
      reconciliation_state: 'MISMATCH',
      q: 'ORD',
    }).company_id).toBe('buyer-1')
  })

  it('FC-D-FLT-003 reconciliation filter mapped to query param', () => {
    const query = buildFreightCostFilterQuery('buyer-1', {
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
      reconciliation_state: 'MISMATCH',
      q: '',
    })
    expect(query.reconciliation_state).toBe('MISMATCH')
  })

  it('FC-D-FLT-004 search query mapped', () => {
    const query = buildFreightCostFilterQuery('buyer-1', {
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
      q: 'ORD-100',
    })
    expect(query.q).toBe('ORD-100')
  })

  it('FC-D-FLT-005 pagination limit and offset mapped', () => {
    const query = buildFreightCostFilterQuery('buyer-1', {
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
    }, { limit: 20, offset: 40 })
    expect(query.limit).toBe(20)
    expect(query.offset).toBe(40)
  })

  it('FC-D-FLT-006 variance state filter mapped', () => {
    const query = buildFreightCostFilterQuery('buyer-1', {
      from: '',
      to: '',
      date_dimension: '',
      currency: '',
      carrier_id: '',
      origin_location_code: '',
      destination_location_code: '',
      order_status: '',
      settlement_status: '',
      variance_state: 'HAS_CURRENT_VARIANCE',
      reconciliation_state: '',
      q: '',
    })
    expect(query.variance_state).toBe('HAS_CURRENT_VARIANCE')
  })

  it('FC-D-FLT-007 carrier filter mapped', () => {
    const query = buildFreightCostFilterQuery('buyer-1', {
      from: '',
      to: '',
      date_dimension: '',
      currency: '',
      carrier_id: 'carrier-9',
      origin_location_code: '',
      destination_location_code: '',
      order_status: '',
      settlement_status: '',
      variance_state: '',
      reconciliation_state: '',
      q: '',
    })
    expect(query.carrier_id).toBe('carrier-9')
  })

  it('FC-D-FLT-008 date dimension mapped when provided', () => {
    const query = buildFreightCostFilterQuery('buyer-1', {
      from: '2026-01-01',
      to: '2026-08-22',
      date_dimension: 'TRANSPORT_ORDER_CREATED_AT',
      currency: '',
      carrier_id: '',
      origin_location_code: '',
      destination_location_code: '',
      order_status: '',
      settlement_status: '',
      variance_state: '',
      reconciliation_state: '',
      q: '',
    })
    expect(query.date_dimension).toBe('TRANSPORT_ORDER_CREATED_AT')
  })

  it('FC-D-FLT-009 empty filters produce minimal query', () => {
    const query = buildFreightCostFilterQuery('buyer-1', {
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
    })
    expect(Object.keys(query)).toEqual(['company_id'])
  })

  it('FC-D-FLT-010 mock adapter receives mapped query contract', async () => {
    const mock = createMockFreightCostDataSource({
      listOrders: async (query) => ({
        items: [summary({ transport_order_id: query.company_id ?? 'missing' })],
        total: 1,
        limit: 20,
        offset: 0,
      }),
    })
    const result = await mock.listOrders({ company_id: 'buyer-1', currency: 'RUB' })
    expect(result.items[0]?.transport_order_id).toBe('buyer-1')
  })
})

describe('FC-D-MON money formatting', () => {
  it('FC-D-MON-001 en-US formats decimal money without Number', () => {
    expect(formatDecimalMoney('1234.56', 'RUB', 'en-US')).toBe('1,234.56 RUB')
  })

  it('FC-D-MON-002 ru-RU uses space grouping', () => {
    expect(formatDecimalMoney('1234.56', 'RUB', 'ru-RU')).toContain('1')
    expect(formatDecimalMoney('1234.56', 'RUB', 'ru-RU')).toContain('RUB')
  })

  it('FC-D-MON-003 null money renders em dash', () => {
    expect(formatDecimalMoney(null, 'RUB', 'en-US')).toBe('—')
  })

  it('FC-D-MON-004 explicit zero string formats as zero', () => {
    expect(formatDecimalMoney('0.00', 'RUB', 'en-US')).toBe('0.00 RUB')
  })

  it('FC-D-MON-005 percent formatting appends suffix', () => {
    expect(formatDecimalPercent('5.00', 'en-US')).toBe('5.00%')
  })

  it('FC-D-MON-006 null percent unavailable', () => {
    expect(formatDecimalPercent(null, 'en-US')).toBe('—')
  })

  it('FC-D-MON-007 money aria label for null uses unavailable label', () => {
    expect(moneyAriaLabel(null, 'RUB', 'Unavailable')).toBe('Unavailable')
  })

  it('FC-D-MON-008 zh-CN locale formatting includes currency', () => {
    expect(formatDecimalMoney('999.10', 'CNY', 'zh-CN')).toContain('CNY')
  })
})

describe('FC-D-I18N localization', () => {
  it('FC-D-I18N-001 EN plannedPlusProposedExposure label frozen', () => {
    expect(enFreightCosts.freightCosts.kpi.plannedPlusProposedExposure).toBe('Planned + proposed exposure')
  })

  it('FC-D-I18N-002 RU plannedPlusProposedExposure label present', () => {
    expect(ruFreightCosts.freightCosts.kpi.plannedPlusProposedExposure).toContain('экспозиция')
  })

  it('FC-D-I18N-003 ZH plannedPlusProposedExposure label present', () => {
    expect(zhFreightCosts.freightCosts.kpi.plannedPlusProposedExposure).toContain('敞口')
  })

  it('FC-D-I18N-004 EN nav overview key present', () => {
    expect(enFreightCosts.freightCosts.nav.overview).toBe('Overview')
  })

  it('FC-D-I18N-005 RU nav plannedVsActual key present', () => {
    expect(ruFreightCosts.freightCosts.nav.plannedVsActual).toBeTruthy()
  })

  it('FC-D-I18N-006 ZH unavailable liveData key present', () => {
    expect(zhFreightCosts.freightCosts.unavailable.liveData).toContain('v2.1E')
  })
})

describe('FC-D-SEC frontend security UX', () => {
  it('FC-D-SEC-001 carrier mask hides forecast exposure field visibility', () => {
    expect(shouldShowFreightCostField('forecast_exposure', 'CARRIER')).toBe(false)
  })

  it('FC-D-SEC-002 mocked 403 maps to forbidden detail state', () => {
    expect(resolveFreightCostDetailError(new ApiError(403, { code: 'FORBIDDEN', message: 'deny', details: {} })))
      .toBe('forbidden')
  })

  it('FC-D-SEC-003 absent buyer field stays null after carrier mask', () => {
    const masked = maskFreightCostSummaryForCarrier(summary({ accrued_amount: '100.00' }))
    expect(masked.accrued_amount).toBeNull()
  })

  it('FC-D-SEC-004 adapter source excludes internal service token literal', () => {
    const adapterSource = readSource('composables/useFreightCostsApi.ts')
    expect(adapterSource).not.toContain('X-Internal-Service-Token')
  })

  it('FC-D-SEC-005 flag off hides nav even for authorized roles', () => {
    expect(shouldShowFreightCostsNav(false, ['PLATFORM_ADMIN'])).toBe(false)
    expect(canSeeVarianceAnalysisNavForRoles(['PROCUREMENT_MANAGER'])).toBe(true)
  })
})

describe('FC-D-ERR error and empty states', () => {
  it('FC-D-ERR-001 loading list state', () => {
    expect(resolveFreightCostListViewState({
      loading: true,
      missingCompany: false,
      forbidden: false,
      liveUnavailable: false,
      apiUnavailable: false,
      itemCount: 0,
    })).toBe('loading')
  })

  it('FC-D-ERR-002 empty list state', () => {
    expect(resolveFreightCostListViewState({
      loading: false,
      missingCompany: false,
      forbidden: false,
      liveUnavailable: false,
      apiUnavailable: false,
      itemCount: 0,
    })).toBe('empty')
  })

  it('FC-D-ERR-003 backend unavailable list state', () => {
    expect(resolveFreightCostListViewState({
      loading: false,
      missingCompany: false,
      forbidden: false,
      liveUnavailable: false,
      apiUnavailable: true,
      itemCount: 0,
    })).toBe('backend_unavailable')
  })

  it('FC-D-ERR-004 missing company list state', () => {
    expect(resolveFreightCostListViewState({
      loading: false,
      missingCompany: true,
      forbidden: false,
      liveUnavailable: false,
      apiUnavailable: false,
      itemCount: 0,
    })).toBe('missing_company')
  })

  it('FC-D-ERR-005 production adapter is live v2.1E (not fail-closed shell)', () => {
    const source = createProductionFreightCostDataSource()
    expect(source.mode).toBe('LIVE_API_V2_1E')
    expect(typeof source.listOrders).toBe('function')
  })

  it('FC-D-ERR-006 data source module documents forbidden browser paths', () => {
    expect(FREIGHT_COST_FORBIDDEN_BROWSER_PATHS).toContain('/internal/v1/freight-cost')
    expect(FREIGHT_COST_FORBIDDEN_BROWSER_HEADERS).toContain('X-Internal-Service-Token')
  })
})
