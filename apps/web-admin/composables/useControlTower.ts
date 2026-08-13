import type { PaginatedResponse } from '~/types/api'
import type { BillingRegister } from '~/types/billing'
import type { Company } from '~/types/company'
import type { DocumentRecord } from '~/types/document'
import type { FreightRequest, RfxEvent } from '~/types/rfx'
import type { Shipment } from '~/types/shipment'
import type { TransportOrder } from '~/types/transportOrder'
import type { AuthUser } from '~/types/api'
import {
  CONTROL_TOWER_SHIPMENT_BOARD_STATUSES,
  createDefaultControlTowerFilters,
  type ControlTowerActivityItem,
  type ControlTowerBillingSummary,
  type ControlTowerData,
  type ControlTowerDataFreshness,
  type ControlTowerDocumentsSummary,
  type ControlTowerEvent,
  type ControlTowerEventAcknowledgement,
  type ControlTowerFetchResult,
  type ControlTowerFilterOption,
  type ControlTowerFilters,
  type ControlTowerFunnelStep,
  type ControlTowerKpiCard,
  type ControlTowerKpiMetric,
  type ControlTowerOperationRow,
  type ControlTowerRiskAlert,
  type ControlTowerShipment,
  type ControlTowerShipmentStatusRow,
  type ControlTowerSummaryKpi,
  type ControlTowerSummaryResponse,
  type ControlTowerStatusSummary,
  type ControlTowerStatusSummaryFreshness,
} from '~/types/controlTower'
import { ApiError, formatApiErrorForUser } from '~/composables/useApi'
import {
  applyControlTowerFilters,
  buildCriticalEvents,
  buildKpiMetrics,
  companyNameMap,
  isActiveShipment,
  mapShipmentToControlTowerRow,
} from '~/utils/controlTowerLogic'
import {
  CONTROL_TOWER_DEMO_EVENTS,
  CONTROL_TOWER_DEMO_SHIPMENTS,
} from '~/utils/controlTowerDemoData'

const DEV_TENANT_FALLBACK = '74519f22-ff9b-4a8b-8fff-a958c689682f'

const DRIVER_REQUIRED_STATUSES = new Set([
  'DRIVER_ASSIGNED',
  'PICKUP_SLOT_BOOKED',
  'IN_PICKUP',
  'LOADED',
  'IN_TRANSIT',
  'ARRIVED_AT_CONSIGNEE',
  'UNLOADING',
  'DELIVERED',
  'DELIVERY_CONFIRMED',
  'DOCUMENTS_COMPLETED',
  'READY_FOR_BILLING',
  'INCLUDED_IN_BILLING_REGISTER',
  'FINANCIALLY_CLOSED',
])

const VEHICLE_REQUIRED_STATUSES = new Set([
  'VEHICLE_ASSIGNED',
  'DRIVER_ASSIGNED',
  'PICKUP_SLOT_BOOKED',
  'IN_PICKUP',
  'LOADED',
  'IN_TRANSIT',
  'ARRIVED_AT_CONSIGNEE',
  'UNLOADING',
  'DELIVERED',
  'DELIVERY_CONFIRMED',
  'DOCUMENTS_COMPLETED',
  'READY_FOR_BILLING',
  'INCLUDED_IN_BILLING_REGISTER',
  'FINANCIALLY_CLOSED',
])

function emptyResult<T>(key: string): ControlTowerFetchResult<T> {
  return { key, ok: false, total: 0, items: [] }
}

function areaStatus(result: ControlTowerFetchResult<unknown>): 'ok' | 'warning' | 'down' {
  if (!result.ok) return 'down'
  if (result.total === 0) return 'warning'
  return 'ok'
}

function countByStatus<T extends { status: string }>(items: T[], status: string): number {
  return items.filter((item) => item.status === status).length
}

function countByFieldStatus<T>(items: T[], field: keyof T, status: string): number {
  return items.filter((item) => String(item[field]) === status).length
}

function parseTimestamp(value?: string | null): number {
  if (!value) return 0
  const time = new Date(value).getTime()
  return Number.isNaN(time) ? 0 : time
}

export function useControlTower() {
  const config = useRuntimeConfig()
  const { apiGet, apiPost, checkGatewayHealth } = useApi()
  const { pushToast } = useToast()
  const tenantStore = useTenantStore()
  const uiStore = useUiStore()
  const { t } = useI18n()

  const summaryApiEnabled = config.public.controlTowerSummaryApiEnabled !== false

  const loading = ref(true)
  const gatewayOnline = ref(true)
  const loadError = ref<string | null>(null)
  const lastUpdatedAt = ref<string | null>(null)
  const demoMode = ref(false)
  const summaryMode = ref(false)
  const dataFreshness = ref<ControlTowerDataFreshness | null>(null)
  const summaryFilterOptions = ref<{
    shippers: ControlTowerFilterOption[]
    carriers: ControlTowerFilterOption[]
  }>({ shippers: [], carriers: [] })
  const summaryPagination = ref({ page: 1, limit: 50, total: 0, hasNext: false })
  const autoRefreshEnabled = ref(false)
  const autoRefreshIntervalMs = ref(60_000)
  const filters = reactive<ControlTowerFilters>(createDefaultControlTowerFilters())

  const summaryShipments = ref<ControlTowerShipment[]>([])
  const summaryKpi = ref<ControlTowerSummaryKpi | null>(null)
  const summaryEvents = ref<ControlTowerEvent[]>([])
  const acknowledgingEventId = ref<string | null>(null)
  const statusSummary = ref<ControlTowerStatusSummary | null>(null)
  const statusSummaryFreshness = ref<ControlTowerStatusSummaryFreshness | null>(null)

  let autoRefreshTimer: ReturnType<typeof setInterval> | undefined

  const data = ref<ControlTowerData>({
    companies: emptyResult('companies'),
    users: emptyResult('users'),
    transportOrders: emptyResult('transportOrders'),
    rfxEvents: emptyResult('rfxEvents'),
    freightRequests: emptyResult('freightRequests'),
    shipments: emptyResult('shipments'),
    documents: emptyResult('documents'),
    billingRegisters: emptyResult('billingRegisters'),
  })

  const tenantId = computed(() => tenantStore.tenantId?.trim() || DEV_TENANT_FALLBACK)

  function formatRoute(origin?: string, destination?: string): string | undefined {
    if (!origin && !destination) return undefined
    if (origin && destination) return `${origin} → ${destination}`
    return origin || destination
  }

  function mapSummaryShipment(row: ControlTowerShipment): ControlTowerShipment {
    const origin = row.origin ?? row.originName
    const destination = row.destination ?? row.destinationName
    return {
      ...row,
      shipperCompanyId: row.shipperCompanyId ?? row.shipperId,
      carrierCompanyId: row.carrierCompanyId ?? row.carrierId,
      origin,
      destination,
      route: row.route ?? formatRoute(origin, destination),
    }
  }

  function buildSummaryQuery(): Record<string, string | number | boolean> {
    const query: Record<string, string | number | boolean> = {
      page: summaryPagination.value.page,
      limit: summaryPagination.value.limit,
    }
    if (filters.search.trim()) query.q = filters.search.trim()
    if (filters.status) query.status = filters.status
    if (filters.slaStatus) query.sla_status = filters.slaStatus
    if (filters.shipperCompanyId) query.shipper_id = filters.shipperCompanyId
    if (filters.carrierCompanyId) query.carrier_id = filters.carrierCompanyId
    if (filters.date) {
      query.date_from = filters.date
      query.date_to = filters.date
    }
    if (filters.criticalOnly) query.critical_only = true
    return query
  }

  function buildKpiMetricsFromSummary(kpi: ControlTowerSummaryKpi): ControlTowerKpiMetric[] {
    const pct = (value: number) => (kpi.active ? Math.round((value / kpi.active) * 100) : 0)
    const combinedCritical = kpi.critical + kpi.delayed

    return [
      {
        key: 'activeShipments',
        titleKey: 'controlTower.kpiV01.activeShipments',
        descriptionKey: 'controlTower.kpiV01.activeShipmentsDesc',
        value: kpi.active,
        tone: 'neutral',
      },
      {
        key: 'onTime',
        titleKey: 'controlTower.kpiV01.onTime',
        descriptionKey: 'controlTower.kpiV01.onTimeDesc',
        value: kpi.onTime,
        percent: pct(kpi.onTime),
        tone: 'ok',
      },
      {
        key: 'atRisk',
        titleKey: 'controlTower.kpiV01.atRisk',
        descriptionKey: 'controlTower.kpiV01.atRiskDesc',
        value: kpi.atRisk,
        percent: pct(kpi.atRisk),
        tone: 'warning',
      },
      {
        key: 'critical',
        titleKey: 'controlTower.kpiV01.critical',
        descriptionKey: 'controlTower.kpiV01.criticalDesc',
        value: combinedCritical,
        percent: pct(combinedCritical),
        tone: 'danger',
      },
      {
        key: 'awaitingDocuments',
        titleKey: 'controlTower.kpiV01.awaitingDocuments',
        descriptionKey: 'controlTower.kpiV01.awaitingDocumentsDesc',
        value: kpi.awaitingDocuments,
        percent: pct(kpi.awaitingDocuments),
        tone: 'warning',
      },
      {
        key: 'readyForBilling',
        titleKey: 'controlTower.kpiV01.readyForBilling',
        descriptionKey: 'controlTower.kpiV01.readyForBillingDesc',
        value: kpi.readyForBilling,
        percent: pct(kpi.readyForBilling),
        tone: 'ok',
      },
    ]
  }

  function applySummaryResponse(summary: ControlTowerSummaryResponse) {
    summaryMode.value = true
    demoMode.value = false
    loadError.value = null
    dataFreshness.value = summary.dataFreshness
    summaryKpi.value = summary.kpi
    summaryShipments.value = summary.shipments.items.map(mapSummaryShipment)
    summaryEvents.value = summary.criticalEvents.map((event) => ({
      ...event,
      occurredAt:
        typeof event.occurredAt === 'string' ? event.occurredAt : String(event.occurredAt),
    }))
    summaryFilterOptions.value = {
      shippers: summary.filters.shippers,
      carriers: summary.filters.carriers,
    }
    summaryPagination.value = {
      page: summary.shipments.page,
      limit: summary.shipments.limit,
      total: summary.shipments.total,
      hasNext: summary.shipments.hasNext,
    }
    statusSummary.value = summary.statusSummary ?? null
    statusSummaryFreshness.value = summary.statusSummaryFreshness ?? null
    lastUpdatedAt.value = summary.generatedAt
  }

  function acknowledgeErrorMessage(error: unknown): string {
    if (error instanceof ApiError) {
      if (error.status === 403) {
        return t('controlTower.events.acknowledgeError.forbidden')
      }
      if (error.status === 404) {
        return t('controlTower.events.acknowledgeError.notFound')
      }
      if (error.status === 503 || error.status >= 500) {
        return t('controlTower.events.acknowledgeError.unavailable')
      }
    }
    return formatApiErrorForUser(error)
  }

  async function acknowledgeCriticalEvent(eventId: string): Promise<boolean> {
    if (demoMode.value || !summaryMode.value) {
      return false
    }
    if (acknowledgingEventId.value) {
      return false
    }

    acknowledgingEventId.value = eventId
    try {
      await apiPost<ControlTowerEventAcknowledgement>(
        `/api/v1/control-tower/critical-events/${encodeURIComponent(eventId)}/acknowledge`,
        undefined,
        { skipTenant: true },
      )
      pushToast('success', t('controlTower.events.acknowledgeSuccess'))
      await loadSummaryData()
      return true
    } catch (error) {
      pushToast('error', acknowledgeErrorMessage(error))
      return false
    } finally {
      acknowledgingEventId.value = null
    }
  }

  async function loadSummaryData(): Promise<boolean> {
    try {
      const summary = await apiGet<ControlTowerSummaryResponse>('/api/v1/control-tower/summary', {
        query: buildSummaryQuery(),
        skipTenant: true,
      })
      applySummaryResponse(summary)
      return true
    } catch (error) {
      summaryMode.value = false
      if (error instanceof ApiError) {
        if (error.status === 503) {
          loadError.value = 'api_unavailable'
          return false
        }
        if (error.status === 404 && import.meta.dev) {
          return false
        }
      }
      if (!summaryApiEnabled) {
        return false
      }
      if (import.meta.dev) {
        return false
      }
      loadError.value = 'api_unavailable'
      return false
    }
  }

  async function fetchList<T>(path: string, key: string): Promise<ControlTowerFetchResult<T>> {
    try {
      const response = await apiGet<PaginatedResponse<T>>(path, {
        query: { tenant_id: tenantId.value, limit: 200, offset: 0 },
      })
      return {
        key,
        ok: true,
        total: response.total ?? response.items?.length ?? 0,
        items: response.items ?? [],
      }
    } catch {
      return emptyResult<T>(key)
    }
  }

  async function loadData() {
    loading.value = true
    loadError.value = null
    demoMode.value = false
    summaryMode.value = false
    try {
      gatewayOnline.value = await checkGatewayHealth()
    } catch {
      gatewayOnline.value = false
    }

    if (summaryApiEnabled) {
      const loaded = await loadSummaryData()
      if (loaded) {
        loading.value = false
        return
      }
      if (loadError.value === 'api_unavailable') {
        loading.value = false
        return
      }
    }

    const settled = await Promise.allSettled([
      fetchList<Company>('/api/v1/companies', 'companies'),
      fetchList<AuthUser>('/api/v1/users', 'users'),
      fetchList<TransportOrder>('/api/v1/transport-orders', 'transportOrders'),
      fetchList<RfxEvent>('/api/v1/rfx-events', 'rfxEvents'),
      fetchList<FreightRequest>('/api/v1/freight-requests', 'freightRequests'),
      fetchList<Shipment>('/api/v1/shipments', 'shipments'),
      fetchList<DocumentRecord>('/api/v1/documents', 'documents'),
      fetchList<BillingRegister>('/api/v1/billing-registers', 'billingRegisters'),
    ])

    const [
      companiesResult,
      usersResult,
      transportOrdersResult,
      rfxEventsResult,
      freightRequestsResult,
      shipmentsResult,
      documentsResult,
      billingRegistersResult,
    ] = settled

    data.value.companies =
      companiesResult.status === 'fulfilled' ? companiesResult.value : emptyResult('companies')
    data.value.users =
      usersResult.status === 'fulfilled' ? usersResult.value : emptyResult('users')
    data.value.transportOrders =
      transportOrdersResult.status === 'fulfilled'
        ? transportOrdersResult.value
        : emptyResult('transportOrders')
    data.value.rfxEvents =
      rfxEventsResult.status === 'fulfilled' ? rfxEventsResult.value : emptyResult('rfxEvents')
    data.value.freightRequests =
      freightRequestsResult.status === 'fulfilled'
        ? freightRequestsResult.value
        : emptyResult('freightRequests')
    data.value.shipments =
      shipmentsResult.status === 'fulfilled' ? shipmentsResult.value : emptyResult('shipments')
    data.value.documents =
      documentsResult.status === 'fulfilled' ? documentsResult.value : emptyResult('documents')
    data.value.billingRegisters =
      billingRegistersResult.status === 'fulfilled'
        ? billingRegistersResult.value
        : emptyResult('billingRegisters')

    const coreUnavailable =
      !gatewayOnline.value ||
      !data.value.shipments.ok ||
      !data.value.companies.ok ||
      !data.value.transportOrders.ok

    if (coreUnavailable && import.meta.dev) {
      demoMode.value = true
    } else if (coreUnavailable) {
      loadError.value = 'api_unavailable'
    }

    lastUpdatedAt.value = new Date().toISOString()
    loading.value = false
  }

  const activeRfxCount = computed(() =>
    data.value.rfxEvents.items.filter((item) => item.status === 'PUBLISHED').length,
  )

  const activeShipmentsCount = computed(() =>
    data.value.shipments.items.filter(
      (item) => !['CANCELLED', 'FINANCIALLY_CLOSED'].includes(item.status),
    ).length,
  )

  // TODO: add global bids list endpoint when available in API Gateway.
  const bidsCount = computed(() => 0)

  const revenueTotal = computed(() =>
    data.value.billingRegisters.items.reduce((sum, item) => sum + (item.total_with_vat ?? 0), 0),
  )

  function buildKpi(
    key: string,
    titleKey: string,
    descriptionKey: string,
    result: ControlTowerFetchResult<unknown>,
    link: string,
    valueOverride?: string | number,
  ): ControlTowerKpiCard {
    const unavailable = !result.ok
    return {
      key,
      titleKey,
      descriptionKey,
      value: unavailable ? '—' : (valueOverride ?? result.total),
      badgeLabel: unavailable ? t('controlTower.badge.unavailable') : t('controlTower.badge.live'),
      badgeTone: unavailable ? 'unavailable' : 'ok',
      link,
      unavailable,
    }
  }

  const kpiCards = computed<ControlTowerKpiCard[]>(() => [
    buildKpi('companies', 'controlTower.kpi.companies', 'controlTower.kpi.companiesDesc', data.value.companies, '/companies'),
    buildKpi('users', 'controlTower.kpi.users', 'controlTower.kpi.usersDesc', data.value.users, '/users'),
    buildKpi(
      'transportOrders',
      'controlTower.kpi.transportOrders',
      'controlTower.kpi.transportOrdersDesc',
      data.value.transportOrders,
      '/transport-orders',
    ),
    buildKpi(
      'freightRequests',
      'controlTower.kpi.freightRequests',
      'controlTower.kpi.freightRequestsDesc',
      data.value.freightRequests,
      '/freight-requests',
    ),
    buildKpi(
      'activeRfx',
      'controlTower.kpi.activeRfx',
      'controlTower.kpi.activeRfxDesc',
      data.value.rfxEvents,
      '/rfx',
      data.value.rfxEvents.ok ? activeRfxCount.value : '—',
    ),
    buildKpi('bids', 'controlTower.kpi.bids', 'controlTower.kpi.bidsDesc', data.value.freightRequests, '/freight-requests', bidsCount.value),
    buildKpi(
      'activeShipments',
      'controlTower.kpi.activeShipments',
      'controlTower.kpi.activeShipmentsDesc',
      data.value.shipments,
      '/shipments',
      data.value.shipments.ok ? activeShipmentsCount.value : '—',
    ),
    buildKpi('documents', 'controlTower.kpi.documents', 'controlTower.kpi.documentsDesc', data.value.documents, '/documents'),
    buildKpi(
      'billingRegisters',
      'controlTower.kpi.billingRegisters',
      'controlTower.kpi.billingRegistersDesc',
      data.value.billingRegisters,
      '/billing-registers',
    ),
    buildKpi(
      'revenue',
      'controlTower.kpi.revenue',
      'controlTower.kpi.revenueDesc',
      data.value.billingRegisters,
      '/billing-registers',
      data.value.billingRegisters.ok
        ? new Intl.NumberFormat(undefined, { maximumFractionDigits: 0 }).format(revenueTotal.value)
        : '—',
    ),
  ])

  const operationsRows = computed<ControlTowerOperationRow[]>(() => [
    { key: 'companies', areaKey: 'controlTower.operations.companies', status: areaStatus(data.value.companies), count: data.value.companies.total, link: '/companies' },
    { key: 'users', areaKey: 'controlTower.operations.users', status: areaStatus(data.value.users), count: data.value.users.total, link: '/users' },
    { key: 'orders', areaKey: 'controlTower.operations.orders', status: areaStatus(data.value.transportOrders), count: data.value.transportOrders.total, link: '/transport-orders' },
    { key: 'rfx', areaKey: 'controlTower.operations.rfx', status: areaStatus(data.value.rfxEvents), count: data.value.rfxEvents.total, link: '/rfx' },
    { key: 'freightRequests', areaKey: 'controlTower.operations.freightRequests', status: areaStatus(data.value.freightRequests), count: data.value.freightRequests.total, link: '/freight-requests' },
    { key: 'shipments', areaKey: 'controlTower.operations.shipments', status: areaStatus(data.value.shipments), count: data.value.shipments.total, link: '/shipments' },
    { key: 'documents', areaKey: 'controlTower.operations.documents', status: areaStatus(data.value.documents), count: data.value.documents.total, link: '/documents' },
    { key: 'billing', areaKey: 'controlTower.operations.billing', status: areaStatus(data.value.billingRegisters), count: data.value.billingRegisters.total, link: '/billing-registers' },
  ])

  const transportFunnel = computed<ControlTowerFunnelStep[]>(() => {
    const orders = data.value.transportOrders.items
    const shipments = data.value.shipments.items
    const freight = data.value.freightRequests.items
    const rfx = data.value.rfxEvents.items

    return [
      { key: 'draftOrders', labelKey: 'controlTower.transportFunnel.draftOrders', count: countByStatus(orders, 'DRAFT') },
      { key: 'readyForSourcing', labelKey: 'controlTower.transportFunnel.readyForSourcing', count: countByStatus(orders, 'READY_FOR_SOURCING') },
      {
        key: 'inTender',
        labelKey: 'controlTower.transportFunnel.inTender',
        count:
          countByStatus(orders, 'SOURCING_IN_PROGRESS') +
          freight.filter((item) => ['PUBLISHED', 'RESPONSES_OPEN'].includes(item.status)).length +
          rfx.filter((item) => item.status === 'PUBLISHED').length,
      },
      {
        key: 'carrierAssigned',
        labelKey: 'controlTower.transportFunnel.carrierAssigned',
        count: countByStatus(orders, 'ASSIGNED') + countByStatus(shipments, 'CARRIER_ASSIGNED'),
      },
      {
        key: 'inTransit',
        labelKey: 'controlTower.transportFunnel.inTransit',
        count: shipments.filter((item) =>
          ['IN_TRANSIT', 'LOADED', 'IN_PICKUP', 'ARRIVED_AT_CONSIGNEE', 'UNLOADING'].includes(item.status),
        ).length,
      },
      {
        key: 'delivered',
        labelKey: 'controlTower.transportFunnel.delivered',
        count: shipments.filter((item) => ['DELIVERED', 'DELIVERY_CONFIRMED'].includes(item.status)).length,
      },
      { key: 'readyForBilling', labelKey: 'controlTower.transportFunnel.readyForBilling', count: countByStatus(shipments, 'READY_FOR_BILLING') },
      {
        key: 'closed',
        labelKey: 'controlTower.transportFunnel.closed',
        count:
          countByStatus(shipments, 'FINANCIALLY_CLOSED') + countByStatus(orders, 'CONVERTED_TO_SHIPMENT'),
      },
    ]
  })

  const transportFunnelEmpty = computed(
    () =>
      data.value.transportOrders.ok &&
      data.value.shipments.ok &&
      transportFunnel.value.every((step) => step.count === 0),
  )

  const tenderFunnel = computed<ControlTowerFunnelStep[]>(() => {
    const rfx = data.value.rfxEvents.items
    const freight = data.value.freightRequests.items

    return [
      { key: 'draftRfx', labelKey: 'controlTower.tenderFunnel.draftRfx', count: rfx.filter((item) => item.status === 'DRAFT').length },
      { key: 'publishedRfx', labelKey: 'controlTower.tenderFunnel.publishedRfx', count: rfx.filter((item) => item.status === 'PUBLISHED').length },
      { key: 'participantsInvited', labelKey: 'controlTower.tenderFunnel.participantsInvited', count: rfx.filter((item) => item.status === 'PUBLISHED').length },
      { key: 'bidsReceived', labelKey: 'controlTower.tenderFunnel.bidsReceived', count: bidsCount.value },
      { key: 'bidSubmitted', labelKey: 'controlTower.tenderFunnel.bidSubmitted', count: 0 },
      { key: 'winnerSelected', labelKey: 'controlTower.tenderFunnel.winnerSelected', count: freight.filter((item) => item.status === 'AWARDED').length },
      {
        key: 'shipmentCreated',
        labelKey: 'controlTower.tenderFunnel.shipmentCreated',
        count: data.value.shipments.items.filter((item) => Boolean(item.transport_order_id)).length,
      },
    ]
  })

  const shipmentStatusBoard = computed<ControlTowerShipmentStatusRow[]>(() =>
    CONTROL_TOWER_SHIPMENT_BOARD_STATUSES.map((status) => ({
      status,
      count: countByStatus(data.value.shipments.items, status),
      link: `/shipments?status=${status}`,
    })),
  )

  const documentsSummary = computed<ControlTowerDocumentsSummary>(() => {
    const docs = data.value.documents
    if (!docs.ok) {
      return {
        total: 0,
        readyForSigning: 0,
        signed: 0,
        archived: 0,
        cancelled: 0,
        unavailable: true,
      }
    }
    return {
      total: docs.total,
      readyForSigning: docs.items.filter((item) =>
        ['READY_FOR_SIGNING', 'SIGNING_IN_PROGRESS'].includes(item.document_status),
      ).length,
      signed: docs.items.filter((item) => ['SIGNED', 'ACCEPTED'].includes(item.document_status)).length,
      archived: countByFieldStatus(docs.items, 'document_status', 'ARCHIVED'),
      cancelled: countByFieldStatus(docs.items, 'document_status', 'CANCELLED'),
      unavailable: false,
    }
  })

  const billingSummary = computed<ControlTowerBillingSummary>(() => {
    const billing = data.value.billingRegisters
    if (!billing.ok) {
      return {
        total: 0,
        draft: 0,
        approved: 0,
        closingDocsCreated: 0,
        sentToEdo: 0,
        signed: 0,
        paid: 0,
        closed: 0,
        revenueTotal: 0,
        unavailable: true,
      }
    }
    return {
      total: billing.total,
      draft: countByStatus(billing.items, 'DRAFT'),
      approved: countByStatus(billing.items, 'APPROVED'),
      closingDocsCreated: countByStatus(billing.items, 'CLOSING_DOCUMENTS_CREATED'),
      sentToEdo: countByStatus(billing.items, 'SENT_TO_EDO'),
      signed: countByStatus(billing.items, 'SIGNED_BY_COUNTERPARTY'),
      paid: countByStatus(billing.items, 'PAID'),
      closed: countByStatus(billing.items, 'CLOSED'),
      revenueTotal: revenueTotal.value,
      unavailable: false,
    }
  })

  const riskAlerts = computed<ControlTowerRiskAlert[]>(() => {
    const alerts: ControlTowerRiskAlert[] = []

    if (!gatewayOnline.value || uiStore.apiGatewayStatus !== 'online') {
      alerts.push({ key: 'gateway', messageKey: 'controlTower.risks.gatewayUnavailable', severity: 'danger' })
    }
    if (!data.value.companies.ok) {
      alerts.push({ key: 'companies', messageKey: 'controlTower.risks.companiesUnavailable', severity: 'danger' })
    }
    if (!data.value.shipments.ok) {
      alerts.push({ key: 'shipments', messageKey: 'controlTower.risks.shipmentsUnavailable', severity: 'danger' })
    }
    if (!data.value.billingRegisters.ok) {
      alerts.push({ key: 'billing', messageKey: 'controlTower.risks.billingUnavailable', severity: 'danger' })
    }

    const shipments = data.value.shipments.items
    const documents = data.value.documents.items
    const billing = data.value.billingRegisters.items

    const withoutDriver = shipments.filter(
      (item) => DRIVER_REQUIRED_STATUSES.has(item.status) && !item.driver_id,
    ).length
    if (withoutDriver > 0) {
      alerts.push({
        key: 'noDriver',
        messageKey: 'controlTower.risks.shipmentsWithoutDriver',
        severity: 'warning',
        count: withoutDriver,
      })
    }

    const withoutVehicle = shipments.filter(
      (item) => VEHICLE_REQUIRED_STATUSES.has(item.status) && !item.vehicle_id,
    ).length
    if (withoutVehicle > 0) {
      alerts.push({
        key: 'noVehicle',
        messageKey: 'controlTower.risks.shipmentsWithoutVehicle',
        severity: 'warning',
        count: withoutVehicle,
      })
    }

    const deliveredStatuses = new Set(['DELIVERED', 'DELIVERY_CONFIRMED', 'DOCUMENTS_COMPLETED'])
    const shipmentIdsWithDocs = new Set(
      documents
        .filter((doc) => doc.related_entity_type === 'SHIPMENT' && doc.related_entity_id)
        .map((doc) => doc.related_entity_id as string),
    )
    const deliveredWithoutDocs = shipments.filter(
      (item) => deliveredStatuses.has(item.status) && !shipmentIdsWithDocs.has(item.id),
    ).length
    if (deliveredWithoutDocs > 0) {
      alerts.push({
        key: 'deliveredNoDocs',
        messageKey: 'controlTower.risks.deliveredWithoutDocuments',
        severity: 'warning',
        count: deliveredWithoutDocs,
      })
    }

    const readyNotInBilling = shipments.filter((item) => item.status === 'READY_FOR_BILLING').length
    if (readyNotInBilling > 0) {
      alerts.push({
        key: 'readyNotBilling',
        messageKey: 'controlTower.risks.readyForBillingNotIncluded',
        severity: 'warning',
        count: readyNotInBilling,
      })
    }

    const approvedNotSigned = billing.filter((item) => item.status === 'APPROVED').length
    if (approvedNotSigned > 0) {
      alerts.push({
        key: 'approvedNotSigned',
        messageKey: 'controlTower.risks.approvedNotSigned',
        severity: 'warning',
        count: approvedNotSigned,
      })
    }

    const signedNotPaid = billing.filter((item) => item.status === 'SIGNED_BY_COUNTERPARTY').length
    if (signedNotPaid > 0) {
      alerts.push({
        key: 'signedNotPaid',
        messageKey: 'controlTower.risks.signedNotPaid',
        severity: 'warning',
        count: signedNotPaid,
      })
    }

    return alerts
  })

  const recentActivity = computed<ControlTowerActivityItem[]>(() => {
    const items: ControlTowerActivityItem[] = []

    for (const company of data.value.companies.items) {
      items.push({
        id: `company-${company.id}`,
        typeKey: 'controlTower.activity.company',
        title: company.short_name || company.legal_name,
        status: company.status,
        timestamp: company.created_at ?? '',
        link: `/companies/${company.id}`,
      })
    }

    for (const order of data.value.transportOrders.items) {
      items.push({
        id: `order-${order.id}`,
        typeKey: 'controlTower.activity.transportOrder',
        title: order.order_number,
        status: order.status,
        timestamp: order.created_at ?? '',
        link: `/transport-orders/${order.id}`,
      })
    }

    for (const event of data.value.rfxEvents.items) {
      items.push({
        id: `rfx-${event.id}`,
        typeKey: 'controlTower.activity.rfx',
        title: event.title || event.rfx_number,
        status: event.status,
        timestamp: event.created_at ?? '',
        link: `/rfx/${event.id}`,
      })
    }

    for (const shipment of data.value.shipments.items) {
      items.push({
        id: `shipment-${shipment.id}`,
        typeKey: 'controlTower.activity.shipment',
        title: shipment.shipment_number,
        status: shipment.status,
        timestamp: shipment.created_at ?? '',
        link: `/shipments/${shipment.id}`,
      })
    }

    for (const document of data.value.documents.items) {
      items.push({
        id: `document-${document.id}`,
        typeKey: 'controlTower.activity.document',
        title: document.document_number,
        status: document.document_status,
        timestamp: document.created_at ?? '',
        link: `/documents/${document.id}`,
      })
    }

    for (const register of data.value.billingRegisters.items) {
      items.push({
        id: `billing-${register.id}`,
        typeKey: 'controlTower.activity.billing',
        title: register.register_number,
        status: register.status,
        timestamp: register.created_at ?? '',
        link: `/billing-registers/${register.id}`,
      })
    }

    return items
      .filter((item) => parseTimestamp(item.timestamp) > 0)
      .sort((a, b) => parseTimestamp(b.timestamp) - parseTimestamp(a.timestamp))
      .slice(0, 15)
  })

  const companiesMap = computed(() => companyNameMap(data.value.companies.items))

  const transportOrderMap = computed(() => {
    const map = new Map<string, TransportOrder>()
    for (const order of data.value.transportOrders.items) {
      map.set(order.id, order)
    }
    return map
  })

  const controlTowerShipments = computed<ControlTowerShipment[]>(() => {
    if (summaryMode.value) {
      return summaryShipments.value
    }
    if (demoMode.value) {
      return CONTROL_TOWER_DEMO_SHIPMENTS
    }

    return data.value.shipments.items
      .filter((shipment) => isActiveShipment(shipment.status))
      .map((shipment) =>
        mapShipmentToControlTowerRow(shipment, {
          transportOrderById: transportOrderMap.value,
          companyNameById: companiesMap.value,
          locationLabelById: new Map(),
        }),
      )
  })

  const filteredShipments = computed(() => {
    if (summaryMode.value) {
      return summaryShipments.value
    }
    return applyControlTowerFilters(controlTowerShipments.value, filters)
  })

  const kpiMetrics = computed<ControlTowerKpiMetric[]>(() => {
    if (summaryMode.value && summaryKpi.value) {
      return buildKpiMetricsFromSummary(summaryKpi.value)
    }
    return buildKpiMetrics(controlTowerShipments.value)
  })

  const criticalEvents = computed<ControlTowerEvent[]>(() => {
    if (summaryMode.value) {
      return summaryEvents.value
    }
    if (demoMode.value) {
      return CONTROL_TOWER_DEMO_EVENTS
    }
    return buildCriticalEvents(data.value.shipments.items, data.value.documents.items, {
      apiUnavailable: !gatewayOnline.value || !data.value.shipments.ok,
    })
  })

  const apiUnavailable = computed(
    () => !demoMode.value && (!gatewayOnline.value || loadError.value === 'api_unavailable'),
  )

  const shipperCompanies = computed(() => {
    if (summaryMode.value) {
      return summaryFilterOptions.value.shippers.map((option) => ({
        id: option.value,
        legal_name: option.label,
        short_name: option.label,
        company_type: 'SHIPPER',
      })) as Company[]
    }
    return data.value.companies.items.filter((company) => company.company_type === 'SHIPPER')
  })

  const carrierCompanies = computed(() => {
    if (summaryMode.value) {
      return summaryFilterOptions.value.carriers.map((option) => ({
        id: option.value,
        legal_name: option.label,
        short_name: option.label,
        company_type: 'CARRIER',
      })) as Company[]
    }
    return data.value.companies.items.filter((company) => company.company_type === 'CARRIER')
  })

  function resetFilters() {
    Object.assign(filters, createDefaultControlTowerFilters())
  }

  function setAutoRefresh(enabled: boolean, intervalMs = autoRefreshIntervalMs.value) {
    autoRefreshEnabled.value = enabled
    autoRefreshIntervalMs.value = intervalMs
    stopAutoRefresh()
    if (enabled) {
      autoRefreshTimer = setInterval(() => {
        void loadData()
      }, intervalMs)
    }
  }

  function stopAutoRefresh() {
    if (autoRefreshTimer) {
      clearInterval(autoRefreshTimer)
      autoRefreshTimer = undefined
    }
  }

  function parseFiltersFromQuery(query: Record<string, unknown>) {
    filters.search = String(query.q ?? query.search ?? '')
    filters.status = String(query.status ?? '')
    filters.slaStatus = String(query.sla ?? query.slaStatus ?? '')
    filters.shipperCompanyId = String(query.shipper ?? query.shipperCompanyId ?? '')
    filters.carrierCompanyId = String(query.carrier ?? query.carrierCompanyId ?? '')
    filters.date = String(query.date ?? '')
    filters.criticalOnly = query.critical === '1' || query.criticalOnly === 'true'
  }

  function filtersToQuery(): Record<string, string> {
    const query: Record<string, string> = {}
    if (filters.search.trim()) query.q = filters.search.trim()
    if (filters.status) query.status = filters.status
    if (filters.slaStatus) query.sla = filters.slaStatus
    if (filters.shipperCompanyId) query.shipper = filters.shipperCompanyId
    if (filters.carrierCompanyId) query.carrier = filters.carrierCompanyId
    if (filters.date) query.date = filters.date
    if (filters.criticalOnly) query.critical = '1'
    return query
  }

  onScopeDispose(() => {
    stopAutoRefresh()
  })

  return {
    loading,
    gatewayOnline,
    loadError,
    lastUpdatedAt,
    demoMode,
    summaryMode,
    dataFreshness,
    summaryPagination,
    autoRefreshEnabled,
    autoRefreshIntervalMs,
    filters,
    tenantId,
    data,
    controlTowerShipments,
    filteredShipments,
    kpiMetrics,
    criticalEvents,
    acknowledgingEventId,
    acknowledgeCriticalEvent,
    apiUnavailable,
    statusSummary,
    statusSummaryFreshness,
    shipperCompanies,
    carrierCompanies,
    resetFilters,
    setAutoRefresh,
    stopAutoRefresh,
    parseFiltersFromQuery,
    filtersToQuery,
    kpiCards,
    operationsRows,
    transportFunnel,
    transportFunnelEmpty,
    tenderFunnel,
    shipmentStatusBoard,
    documentsSummary,
    billingSummary,
    riskAlerts,
    recentActivity,
    loadData,
  }
}
