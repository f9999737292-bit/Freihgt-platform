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
  type ControlTowerActivityItem,
  type ControlTowerBillingSummary,
  type ControlTowerData,
  type ControlTowerDocumentsSummary,
  type ControlTowerFetchResult,
  type ControlTowerFunnelStep,
  type ControlTowerKpiCard,
  type ControlTowerOperationRow,
  type ControlTowerRiskAlert,
  type ControlTowerShipmentStatusRow,
} from '~/types/controlTower'

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
  const { apiGet, checkGatewayHealth } = useApi()
  const tenantStore = useTenantStore()
  const uiStore = useUiStore()
  const { t } = useI18n()

  const loading = ref(true)
  const gatewayOnline = ref(true)

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
    try {
      gatewayOnline.value = await checkGatewayHealth()
    } catch {
      gatewayOnline.value = false
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

  return {
    loading,
    gatewayOnline,
    tenantId,
    data,
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
