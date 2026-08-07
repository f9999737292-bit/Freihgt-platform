import type { Company } from '~/types/company'
import type { DocumentRecord } from '~/types/document'
import type { Shipment } from '~/types/shipment'
import type { TransportOrder } from '~/types/transportOrder'
import type {
  ControlTowerEvent,
  ControlTowerEventType,
  ControlTowerFilters,
  ControlTowerKpiMetric,
  ControlTowerSeverity,
  ControlTowerShipment,
  SlaStatus,
} from '~/types/controlTower'
import { shortId } from '~/types/shipment'

const TERMINAL_STATUSES = new Set(['CANCELLED', 'FINANCIALLY_CLOSED'])

const IN_TRANSIT_STATUSES = new Set([
  'IN_PICKUP',
  'LOADED',
  'IN_TRANSIT',
  'ARRIVED_AT_CONSIGNEE',
  'UNLOADING',
])

const PRE_PICKUP_STATUSES = new Set([
  'CARRIER_ASSIGNED',
  'ACCEPTED_BY_CARRIER',
  'VEHICLE_ASSIGNED',
  'DRIVER_ASSIGNED',
  'PICKUP_SLOT_BOOKED',
])

const DELIVERED_STATUSES = new Set(['DELIVERED', 'DELIVERY_CONFIRMED', 'DOCUMENTS_COMPLETED'])

const MS_HOUR = 60 * 60 * 1000
const MS_DAY = 24 * MS_HOUR

function parseTime(value?: string | null): number | null {
  if (!value) return null
  const time = new Date(value).getTime()
  return Number.isNaN(time) ? null : time
}

function formatRoute(origin?: string, destination?: string): string | undefined {
  if (!origin && !destination) return undefined
  if (origin && destination) return `${origin} → ${destination}`
  return origin || destination
}

export function isActiveShipment(status: string): boolean {
  return !TERMINAL_STATUSES.has(status)
}

export function computeSlaStatus(shipment: Shipment, now = Date.now()): SlaStatus {
  if (shipment.status === 'CANCELLED') return 'CRITICAL'
  if (!shipment.planned_pickup_at && !shipment.planned_delivery_at) return 'UNKNOWN'

  const plannedPickup = parseTime(shipment.planned_pickup_at)
  const plannedDelivery = parseTime(shipment.planned_delivery_at)
  const actualPickup = parseTime(shipment.actual_pickup_at)
  const actualDelivery = parseTime(shipment.actual_delivery_at)
  const updatedAt = parseTime(shipment.updated_at ?? shipment.created_at)

  if (shipment.status === 'CANCELLED') return 'CRITICAL'

  if (plannedDelivery && now > plannedDelivery + MS_DAY && !actualDelivery && !DELIVERED_STATUSES.has(shipment.status)) {
    return 'CRITICAL'
  }

  if (plannedPickup && now > plannedPickup && !actualPickup && PRE_PICKUP_STATUSES.has(shipment.status)) {
    return now > plannedPickup + MS_HOUR ? 'DELAYED' : 'AT_RISK'
  }

  if (plannedDelivery && now > plannedDelivery && !actualDelivery && !DELIVERED_STATUSES.has(shipment.status)) {
    return 'DELAYED'
  }

  if (plannedPickup && now > plannedPickup - 2 * MS_HOUR && now <= plannedPickup && PRE_PICKUP_STATUSES.has(shipment.status)) {
    return 'AT_RISK'
  }

  if (plannedDelivery && now > plannedDelivery - 2 * MS_HOUR && now <= plannedDelivery && IN_TRANSIT_STATUSES.has(shipment.status)) {
    return 'AT_RISK'
  }

  if (IN_TRANSIT_STATUSES.has(shipment.status) && updatedAt && now - updatedAt > MS_DAY) {
    return 'AT_RISK'
  }

  if (DELIVERED_STATUSES.has(shipment.status) || shipment.status === 'READY_FOR_BILLING') {
    return 'ON_TIME'
  }

  if (actualPickup || actualDelivery) return 'ON_TIME'

  return 'ON_TIME'
}

export function deriveLastEvent(shipment: Shipment): string | undefined {
  if (shipment.status === 'CANCELLED') return 'CANCELLED'
  if (shipment.actual_delivery_at) return 'DELIVERED'
  if (shipment.actual_pickup_at) return 'PICKED_UP'
  return shipment.status
}

export function mapShipmentToControlTowerRow(
  shipment: Shipment,
  ctx: {
    transportOrderById: Map<string, TransportOrder>
    companyNameById: Map<string, string>
    locationLabelById: Map<string, string>
    now?: number
  },
): ControlTowerShipment {
  const order = shipment.transport_order_id
    ? ctx.transportOrderById.get(shipment.transport_order_id)
    : undefined
  const origin =
    (shipment.origin_location_id && ctx.locationLabelById.get(shipment.origin_location_id)) ||
    (shipment.origin_location_id ? shortId(shipment.origin_location_id) : undefined)
  const destination =
    (shipment.destination_location_id && ctx.locationLabelById.get(shipment.destination_location_id)) ||
    (shipment.destination_location_id ? shortId(shipment.destination_location_id) : undefined)

  return {
    id: shipment.id,
    shipmentNumber: shipment.shipment_number,
    transportOrderId: shipment.transport_order_id ?? undefined,
    transportOrderNumber: order?.order_number,
    shipperCompanyId: shipment.shipper_company_id ?? undefined,
    carrierCompanyId: shipment.carrier_company_id ?? undefined,
    shipperName: shipment.shipper_company_id
      ? ctx.companyNameById.get(shipment.shipper_company_id)
      : undefined,
    carrierName: shipment.carrier_company_id
      ? ctx.companyNameById.get(shipment.carrier_company_id)
      : undefined,
    origin,
    destination,
    route: formatRoute(origin, destination),
    plannedPickupAt: shipment.planned_pickup_at ?? undefined,
    plannedDeliveryAt: shipment.planned_delivery_at ?? undefined,
    status: shipment.status,
    slaStatus: computeSlaStatus(shipment, ctx.now),
    lastEvent: deriveLastEvent(shipment),
    lastUpdatedAt: shipment.updated_at ?? shipment.created_at,
  }
}


export function buildCriticalEvents(
  shipments: Shipment[],
  documents: DocumentRecord[],
  options: { apiUnavailable?: boolean; now?: number } = {},
): ControlTowerEvent[] {
  const now = options.now ?? Date.now()
  const events: ControlTowerEvent[] = []
  const shipmentIdsWithDocs = new Set(
    documents
      .filter((doc) => doc.related_entity_type === 'SHIPMENT' && doc.related_entity_id)
      .map((doc) => doc.related_entity_id as string),
  )

  if (options.apiUnavailable) {
    events.push({
      id: 'technical-api',
      shipmentId: '',
      shipmentNumber: '—',
      type: 'TECHNICAL_ISSUE',
      severity: 'CRITICAL',
      occurredAt: new Date(now).toISOString(),
      descriptionKey: 'controlTower.events.descriptions.technicalIssue',
    })
  }

  for (const shipment of shipments) {
    const plannedPickup = parseTime(shipment.planned_pickup_at)
    const plannedDelivery = parseTime(shipment.planned_delivery_at)
    const updatedAt = parseTime(shipment.updated_at ?? shipment.created_at)

    if (shipment.status === 'CANCELLED') {
      events.push({
        id: `${shipment.id}-cancelled`,
        shipmentId: shipment.id,
        shipmentNumber: shipment.shipment_number,
        type: 'SHIPMENT_CANCELLED',
        severity: 'CRITICAL',
        occurredAt: shipment.updated_at ?? shipment.created_at ?? new Date(now).toISOString(),
        descriptionKey: 'controlTower.events.descriptions.shipmentCancelled',
      })
      continue
    }

    if (plannedPickup && now > plannedPickup && PRE_PICKUP_STATUSES.has(shipment.status)) {
      events.push({
        id: `${shipment.id}-pickup-delay`,
        shipmentId: shipment.id,
        shipmentNumber: shipment.shipment_number,
        type: 'PICKUP_DELAY',
        severity: 'WARNING',
        occurredAt: shipment.planned_pickup_at!,
        descriptionKey: 'controlTower.events.descriptions.pickupDelay',
      })
    }

    if (plannedDelivery && now > plannedDelivery && !DELIVERED_STATUSES.has(shipment.status)) {
      events.push({
        id: `${shipment.id}-delivery-delay`,
        shipmentId: shipment.id,
        shipmentNumber: shipment.shipment_number,
        type: 'DELIVERY_DELAY',
        severity: 'WARNING',
        occurredAt: shipment.planned_delivery_at!,
        descriptionKey: 'controlTower.events.descriptions.deliveryDelay',
      })
    }

    if (
      !shipment.origin_location_id &&
      !shipment.destination_location_id &&
      IN_TRANSIT_STATUSES.has(shipment.status)
    ) {
      events.push({
        id: `${shipment.id}-no-geo`,
        shipmentId: shipment.id,
        shipmentNumber: shipment.shipment_number,
        type: 'NO_GEOLOCATION',
        severity: 'INFO',
        occurredAt: shipment.updated_at ?? shipment.created_at ?? new Date(now).toISOString(),
        descriptionKey: 'controlTower.events.descriptions.noGeolocation',
      })
    }

    if (updatedAt && now - updatedAt > MS_DAY && isActiveShipment(shipment.status)) {
      events.push({
        id: `${shipment.id}-stale`,
        shipmentId: shipment.id,
        shipmentNumber: shipment.shipment_number,
        type: 'STALE_UPDATES',
        severity: 'WARNING',
        occurredAt: shipment.updated_at ?? shipment.created_at ?? new Date(now).toISOString(),
        descriptionKey: 'controlTower.events.descriptions.staleUpdates',
      })
    }

    if (DELIVERED_STATUSES.has(shipment.status) && !shipmentIdsWithDocs.has(shipment.id)) {
      events.push({
        id: `${shipment.id}-missing-docs`,
        shipmentId: shipment.id,
        shipmentNumber: shipment.shipment_number,
        type: 'MISSING_DOCUMENTS',
        severity: 'WARNING',
        occurredAt: shipment.updated_at ?? shipment.created_at ?? new Date(now).toISOString(),
        descriptionKey: 'controlTower.events.descriptions.missingDocuments',
      })
    }
  }

  return events.sort((a, b) => parseTime(b.occurredAt)! - parseTime(a.occurredAt)!)
}

export function buildKpiMetrics(rows: ControlTowerShipment[]): ControlTowerKpiMetric[] {
  const active = rows.filter((row) => isActiveShipment(row.status))
  const activeCount = active.length || 1

  const onTime = active.filter((row) => row.slaStatus === 'ON_TIME').length
  const atRisk = active.filter((row) => row.slaStatus === 'AT_RISK').length
  const criticalSla = active.filter((row) => row.slaStatus === 'CRITICAL' || row.slaStatus === 'DELAYED').length
  const awaitingDocs = active.filter((row) =>
    ['DELIVERED', 'DELIVERY_CONFIRMED'].includes(row.status),
  ).length
  const readyForBilling = active.filter((row) => row.status === 'READY_FOR_BILLING').length

  const pct = (value: number) => (active.length ? Math.round((value / active.length) * 100) : 0)

  return [
    {
      key: 'activeShipments',
      titleKey: 'controlTower.kpiV01.activeShipments',
      descriptionKey: 'controlTower.kpiV01.activeShipmentsDesc',
      value: active.length,
      tone: 'neutral',
    },
    {
      key: 'onTime',
      titleKey: 'controlTower.kpiV01.onTime',
      descriptionKey: 'controlTower.kpiV01.onTimeDesc',
      value: onTime,
      percent: pct(onTime),
      tone: 'ok',
    },
    {
      key: 'atRisk',
      titleKey: 'controlTower.kpiV01.atRisk',
      descriptionKey: 'controlTower.kpiV01.atRiskDesc',
      value: atRisk,
      percent: pct(atRisk),
      tone: 'warning',
    },
    {
      key: 'critical',
      titleKey: 'controlTower.kpiV01.critical',
      descriptionKey: 'controlTower.kpiV01.criticalDesc',
      value: criticalSla,
      percent: pct(criticalSla),
      tone: 'danger',
    },
    {
      key: 'awaitingDocuments',
      titleKey: 'controlTower.kpiV01.awaitingDocuments',
      descriptionKey: 'controlTower.kpiV01.awaitingDocumentsDesc',
      value: awaitingDocs,
      percent: pct(awaitingDocs),
      tone: 'warning',
    },
    {
      key: 'readyForBilling',
      titleKey: 'controlTower.kpiV01.readyForBilling',
      descriptionKey: 'controlTower.kpiV01.readyForBillingDesc',
      value: readyForBilling,
      percent: pct(readyForBilling),
      tone: 'ok',
    },
  ]
}

function matchesDateFilter(value: string | undefined, dateFilter: string): boolean {
  if (!dateFilter) return true
  if (!value) return false
  return value.slice(0, 10) === dateFilter
}

export function filterControlTowerShipments(
  rows: ControlTowerShipment[],
  filters: ControlTowerFilters,
): ControlTowerShipment[] {
  const search = filters.search.trim().toLowerCase()

  return rows.filter((row) => {
    if (filters.criticalOnly && row.slaStatus !== 'CRITICAL' && row.slaStatus !== 'DELAYED') {
      return false
    }
    if (filters.status && row.status !== filters.status) return false
    if (filters.slaStatus && row.slaStatus !== filters.slaStatus) return false
    if (search) {
      const haystack = [
        row.shipmentNumber,
        row.transportOrderNumber,
        row.shipperName,
        row.carrierName,
        row.route,
      ]
        .filter(Boolean)
        .join(' ')
        .toLowerCase()
      if (!haystack.includes(search)) return false
    }
    if (
      filters.date &&
      !matchesDateFilter(row.plannedPickupAt, filters.date) &&
      !matchesDateFilter(row.plannedDeliveryAt, filters.date)
    ) {
      return false
    }
    return true
  })
}

export function applyControlTowerFilters(
  rows: ControlTowerShipment[],
  filters: ControlTowerFilters,
): ControlTowerShipment[] {
  let result = filterControlTowerShipments(rows, filters)
  if (filters.shipperCompanyId) {
    result = result.filter((row) => row.shipperCompanyId === filters.shipperCompanyId)
  }
  if (filters.carrierCompanyId) {
    result = result.filter((row) => row.carrierCompanyId === filters.carrierCompanyId)
  }
  return result
}

export function companyNameMap(companies: Company[]): Map<string, string> {
  return new Map(
    companies.map((company) => [company.id, company.short_name || company.legal_name]),
  )
}
