import type { BillingRegister } from '~/types/billing'
import type { Company } from '~/types/company'
import type { DocumentRecord } from '~/types/document'
import type { FreightRequest, RfxEvent } from '~/types/rfx'
import type { Shipment } from '~/types/shipment'
import type { TransportOrder } from '~/types/transportOrder'
import type { AuthUser } from '~/types/api'

export type ControlTowerAreaStatus = 'ok' | 'warning' | 'down'

export type ControlTowerBadgeTone = 'ok' | 'warning' | 'down' | 'unavailable' | 'neutral'

export interface ControlTowerFetchResult<T> {
  key: string
  ok: boolean
  total: number
  items: T[]
}

export interface ControlTowerKpiCard {
  key: string
  titleKey: string
  descriptionKey: string
  value: string | number
  badgeLabel: string
  badgeTone: ControlTowerBadgeTone
  link: string
  unavailable: boolean
}

export interface ControlTowerOperationRow {
  key: string
  areaKey: string
  status: ControlTowerAreaStatus
  count: number
  link: string
}

export interface ControlTowerFunnelStep {
  key: string
  labelKey: string
  count: number
}

export interface ControlTowerShipmentStatusRow {
  status: string
  count: number
  link: string
}

export interface ControlTowerRiskAlert {
  key: string
  messageKey: string
  severity: 'warning' | 'danger'
  count?: number
}

export interface ControlTowerActivityItem {
  id: string
  typeKey: string
  title: string
  status: string
  timestamp: string
  link: string
}

export interface ControlTowerQuickAction {
  key: string
  labelKey: string
  to?: string
  href?: string
  external?: boolean
}

export interface ControlTowerDocumentsSummary {
  total: number
  readyForSigning: number
  signed: number
  archived: number
  cancelled: number
  unavailable: boolean
}

export interface ControlTowerBillingSummary {
  total: number
  draft: number
  approved: number
  closingDocsCreated: number
  sentToEdo: number
  signed: number
  paid: number
  closed: number
  revenueTotal: number
  unavailable: boolean
}

export interface ControlTowerData {
  companies: ControlTowerFetchResult<Company>
  users: ControlTowerFetchResult<AuthUser>
  transportOrders: ControlTowerFetchResult<TransportOrder>
  rfxEvents: ControlTowerFetchResult<RfxEvent>
  freightRequests: ControlTowerFetchResult<FreightRequest>
  shipments: ControlTowerFetchResult<Shipment>
  documents: ControlTowerFetchResult<DocumentRecord>
  billingRegisters: ControlTowerFetchResult<BillingRegister>
}

export type SlaStatus = 'ON_TIME' | 'AT_RISK' | 'DELAYED' | 'CRITICAL' | 'UNKNOWN'

export type ControlTowerSeverity = 'INFO' | 'WARNING' | 'CRITICAL'

export type ControlTowerEventType =
  | 'PICKUP_DELAY'
  | 'DELIVERY_DELAY'
  | 'NO_GEOLOCATION'
  | 'STALE_UPDATES'
  | 'ROUTE_DEVIATION'
  | 'MISSING_DOCUMENTS'
  | 'SHIPMENT_CANCELLED'
  | 'TECHNICAL_ISSUE'
  | 'TECHNICAL_PROBLEM'
  | 'UNKNOWN_CRITICAL'
  | 'UNKNOWN_CRITICAL_EVENT'

export interface ControlTowerEventAcknowledgedBy {
  userId: string
  displayName?: string
}

export interface ControlTowerEventAcknowledgementSummary {
  acknowledgedAt: string
  acknowledgedBy: ControlTowerEventAcknowledgedBy
}

export interface ControlTowerEventAcknowledgement extends ControlTowerEventAcknowledgementSummary {
  eventId: string
  shipmentId: string
  eventType: ControlTowerEventType
  occurredAt: string
  source?: 'control-tower'
}

export interface ControlTowerEvent {
  id: string
  shipmentId: string
  shipmentNumber: string
  type: ControlTowerEventType
  severity: ControlTowerSeverity
  occurredAt: string
  description?: string
  descriptionKey?: string
  acknowledgement?: ControlTowerEventAcknowledgementSummary
}

export interface ControlTowerShipment {
  id: string
  shipmentNumber: string
  transportOrderId?: string
  transportOrderNumber?: string
  shipperId?: string
  shipperCompanyId?: string
  carrierId?: string
  carrierCompanyId?: string
  shipperName?: string
  carrierName?: string
  originId?: string
  originName?: string
  destinationId?: string
  destinationName?: string
  origin?: string
  destination?: string
  route?: string
  plannedPickupAt?: string
  plannedDeliveryAt?: string
  status: string
  slaStatus: SlaStatus
  lastEvent?: string
  lastUpdatedAt?: string
}

export interface ControlTowerFilters {
  search: string
  status: string
  slaStatus: string
  shipperCompanyId: string
  carrierCompanyId: string
  date: string
  criticalOnly: boolean
}

export type ControlTowerKpiTone = 'ok' | 'warning' | 'danger' | 'neutral'

export interface ControlTowerKpiMetric {
  key: string
  titleKey: string
  descriptionKey: string
  value: number
  percent?: number
  tone: ControlTowerKpiTone
}

export const CONTROL_TOWER_AUTO_REFRESH_INTERVALS = [
  { key: '30s', ms: 30_000 },
  { key: '1m', ms: 60_000 },
  { key: '5m', ms: 300_000 },
] as const

export const CONTROL_TOWER_ACCESS_ROLES = [
  'PLATFORM_ADMIN',
  'CARRIER_DISPATCHER',
  'SHIPPER_ADMIN',
  'SHIPPER_LOGIST',
  'FORWARDER_MANAGER',
] as const

export const CONTROL_TOWER_SHIPMENT_BOARD_STATUSES = [
  'CARRIER_ASSIGNED',
  'ACCEPTED_BY_CARRIER',
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
] as const

export interface ControlTowerFilterOption {
  value: string
  label: string
}

export interface ControlTowerDataFreshness {
  shipmentsLoaded: boolean
  transportOrdersLoaded: boolean
  companiesLoaded: boolean
  documentsLoaded: boolean
  partial: boolean
  warnings: string[]
}

export interface ControlTowerSummaryKpi {
  active: number
  onTime: number
  atRisk: number
  delayed: number
  critical: number
  awaitingDocuments: number
  readyForBilling: number
}

export interface ControlTowerSummaryPagination {
  items: ControlTowerShipment[]
  page: number
  limit: number
  total: number
  hasNext: boolean
}

export interface ControlTowerSummaryFilters {
  statuses: ControlTowerFilterOption[]
  shippers: ControlTowerFilterOption[]
  carriers: ControlTowerFilterOption[]
}

export interface ControlTowerSummaryResponse {
  generatedAt: string
  dataFreshness: ControlTowerDataFreshness
  kpi: ControlTowerSummaryKpi
  shipments: ControlTowerSummaryPagination
  criticalEvents: ControlTowerEvent[]
  filters: ControlTowerSummaryFilters
  statusSummary?: ControlTowerStatusSummary
  statusSummaryFreshness?: ControlTowerStatusSummaryFreshness
}

export type ControlTowerStatusSummarySource = 'LEGACY' | 'READ_MODEL'

export interface ControlTowerStatusSummary {
  totalShipments: number
  countedShipments?: number
  byStatus: Record<string, number>
  incompleteProjections?: number
  source?: ControlTowerStatusSummarySource
  limitedDataset?: boolean
}

export interface ControlTowerStatusSummaryFreshness {
  loaded?: boolean
  fallbackUsed?: boolean
  partial?: boolean
  source?: ControlTowerStatusSummarySource
  consumerRunning?: boolean
  lastRecordReceivedAt?: string
  lastProjectionAppliedAt?: string
}

export function createDefaultControlTowerFilters(): ControlTowerFilters {
  return {
    search: '',
    status: '',
    slaStatus: '',
    shipperCompanyId: '',
    carrierCompanyId: '',
    date: '',
    criticalOnly: false,
  }
}

export const CONTROL_TOWER_PARTIAL_WARNING_CODES = [
  'COMPANIES_UNAVAILABLE',
  'DOCUMENTS_UNAVAILABLE',
  'TRANSPORT_ORDERS_UNAVAILABLE',
  'KPI_CALCULATED_FROM_LIMITED_DATASET',
  'FILTER_OPTIONS_INCOMPLETE',
  'CONTROL_TOWER_READ_MODEL_UNAVAILABLE',
  'CONTROL_TOWER_READ_MODEL_CONSUMER_NOT_RUNNING',
  'CONTROL_TOWER_READ_MODEL_PARTIAL',
  'CONTROL_TOWER_READ_MODEL_FALLBACK_USED',
  'CONTROL_TOWER_LEGACY_STATUS_SUMMARY_LIMITED',
] as const
