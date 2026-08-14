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

export type ControlTowerEventWorkflowStatus = 'open' | 'acknowledged' | 'assigned' | 'resolved'

export type ControlTowerExceptionPriority = 'p1' | 'p2' | 'p3' | 'p4'
export type ControlTowerExceptionSLAStatus = 'within_sla' | 'warning' | 'breached' | 'completed'
export type ControlTowerEscalationLevel = 'none' | 'level_1' | 'level_2' | 'level_3'

export const CONTROL_TOWER_PRIORITIES: ControlTowerExceptionPriority[] = ['p1', 'p2', 'p3', 'p4']

export const CONTROL_TOWER_EXCEPTION_CATEGORIES = [
  'delay',
  'route_deviation',
  'document_issue',
  'vehicle_issue',
  'driver_issue',
  'slot_issue',
  'delivery_issue',
  'pickup_issue',
  'billing_issue',
  'integration_issue',
  'data_quality',
  'other',
] as const

export const CONTROL_TOWER_BUSINESS_IMPACTS = ['none', 'low', 'medium', 'high', 'critical'] as const

export const CONTROL_TOWER_EVENT_SLA_STATUSES: ControlTowerExceptionSLAStatus[] = [
  'within_sla',
  'warning',
  'breached',
  'completed',
]

export const CONTROL_TOWER_ESCALATION_LEVELS: ControlTowerEscalationLevel[] = [
  'none',
  'level_1',
  'level_2',
  'level_3',
]

export interface ControlTowerEventSLA {
  phase: 'acknowledgement' | 'assignment' | 'resolution'
  status: ControlTowerExceptionSLAStatus
  acknowledgeDueAt: string
  assignmentDueAt: string
  resolutionDueAt: string
  remainingSeconds?: number
}

export interface ControlTowerEventEscalation {
  level: ControlTowerEscalationLevel
}

export interface ControlTowerExceptionKpi {
  totalOpenExceptions: number
  p1Open: number
  p2Open: number
  slaWarning: number
  slaBreached: number
  unassignedExceptions: number
  resolvedWithinSla: number
  resolvedOutsideSla: number
}

export type ControlTowerRiskLevel = 'none' | 'low' | 'medium' | 'high' | 'critical'

export type ControlTowerRiskStatus =
  | 'active'
  | 'acknowledged'
  | 'mitigating'
  | 'cleared'
  | 'materialized'

export type ControlTowerPredictedExceptionType =
  | 'pickup_delay_risk'
  | 'delivery_delay_risk'
  | 'slot_miss_risk'
  | 'route_deviation_risk'
  | 'tracking_loss_risk'
  | 'document_readiness_risk'
  | 'vehicle_assignment_risk'
  | 'driver_assignment_risk'

export const CONTROL_TOWER_RISK_LEVELS: ControlTowerRiskLevel[] = [
  'critical',
  'high',
  'medium',
  'low',
]

export const CONTROL_TOWER_RISK_STATUSES: ControlTowerRiskStatus[] = [
  'active',
  'acknowledged',
  'mitigating',
  'cleared',
  'materialized',
]

export const CONTROL_TOWER_PREDICTED_EXCEPTION_TYPES: ControlTowerPredictedExceptionType[] = [
  'pickup_delay_risk',
  'delivery_delay_risk',
  'slot_miss_risk',
  'tracking_loss_risk',
  'driver_assignment_risk',
  'vehicle_assignment_risk',
]

export const CONTROL_TOWER_MITIGATION_CODES = [
  'contact_carrier',
  'contact_driver',
  'reschedule_slot',
  'request_documents',
  'reassign_driver',
  'reassign_vehicle',
  'adjust_plan',
  'monitor',
  'other',
] as const

export type ControlTowerMitigationCode = (typeof CONTROL_TOWER_MITIGATION_CODES)[number]

export interface ControlTowerRiskSignal {
  signalCode: string
  severity: string
  weight: number
  observedAt: string
  source: string
  value?: Record<string, unknown>
  explanationKey: string
}

export interface ControlTowerRiskAction {
  actionType: string
  actorUserId?: string
  occurredAt: string
  metadata?: Record<string, unknown>
}

export interface ControlTowerShipmentRisk {
  riskId: string
  shipmentId: string
  shipmentNumber: string
  predictedExceptionType: ControlTowerPredictedExceptionType | string
  score: number
  level: ControlTowerRiskLevel
  status: ControlTowerRiskStatus
  signals: ControlTowerRiskSignal[]
  firstDetectedAt: string
  evaluatedAt: string
  nextEvaluationAt?: string
  threatenedDeadlineAt?: string
  mitigationCode?: string
  mitigationComment?: string
  actualEventId?: string
  materializedAt?: string
  clearedAt?: string
  clearReason?: string
  leadTimeSeconds?: number
  actions?: ControlTowerRiskAction[]
}

export interface ControlTowerRiskKpi {
  activeRisks: number
  criticalRisks: number
  highRisks: number
  deliveryDelayRisks: number
  pickupDelayRisks: number
  slotMissRisks: number
  mitigatingRisks: number
  risksMaterialized: number
  risksCleared: number
  clearedBeforeMaterialization: number
}

export interface ControlTowerRisksListResponse {
  items: ControlTowerShipmentRisk[]
}

export type ControlTowerEventResolutionCode =
  | 'issue_resolved'
  | 'false_positive'
  | 'duplicate'
  | 'cancelled'
  | 'other'

export const CONTROL_TOWER_RESOLUTION_CODES: ControlTowerEventResolutionCode[] = [
  'issue_resolved',
  'false_positive',
  'duplicate',
  'cancelled',
  'other',
]

export interface ControlTowerEventAssignment {
  assignedToUserId: string
  assignedByUserId: string
  assignedAt: string
}

export interface ControlTowerEventResolution {
  resolvedByUserId: string
  resolvedAt: string
  resolutionCode: ControlTowerEventResolutionCode
  comment?: string
}

export interface ControlTowerEventWorkflow {
  eventId: string
  status: ControlTowerEventWorkflowStatus
  acknowledgement?: ControlTowerEventAcknowledgementSummary
  assignment?: ControlTowerEventAssignment
  resolution?: ControlTowerEventResolution
}

export interface ControlTowerEventAction {
  actionType:
    | 'acknowledged'
    | 'assigned'
    | 'reassigned'
    | 'resolved'
    | 'reopened'
    | 'exception_updated'
    | 'ack_sla_breached'
    | 'assign_sla_breached'
    | 'resolve_sla_breached'
    | 'escalation_changed'
  actorUserId: string
  occurredAt: string
  metadata?: Record<string, unknown>
}

export interface ControlTowerEventActionsResponse {
  items: ControlTowerEventAction[]
}

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
  status?: ControlTowerEventWorkflowStatus
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
  status?: ControlTowerEventWorkflowStatus
  priority?: ControlTowerExceptionPriority
  exceptionCategory?: string
  businessImpact?: string
  sla?: ControlTowerEventSLA
  escalation?: ControlTowerEventEscalation
  acknowledgement?: ControlTowerEventAcknowledgementSummary
  assignment?: ControlTowerEventAssignment
  resolution?: ControlTowerEventResolution
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
  eventStatus: string
  priority: string
  exceptionCategory: string
  businessImpact: string
  eventSlaStatus: string
  escalationLevel: string
  unassignedOnly: boolean
  riskLevel: string
  riskStatus: string
  predictedExceptionType: string
  riskShipmentId: string
  riskMitigatingOnly: boolean
  riskNonMitigatingOnly: boolean
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
  exceptionKpi?: ControlTowerExceptionKpi
  riskKpi?: ControlTowerRiskKpi
  shipments: ControlTowerSummaryPagination
  criticalEvents: ControlTowerEvent[]
  shipmentRisks?: ControlTowerShipmentRisk[]
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
    eventStatus: '',
    priority: '',
    exceptionCategory: '',
    businessImpact: '',
    eventSlaStatus: '',
    escalationLevel: '',
    unassignedOnly: false,
    riskLevel: '',
    riskStatus: '',
    predictedExceptionType: '',
    riskShipmentId: '',
    riskMitigatingOnly: false,
    riskNonMitigatingOnly: false,
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

export type ControlTowerWorkItemType = 'exception' | 'risk'

export type ControlTowerWorkItemUrgency = 'critical' | 'high' | 'normal' | 'low'

export interface ControlTowerWorkItem {
  id: string
  itemType: ControlTowerWorkItemType
  sourceId: string
  shipmentId: string
  shipmentNumber?: string
  title: string
  summary: string
  workflowStatus: string
  priority?: string
  businessImpact?: string
  exceptionCategory?: string
  slaStatus?: string
  slaPhase?: string
  slaDueAt?: string
  riskLevel?: string
  riskScore?: number
  riskStatus?: string
  predictedExceptionType?: string
  escalationLevel?: string
  urgency: ControlTowerWorkItemUrgency
  ownerUserId?: string
  ownerDisplayName?: string
  createdAt: string
  updatedAt: string
  threatenedDeadlineAt?: string
  availableActions: string[]
  linkedEventId?: string
  eventType?: string
  timeline?: ControlTowerWorkItemTimelineEntry[]
}

export interface ControlTowerWorkItemTimelineEntry {
  source: string
  actionType: string
  actorUserId?: string
  actorDisplayName?: string
  occurredAt: string
  metadata?: Record<string, unknown>
}

export interface ControlTowerWorkItemsResponse {
  items: ControlTowerWorkItem[]
  page: number
  limit: number
  total: number
  hasNext: boolean
  kpi?: ControlTowerWorkspaceKpi
}

export interface ControlTowerWorkspaceKpi {
  myActiveWork: number
  myCriticalWork: number
  unassignedWork: number
  teamActiveWork: number
  slaBreachedWork: number
  slaWarningWork: number
  criticalRiskWork: number
}

export interface ControlTowerSavedView {
  id: string
  name: string
  scope: 'private' | 'shared'
  filterSchemaVersion: number
  filters: Record<string, unknown>
  sort: Record<string, unknown>
  isDefault: boolean
  createdAt: string
  updatedAt: string
}

export interface ControlTowerBulkActionResult {
  itemType: string
  itemId: string
  success: boolean
  error?: string
}

export interface ControlTowerBulkActionOutcome {
  requested: number
  succeeded: number
  failed: number
  results: ControlTowerBulkActionResult[]
}

export const CONTROL_TOWER_WORKSPACE_PRESETS = [
  'my_work',
  'unassigned',
  'all_active',
  'critical',
  'sla_breached',
  'sla_warning',
  'p1_exceptions',
  'p2_exceptions',
  'critical_risks',
  'high_risks',
  'mitigating_risks',
  'completed',
] as const

export type ControlTowerWorkspacePreset = (typeof CONTROL_TOWER_WORKSPACE_PRESETS)[number]
