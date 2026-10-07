export const STOP_STATUSES = [
  'PLANNED',
  'ARRIVED',
  'SERVICE_STARTED',
  'COMPLETED',
  'CANCELLED',
  'SKIPPED',
] as const

export type DriverStopStatus = (typeof STOP_STATUSES)[number]
export type DriverStopPosition = 'CURRENT' | 'NEXT'
export type DriverStopActionType = 'PICKUP' | 'DELIVERY'

export const FAIL_REASON_CODES = [
  'TRAFFIC',
  'VEHICLE_BREAKDOWN',
  'ACCIDENT',
  'LOADING_DELAY',
  'UNLOADING_DELAY',
  'CARGO_ISSUE',
  'DOCUMENT_ISSUE',
  'CUSTOMER_UNAVAILABLE',
  'ROUTE_BLOCKED',
  'OTHER',
] as const

export type DriverFailReasonCode = (typeof FAIL_REASON_CODES)[number]

export const DISPOSITION_REASON_CODES = [
  'DAMAGE',
  'MIS_SORT',
  'SHORTAGE',
  'OVERAGE',
  'PACKAGING_DAMAGE',
  'TEMPERATURE_DEVIATION',
  'QUALITY_REJECTION',
  'DOCUMENT_PROBLEM',
  'WRONG_PRODUCT',
  'EXPIRED_PRODUCT',
  'CUSTOMER_REFUSAL',
  'OTHER',
] as const

export type DriverDispositionReasonCode = (typeof DISPOSITION_REASON_CODES)[number]

export interface DriverStopActionFact {
  actionId: string
  actionType: DriverStopActionType
  shipmentId: string
  cargoId: string
  ordinal: number
}

export interface DriverStopActionSummary {
  actions: DriverStopActionFact[]
  counts: Record<string, number>
}

export interface DriverStopTask {
  taskId: string
  executionId: string
  executionStopId: string
  shipmentId?: string | null
  ordinal: number
  locationId: string
  plannedArrival?: string | null
  status: DriverStopStatus
  version: number
  actionSummary: DriverStopActionSummary
  position: DriverStopPosition
}

export interface DriverCurrentNextStopsResponse {
  current: DriverStopTask | null
  next: DriverStopTask | null
}

export interface DriverStopCommandRequest {
  occurredAt: string
  expectedVersion: number
}

export interface DriverStopFailRequest {
  occurredAt: string
  expectedVersion: number
  reasonCode: DriverFailReasonCode
}

export interface DriverStopCommandResponse {
  taskId: string
  executionStopId: string
  status: string
  version: number
  actionStatus?: string
  evidenceId?: string
  shipmentStatus?: string
  replayed: boolean
}

export interface DriverDeliveryDispositionRequest {
  shipmentId: string
  cargoId: string
  acceptedQuantity: number
  rejectedQuantity: number
  uom: 'PALLET'
  reasonCode?: DriverDispositionReasonCode
  reasonComment?: string
  occurredAt: string
  evidence: []
}

export interface DriverDeliveryDispositionResponse {
  executionId: string
  caseId: string | null
  status: string
  acceptedQuantity: number
  rejectedQuantity: number
  revisionId: string
  replayed: boolean
}
